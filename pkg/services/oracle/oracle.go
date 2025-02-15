package oracle

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

type (
	// Ledger is the interface to Blockchain sufficient for Oracle.
	Ledger interface {
		BlockHeight() uint32
		FeePerByte() int64
		GetBaseExecFee() int64
		GetConfig() config.ProtocolConfiguration
		GetMaxVerificationGAS() int64
		GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) *interop.Context
		GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	}

	// Oracle represents oracle module capable of talking
	// with the external world.
	Oracle struct {
		Config

		// This fields are readonly thus not protected by mutex.
		oracleHash     util.Uint160
		oracleResponse []byte
		oracleScript   []byte
		verifyOffset   int

		// mtx protects setting callbacks.
		mtx sync.RWMutex

		// accMtx protects account and oracle nodes.
		accMtx             sync.RWMutex
		currAccount        *wallet.Account
		oracleNodes        keys.PublicKeys
		oracleSignContract []byte

		close      chan struct{}
		requestCh  chan request
		requestMap chan map[uint64]*state.OracleRequest

		// respMtx protects responses and pending maps.
		respMtx sync.RWMutex
		// running is false until Run() is invoked.
		running bool
		// pending contains requests for not yet started service.
		pending map[uint64]*state.OracleRequest
		// responses contains active not completely processed requests.
		responses map[uint64]*incompleteTx
		// removed contains ids of requests which won't be processed further due to expiration.
		removed map[uint64]bool

		wallet *wallet.Wallet
	}

	// Config contains oracle module parameters.
	Config struct {
		Log             *zap.Logger
		Network         netmode.Magic
		MainCfg         config.OracleConfiguration
		Client          HTTPClient
		Chain           Ledger
		ResponseHandler Broadcaster
		OnTransaction   TxCallback
	}

	// HTTPClient is an interface capable of doing oracle requests.
	HTTPClient interface {
		Do(*http.Request) (*http.Response, error)
	}

	// Broadcaster broadcasts oracle responses.
	Broadcaster interface {
		SendResponse(priv *keys.PrivateKey, resp *transaction.OracleResponse, txSig []byte)
		Run()
		Shutdown()
	}

	defaultResponseHandler struct{}

	// TxCallback executes on new transactions when they are ready to be pooled.
	TxCallback = func(tx *transaction.Transaction) error
)

const (
	// defaultRequestTimeout is default request timeout.
	defaultRequestTimeout = time.Second * 5

	// defaultMaxTaskTimeout is default timeout for the request to be dropped if it can't be processed.
	defaultMaxTaskTimeout = time.Hour

	// defaultRefreshInterval is default timeout for the failed request to be reprocessed.
	defaultRefreshInterval = time.Minute * 3

	// maxRedirections is the number of allowed redirections for Oracle HTTPS request.
	maxRedirections = 5
)

// ErrRestrictedRedirect is returned when redirection to forbidden address occurs
// during Oracle response creation.
var ErrRestrictedRedirect = errors.New("oracle request redirection error")

// NewOracle returns new oracle instance.
func NewOracle(cfg Config) (*Oracle, error) {
	o := &Oracle{
		Config: cfg,

		close:      make(chan struct{}),
		requestMap: make(chan map[uint64]*state.OracleRequest, 1),
		pending:    make(map[uint64]*state.OracleRequest),
		responses:  make(map[uint64]*incompleteTx),
		removed:    make(map[uint64]bool),
	}
	if o.MainCfg.RequestTimeout == 0 {
		o.MainCfg.RequestTimeout = defaultRequestTimeout
	}
	if o.MainCfg.NeoFS.Timeout == 0 {
		o.MainCfg.NeoFS.Timeout = defaultRequestTimeout
	}
	if o.MainCfg.MaxConcurrentRequests == 0 {
		o.MainCfg.MaxConcurrentRequests = defaultMaxConcurrentRequests
	}
	o.requestCh = make(chan request, o.MainCfg.MaxConcurrentRequests)
	if o.MainCfg.MaxTaskTimeout == 0 {
		o.MainCfg.MaxTaskTimeout = defaultMaxTaskTimeout
	}
	if o.MainCfg.RefreshInterval == 0 {
		o.MainCfg.RefreshInterval = defaultRefreshInterval
	}

	var err error
	w := cfg.MainCfg.UnlockWallet
	if o.wallet, err = wallet.NewWalletFromFile(w.Path); err != nil {
		return nil, err
	}

	haveAccount := false
	for _, acc := range o.wallet.Accounts {
		if err := acc.Decrypt(w.Password, o.wallet.Scrypt); err == nil {
			haveAccount = true
			break
		}
	}
	if !haveAccount {
		return nil, errors.New("no wallet account could be unlocked")
	}

	if o.ResponseHandler == nil {
		o.ResponseHandler = defaultResponseHandler{}
	}
	if o.OnTransaction == nil {
		o.OnTransaction = func(*transaction.Transaction) error { return nil }
	}
	if o.Client == nil {
		o.Client = getDefaultClient(o.MainCfg)
	}
	return o, nil
}

// Shutdown shutdowns Oracle.
func (o *Oracle) Shutdown() {
	close(o.close)
	o.getBroadcaster().Shutdown()
}

// Start runs the oracle service in a separate goroutine.
func (o *Oracle) Start() {
	o.respMtx.Lock()
	if o.running {
		o.respMtx.Unlock()
		return
	}
	o.Log.Info("starting oracle service")
	go o.start()
}

func (o *Oracle) start() {
	o.requestMap <- o.pending // Guaranteed to not block, only AddRequests sends to it.
	o.pending = nil
	o.running = true
	o.respMtx.Unlock()

	for i := 0; i < o.MainCfg.MaxConcurrentRequests; i++ {
		go o.runRequestWorker()
	}

	tick := time.NewTicker(o.MainCfg.RefreshInterval)
	for {
		select {
		case <-o.close:
			tick.Stop()
			return
		case <-tick.C:
			var reprocess []uint64
			o.respMtx.Lock()
			o.removed = make(map[uint64]bool)
			for id, incTx := range o.responses {
				incTx.RLock()
				since := time.Since(incTx.time)
				if since > o.MainCfg.MaxTaskTimeout {
					o.removed[id] = true
				} else if since > o.MainCfg.RefreshInterval {
					reprocess = append(reprocess, id)
				}
				incTx.RUnlock()
			}
			for id := range o.removed {
				delete(o.responses, id)
			}
			o.respMtx.Unlock()

			for _, id := range reprocess {
				o.requestCh <- request{ID: id}
			}
		case reqs := <-o.requestMap:
			for id, req := range reqs {
				o.requestCh <- request{
					ID:  id,
					Req: req,
				}
			}
		}
	}
}

// UpdateNativeContract updates native oracle contract info for tx verification.
func (o *Oracle) UpdateNativeContract(script, resp []byte, h util.Uint160, verifyOffset int) {
	o.oracleScript = slice.Copy(script)
	o.oracleResponse = slice.Copy(resp)

	o.oracleHash = h
	o.verifyOffset = verifyOffset
}

func (o *Oracle) sendTx(tx *transaction.Transaction) {
	if err := o.OnTransaction(tx); err != nil {
		o.Log.Error("can't pool oracle tx",
			zap.String("hash", tx.Hash().StringLE()),
			zap.Error(err))
	}
}

func (o *Oracle) getBroadcaster() Broadcaster {
	o.mtx.RLock()
	defer o.mtx.RUnlock()
	return o.ResponseHandler
}

// SetBroadcaster sets callback to broadcast response.
func (o *Oracle) SetBroadcaster(b Broadcaster) {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	o.ResponseHandler.Shutdown()
	o.ResponseHandler = b
	go b.Run()
}

// SendResponse implements Broadcaster interface.
func (defaultResponseHandler) SendResponse(*keys.PrivateKey, *transaction.OracleResponse, []byte) {
}

// Run implements Broadcaster interface.
func (defaultResponseHandler) Run() {}

// Shutdown implements Broadcaster interface.
func (defaultResponseHandler) Shutdown() {}
