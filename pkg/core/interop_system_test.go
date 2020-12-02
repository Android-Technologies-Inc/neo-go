package core

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/callback"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestBCGetTransaction(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()

	t.Run("success", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 0, nil))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 8, len(actual))
		require.Equal(t, tx.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(tx.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.Nonce), actual[2].Value().(*big.Int).Int64())
		require.Equal(t, tx.Sender().BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(tx.SystemFee), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.NetworkFee), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.ValidUntilBlock), actual[6].Value().(*big.Int).Int64())
		require.Equal(t, tx.Script, actual[7].Value().([]byte))
	})

	t.Run("isn't traceable", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1, nil))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad hash", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1, nil))
		v.Estack().PushVal(tx.Hash().BytesLE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}

func TestBCGetTransactionFromBlock(t *testing.T) {
	v, block, context, chain := createVMAndBlock(t)
	defer chain.Close()
	require.NoError(t, chain.AddBlock(chain.newBlock()))
	require.NoError(t, context.DAO.StoreAsBlock(block, nil))

	t.Run("success", func(t *testing.T) {
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]byte)
		require.True(t, ok)
		require.Equal(t, block.Transactions[0].Hash().BytesBE(), actual)
	})

	t.Run("invalid block hash", func(t *testing.T) {
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE()[:10])
		err := bcGetTransactionFromBlock(context)
		require.Error(t, err)
	})

	t.Run("isn't traceable", func(t *testing.T) {
		block.Index = 2
		require.NoError(t, context.DAO.StoreAsBlock(block, nil))
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad block hash", func(t *testing.T) {
		block.Index = 1
		require.NoError(t, context.DAO.StoreAsBlock(block, nil))
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesLE())
		err := bcGetTransactionFromBlock(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad transaction index", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsBlock(block, nil))
		v.Estack().PushVal(1)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context)
		require.Error(t, err)
	})
}

func TestBCGetBlock(t *testing.T) {
	v, context, chain := createVM(t)
	defer chain.Close()
	block := chain.newBlock()
	require.NoError(t, chain.AddBlock(block))

	t.Run("success", func(t *testing.T) {
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetBlock(context)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 8, len(actual))
		require.Equal(t, block.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(block.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, block.PrevHash.BytesBE(), actual[2].Value().([]byte))
		require.Equal(t, block.MerkleRoot.BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(block.Timestamp), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(block.Index), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, block.NextConsensus.BytesBE(), actual[6].Value().([]byte))
		require.Equal(t, int64(len(block.Transactions)), actual[7].Value().(*big.Int).Int64())
	})

	t.Run("bad hash", func(t *testing.T) {
		v.Estack().PushVal(block.Hash().BytesLE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}

func TestContractIsStandard(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	t.Run("contract not stored", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)

		pub := priv.PublicKey()
		tx := transaction.New(netmode.TestNet, []byte{1, 2, 3}, 1)
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript:   []byte{1, 2, 3},
				VerificationScript: pub.GetVerificationScript(),
			},
		}
		ic.Container = tx

		t.Run("true", func(t *testing.T) {
			v.Estack().PushVal(pub.GetScriptHash().BytesBE())
			require.NoError(t, contractIsStandard(ic))
			require.True(t, v.Estack().Pop().Bool())
		})

		t.Run("false", func(t *testing.T) {
			tx.Scripts[0].VerificationScript = []byte{9, 8, 7}
			v.Estack().PushVal(pub.GetScriptHash().BytesBE())
			require.NoError(t, contractIsStandard(ic))
			require.False(t, v.Estack().Pop().Bool())
		})
	})

	t.Run("contract stored, true", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)

		pub := priv.PublicKey()
		err = ic.DAO.PutContractState(&state.Contract{ID: 42, Hash: pub.GetScriptHash(), Script: pub.GetVerificationScript()})
		require.NoError(t, err)

		v.Estack().PushVal(pub.GetScriptHash().BytesBE())
		require.NoError(t, contractIsStandard(ic))
		require.True(t, v.Estack().Pop().Bool())
	})
	t.Run("contract stored, false", func(t *testing.T) {
		script := []byte{byte(opcode.PUSHT)}
		require.NoError(t, ic.DAO.PutContractState(&state.Contract{ID: 24, Hash: hash.Hash160(script), Script: script}))

		v.Estack().PushVal(crypto.Hash160(script).BytesBE())
		require.NoError(t, contractIsStandard(ic))
		require.False(t, v.Estack().Pop().Bool())
	})
}

func TestContractCreateAccount(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()
	t.Run("Good", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		pub := priv.PublicKey()
		v.Estack().PushVal(pub.Bytes())
		require.NoError(t, contractCreateStandardAccount(ic))

		value := v.Estack().Pop().Bytes()
		u, err := util.Uint160DecodeBytesBE(value)
		require.NoError(t, err)
		require.Equal(t, pub.GetScriptHash(), u)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		v.Estack().PushVal([]byte{1, 2, 3})
		require.Error(t, contractCreateStandardAccount(ic))
	})
}

func TestBlockchainGetContractState(t *testing.T) {
	v, cs, ic, bc := createVMAndContractState(t)
	defer bc.Close()
	require.NoError(t, ic.DAO.PutContractState(cs))

	t.Run("positive", func(t *testing.T) {
		v.Estack().PushVal(cs.Hash.BytesBE())
		require.NoError(t, bcGetContract(ic))

		actual := v.Estack().Pop().Item()
		compareContractStates(t, cs, actual)
	})

	t.Run("uncknown contract state", func(t *testing.T) {
		v.Estack().PushVal(util.Uint160{1, 2, 3}.BytesBE())
		require.NoError(t, bcGetContract(ic))

		actual := v.Estack().Pop().Item()
		require.Equal(t, stackitem.Null{}, actual)
	})
}

func TestStoragePut(t *testing.T) {
	_, cs, ic, bc := createVMAndContractState(t)
	defer bc.Close()

	require.NoError(t, ic.DAO.PutContractState(cs))

	initVM := func(t *testing.T, key, value []byte, gas int64) {
		v := ic.SpawnVM()
		v.LoadScript(cs.Script)
		v.GasLimit = gas
		v.Estack().PushVal(value)
		v.Estack().PushVal(key)
		require.NoError(t, storageGetContext(ic))
	}

	t.Run("create, not enough gas", func(t *testing.T) {
		initVM(t, []byte{1}, []byte{2, 3}, 2*StoragePrice)
		err := storagePut(ic)
		require.True(t, errors.Is(err, errGasLimitExceeded), "got: %v", err)
	})

	initVM(t, []byte{4}, []byte{5, 6}, 3*StoragePrice)
	require.NoError(t, storagePut(ic))

	t.Run("update", func(t *testing.T) {
		t.Run("not enough gas", func(t *testing.T) {
			initVM(t, []byte{4}, []byte{5, 6, 7, 8}, StoragePrice)
			err := storagePut(ic)
			require.True(t, errors.Is(err, errGasLimitExceeded), "got: %v", err)
		})
		initVM(t, []byte{4}, []byte{5, 6, 7, 8}, 3*StoragePrice)
		require.NoError(t, storagePut(ic))
		initVM(t, []byte{4}, []byte{5, 6}, StoragePrice)
		require.NoError(t, storagePut(ic))
	})
}

// getTestContractState returns 2 contracts second of which is allowed to call the first.
func getTestContractState() (*state.Contract, *state.Contract) {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	addOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.ADD, opcode.RET)
	ret7Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.PUSH7, opcode.RET)
	dropOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.DROP, opcode.RET)
	initOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.INITSSLOT, 1, opcode.PUSH3, opcode.STSFLD0, opcode.RET)
	add3Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.ADD, opcode.RET)
	invalidRetOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PUSH2, opcode.RET)
	justRetOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.RET)
	verifyOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.SUB,
		opcode.CONVERT, opcode.Opcode(stackitem.BooleanT), opcode.RET)
	deployOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.JMPIF, 2+8+3)
	emit.String(w.BinWriter, "create")
	emit.Opcodes(w.BinWriter, opcode.CALL, 3+8+3, opcode.RET)
	emit.String(w.BinWriter, "update")
	emit.Opcodes(w.BinWriter, opcode.CALL, 3, opcode.RET)
	putValOff := w.Len()
	emit.String(w.BinWriter, "initial")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStoragePut)
	emit.Opcodes(w.BinWriter, opcode.RET)
	getValOff := w.Len()
	emit.String(w.BinWriter, "initial")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGet)
	emit.Opcodes(w.BinWriter, opcode.RET)
	onPaymentOff := w.Len()
	emit.Int(w.BinWriter, 3)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "LastPayment")
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcodes(w.BinWriter, opcode.RET)

	script := w.Bytes()
	h := hash.Hash160(script)
	m := manifest.NewManifest("TestMain")
	m.ABI.Methods = []manifest.Method{
		{
			Name:   "add",
			Offset: addOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend1", smartcontract.IntegerType),
				manifest.NewParameter("addend2", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "ret7",
			Offset:     ret7Off,
			Parameters: []manifest.Parameter{},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "drop",
			Offset:     dropOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       manifest.MethodInit,
			Offset:     initOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "add3",
			Offset: add3Off,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "invalidReturn",
			Offset:     invalidRetOff,
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "justReturn",
			Offset:     justRetOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       manifest.MethodVerify,
			Offset:     verifyOff,
			ReturnType: smartcontract.BoolType,
		},
		{
			Name:   manifest.MethodDeploy,
			Offset: deployOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("isUpdate", smartcontract.BoolType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "getValue",
			Offset:     getValOff,
			ReturnType: smartcontract.StringType,
		},
		{
			Name:   "putValue",
			Offset: putValOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("value", smartcontract.StringType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "onPayment",
			Offset: onPaymentOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("from", smartcontract.Hash160Type),
				manifest.NewParameter("amount", smartcontract.IntegerType),
				manifest.NewParameter("data", smartcontract.AnyType),
			},
			ReturnType: smartcontract.VoidType,
		},
	}
	cs := &state.Contract{
		Script:   script,
		Hash:     h,
		Manifest: *m,
		ID:       42,
	}

	currScript := []byte{byte(opcode.RET)}
	m = manifest.NewManifest("TestAux")
	perm := manifest.NewPermission(manifest.PermissionHash, h)
	perm.Methods.Add("add")
	perm.Methods.Add("drop")
	perm.Methods.Add("add3")
	perm.Methods.Add("invalidReturn")
	perm.Methods.Add("justReturn")
	perm.Methods.Add("getValue")
	m.Permissions = append(m.Permissions, *perm)

	return cs, &state.Contract{
		Script:   currScript,
		Hash:     hash.Hash160(currScript),
		Manifest: *m,
		ID:       123,
	}
}

func loadScript(ic *interop.Context, script []byte, args ...interface{}) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithFlags(script, smartcontract.AllowCall)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func loadScriptWithHashAndFlags(ic *interop.Context, script []byte, hash util.Uint160, f smartcontract.CallFlag, args ...interface{}) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithHash(script, hash, f)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func TestContractCall(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	cs, currCs := getTestContractState()
	require.NoError(t, ic.DAO.PutContractState(cs))
	require.NoError(t, ic.DAO.PutContractState(currCs))

	currScript := currCs.Script
	h := hash.Hash160(cs.Script)

	addArgs := stackitem.NewArray([]stackitem.Item{stackitem.Make(1), stackitem.Make(2)})
	t.Run("Good", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(addArgs)
		ic.VM.Estack().PushVal("add")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(3), ic.VM.Estack().Pop().Value())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
	})

	t.Run("CallExInvalidFlag", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(byte(0xFF))
		ic.VM.Estack().PushVal(addArgs)
		ic.VM.Estack().PushVal("add")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.Error(t, contract.CallEx(ic))
	})

	runInvalid := func(args ...interface{}) func(t *testing.T) {
		return func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, currScript, h, smartcontract.All, 42)
			for i := range args {
				ic.VM.Estack().PushVal(args[i])
			}
			// interops can both return error and panic,
			// we don't care which kind of error has occurred
			require.Panics(t, func() {
				err := contract.Call(ic)
				if err != nil {
					panic(err)
				}
			})
		}
	}

	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", runInvalid(addArgs, "add", h.BytesBE()[1:]))
		t.Run("MissingHash", runInvalid(addArgs, "add", util.Uint160{}.BytesBE()))
		t.Run("Method", runInvalid(addArgs, stackitem.NewInterop("add"), h.BytesBE()))
		t.Run("MissingMethod", runInvalid(addArgs, "sub", h.BytesBE()))
		t.Run("DisallowedMethod", runInvalid(stackitem.NewArray(nil), "ret7", h.BytesBE()))
		t.Run("Arguments", runInvalid(1, "add", h.BytesBE()))
		t.Run("NotEnoughArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{stackitem.Make(1)}), "add", h.BytesBE()))
	})

	t.Run("ReturnValues", func(t *testing.T) {
		t.Run("Many", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal("invalidReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.Error(t, ic.VM.Run())
		})
		t.Run("Void", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal("justReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, stackitem.Null{}, ic.VM.Estack().Pop().Item())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("IsolatedStack", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray(nil))
		ic.VM.Estack().PushVal("drop")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.Error(t, ic.VM.Run())
	})

	t.Run("CallInitialize", func(t *testing.T) {
		t.Run("Directly", runInvalid(stackitem.NewArray([]stackitem.Item{}), "_initialize", h.BytesBE()))

		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.Make(5)}))
		ic.VM.Estack().PushVal("add3")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(8), ic.VM.Estack().Pop().Value())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
	})
}

func TestContractCreate(t *testing.T) {
	v, cs, ic, bc := createVMAndContractState(t)
	v.GasLimit = -1
	defer bc.Close()

	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"
	putArgsOnStack := func() {
		manifest, err := json.Marshal(cs.Manifest)
		require.NoError(t, err)
		ne, err := nef.NewFile(cs.Script)
		require.NoError(t, err)
		neb, err := ne.Bytes()
		require.NoError(t, err)
		v.Estack().PushVal(manifest)
		v.Estack().PushVal(neb)
	}

	t.Run("no tx", func(t *testing.T) {
		putArgsOnStack()

		require.Error(t, contractCreate(ic))
	})

	ic.Tx = transaction.New(netmode.UnitTestNet, []byte{1}, 0)
	var sender = util.Uint160{1, 2, 3}
	ic.Tx.Signers = append(ic.Tx.Signers, transaction.Signer{Account: sender})
	cs.ID = 0
	cs.Hash = state.CreateContractHash(sender, cs.Script)

	t.Run("positive", func(t *testing.T) {
		putArgsOnStack()

		require.NoError(t, contractCreate(ic))
		actual := v.Estack().Pop().Item()
		compareContractStates(t, cs, actual)
	})

	t.Run("contract already exists", func(t *testing.T) {
		putArgsOnStack()

		require.Error(t, contractCreate(ic))
	})
}

func compareContractStates(t *testing.T, expected *state.Contract, actual stackitem.Item) {
	act, ok := actual.Value().([]stackitem.Item)
	require.True(t, ok)

	expectedManifest, err := json.Marshal(expected.Manifest)
	require.NoError(t, err)

	require.Equal(t, 5, len(act))
	require.Equal(t, expected.ID, int32(act[0].Value().(*big.Int).Int64()))
	require.Equal(t, expected.UpdateCounter, uint16(act[1].Value().(*big.Int).Int64()))
	require.Equal(t, expected.Hash.BytesBE(), act[2].Value().([]byte))
	require.Equal(t, expected.Script, act[3].Value().([]byte))
	require.Equal(t, expectedManifest, act[4].Value().([]byte))
}

func TestContractUpdate(t *testing.T) {
	v, cs, ic, bc := createVMAndContractState(t)
	defer bc.Close()
	v.GasLimit = -1

	putArgsOnStack := func(script, manifest interface{}) {
		v.Estack().PushVal(manifest)
		b, ok := script.([]byte)
		if ok {
			ne, err := nef.NewFile(b)
			require.NoError(t, err)
			script, err = ne.Bytes()
			require.NoError(t, err)
		}
		v.Estack().PushVal(script)
	}

	t.Run("no args", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		putArgsOnStack(stackitem.Null{}, stackitem.Null{})
		require.Error(t, contractUpdate(ic))
	})

	t.Run("no contract", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, util.Uint160{8, 9, 7}, smartcontract.All)
		putArgsOnStack([]byte{1}, stackitem.Null{})
		require.Error(t, contractUpdate(ic))
	})

	t.Run("too large script", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		putArgsOnStack(make([]byte, MaxContractScriptSize+1), stackitem.Null{})
		require.Error(t, contractUpdate(ic))
	})

	t.Run("too large manifest", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		putArgsOnStack(stackitem.Null{}, make([]byte, manifest.MaxManifestSize+1))
		require.Error(t, contractUpdate(ic))
	})

	t.Run("gas limit exceeded", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.GasLimit = 0
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		putArgsOnStack([]byte{1}, []byte{2})
		require.Error(t, contractUpdate(ic))
	})

	v.GasLimit = -1
	t.Run("update script, positive", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		t.Run("empty manifest", func(t *testing.T) {
			v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
			newScript := []byte{9, 8, 7, 6, 5}
			putArgsOnStack(newScript, []byte{})
			require.Error(t, contractUpdate(ic))
		})

		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		newScript := []byte{9, 8, 7, 6, 5}
		putArgsOnStack(newScript, stackitem.Null{})

		require.NoError(t, contractUpdate(ic))

		// updated contract should have the same scripthash
		actual, err := ic.DAO.GetContractState(cs.Hash)
		require.NoError(t, err)
		expected := &state.Contract{
			ID:            cs.ID,
			UpdateCounter: 1,
			Hash:          cs.Hash,
			Script:        newScript,
			Manifest:      cs.Manifest,
		}
		require.Equal(t, expected, actual)
	})

	t.Run("update manifest, bad manifest", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		putArgsOnStack(stackitem.Null{}, []byte{1, 2, 3})

		require.Error(t, contractUpdate(ic))
	})

	t.Run("update manifest, positive", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		manifest := &manifest.Manifest{
			ABI: manifest.ABI{},
		}
		manifestBytes, err := json.Marshal(manifest)
		require.NoError(t, err)

		t.Run("empty script", func(t *testing.T) {
			v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
			putArgsOnStack([]byte{}, manifestBytes)
			require.Error(t, contractUpdate(ic))
		})

		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		putArgsOnStack(stackitem.Null{}, manifestBytes)
		require.NoError(t, contractUpdate(ic))

		// updated contract should have old scripthash
		actual, err := ic.DAO.GetContractState(cs.Hash)
		require.NoError(t, err)
		expected := &state.Contract{
			ID:            cs.ID,
			UpdateCounter: 2,
			Hash:          cs.Hash,
			Script:        cs.Script,
			Manifest:      *manifest,
		}
		require.Equal(t, expected, actual)
	})

	t.Run("update both script and manifest", func(t *testing.T) {
		require.NoError(t, ic.DAO.PutContractState(cs))
		v.LoadScriptWithHash([]byte{byte(opcode.RET)}, cs.Hash, smartcontract.All)
		newScript := []byte{12, 13, 14}
		newManifest := manifest.Manifest{
			ABI: manifest.ABI{},
		}
		newManifestBytes, err := json.Marshal(newManifest)
		require.NoError(t, err)

		putArgsOnStack(newScript, newManifestBytes)

		require.NoError(t, contractUpdate(ic))

		// updated contract should have new script and manifest
		actual, err := ic.DAO.GetContractState(cs.Hash)
		require.NoError(t, err)
		expected := &state.Contract{
			ID:            cs.ID,
			UpdateCounter: 3,
			Hash:          cs.Hash,
			Script:        newScript,
			Manifest:      newManifest,
		}
		require.Equal(t, expected, actual)
	})
}

// TestContractCreateDeploy checks that `_deploy` method was called
// during contract creation or update.
func TestContractCreateDeploy(t *testing.T) {
	v, ic, bc := createVM(t)
	defer bc.Close()
	v.GasLimit = -1

	putArgs := func(cs *state.Contract) {
		rawManifest, err := json.Marshal(cs.Manifest)
		require.NoError(t, err)
		v.Estack().PushVal(rawManifest)
		ne, err := nef.NewFile(cs.Script)
		require.NoError(t, err)
		b, err := ne.Bytes()
		require.NoError(t, err)
		v.Estack().PushVal(b)
	}
	cs, currCs := getTestContractState()

	ic.Tx = transaction.New(netmode.UnitTestNet, []byte{1}, 0)
	var sender = util.Uint160{1, 2, 3}
	ic.Tx.Signers = append(ic.Tx.Signers, transaction.Signer{Account: sender})
	v.LoadScriptWithFlags([]byte{byte(opcode.RET)}, smartcontract.All)
	putArgs(cs)
	require.NoError(t, contractCreate(ic))
	require.NoError(t, ic.VM.Run())

	cs.Hash = state.CreateContractHash(sender, cs.Script)
	v.LoadScriptWithHash(currCs.Script, cs.Hash, smartcontract.All)
	err := contract.CallExInternal(ic, cs, "getValue", nil, smartcontract.All, vm.EnsureNotEmpty, nil)
	require.NoError(t, err)
	require.NoError(t, v.Run())
	require.Equal(t, "create", v.Estack().Pop().String())

	v.LoadScriptWithHash(cs.Script, cs.Hash, smartcontract.All)
	md := cs.Manifest.ABI.GetMethod("justReturn")
	v.Jump(v.Context(), md.Offset)

	t.Run("Update", func(t *testing.T) {
		newCs := &state.Contract{
			ID:       cs.ID,
			Hash:     cs.Hash,
			Script:   append(cs.Script, byte(opcode.RET)),
			Manifest: cs.Manifest,
		}
		putArgs(newCs)
		require.NoError(t, contractUpdate(ic))
		require.NoError(t, v.Run())

		v.LoadScriptWithHash(currCs.Script, cs.Hash, smartcontract.All)
		err = contract.CallExInternal(ic, newCs, "getValue", nil, smartcontract.All, vm.EnsureNotEmpty, nil)
		require.NoError(t, err)
		require.NoError(t, v.Run())
		require.Equal(t, "update", v.Estack().Pop().String())
	})
}

func TestContractGetCallFlags(t *testing.T) {
	v, ic, bc := createVM(t)
	defer bc.Close()

	v.LoadScriptWithHash([]byte{byte(opcode.RET)}, util.Uint160{1, 2, 3}, smartcontract.All)
	require.NoError(t, contractGetCallFlags(ic))
	require.Equal(t, int64(smartcontract.All), v.Estack().Pop().Value().(*big.Int).Int64())
}

func TestPointerCallback(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	script := []byte{
		byte(opcode.NOP), byte(opcode.INC), byte(opcode.RET),
		byte(opcode.DIV), byte(opcode.RET),
	}
	t.Run("Good", func(t *testing.T) {
		loadScript(ic, script, 2, stackitem.NewPointer(3, script))
		ic.VM.Estack().PushVal(ic.VM.Context())
		require.NoError(t, callback.Create(ic))

		args := stackitem.NewArray([]stackitem.Item{stackitem.Make(3), stackitem.Make(12)})
		ic.VM.Estack().InsertAt(vm.NewElement(args), 1)
		require.NoError(t, callback.Invoke(ic))

		require.NoError(t, ic.VM.Run())
		require.Equal(t, 1, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(5), ic.VM.Estack().Pop().Item().Value())
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("NotEnoughParameters", func(t *testing.T) {
			loadScript(ic, script, 2, stackitem.NewPointer(3, script))
			ic.VM.Estack().PushVal(ic.VM.Context())
			require.NoError(t, callback.Create(ic))

			args := stackitem.NewArray([]stackitem.Item{stackitem.Make(3)})
			ic.VM.Estack().InsertAt(vm.NewElement(args), 1)
			require.Error(t, callback.Invoke(ic))
		})
	})

}

func TestMethodCallback(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	cs, currCs := getTestContractState()
	require.NoError(t, ic.DAO.PutContractState(cs))
	require.NoError(t, ic.DAO.PutContractState(currCs))

	ic.Functions = append(ic.Functions, systemInterops)
	rawHash := hash.Hash160(cs.Script).BytesBE()

	t.Run("Invalid", func(t *testing.T) {
		runInvalid := func(args ...interface{}) func(t *testing.T) {
			return func(t *testing.T) {
				loadScript(ic, currCs.Script, 42)
				for i := range args {
					ic.VM.Estack().PushVal(args[i])
				}
				require.Error(t, callback.CreateFromMethod(ic))
			}
		}
		t.Run("Hash", runInvalid("add", rawHash[1:]))
		t.Run("MissingHash", runInvalid("add", util.Uint160{}.BytesBE()))
		t.Run("MissingMethod", runInvalid("sub", rawHash))
		t.Run("DisallowedMethod", runInvalid("ret7", rawHash))
		t.Run("Initialize", runInvalid("_initialize", rawHash))
		t.Run("NotEnoughArguments", func(t *testing.T) {
			loadScript(ic, currCs.Script, 42, "add", rawHash)
			require.NoError(t, callback.CreateFromMethod(ic))

			ic.VM.Estack().InsertAt(vm.NewElement(stackitem.NewArray([]stackitem.Item{stackitem.Make(1)})), 1)
			require.Error(t, callback.Invoke(ic))
		})
		t.Run("CallIsNotAllowed", func(t *testing.T) {
			ic.SpawnVM()
			ic.VM.Load(currCs.Script)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(rawHash)
			require.NoError(t, callback.CreateFromMethod(ic))

			args := stackitem.NewArray([]stackitem.Item{stackitem.Make(1), stackitem.Make(5)})
			ic.VM.Estack().InsertAt(vm.NewElement(args), 1)
			require.Error(t, callback.Invoke(ic))
		})
	})

	t.Run("Good", func(t *testing.T) {
		loadScript(ic, currCs.Script, 42, "add", rawHash)
		require.NoError(t, callback.CreateFromMethod(ic))

		args := stackitem.NewArray([]stackitem.Item{stackitem.Make(1), stackitem.Make(5)})
		ic.VM.Estack().InsertAt(vm.NewElement(args), 1)

		require.NoError(t, callback.Invoke(ic))
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(6), ic.VM.Estack().Pop().Item().Value())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Item().Value())
	})
}
func TestSyscallCallback(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	ic.Functions = append(ic.Functions, []interop.Function{
		{
			ID: 0x42,
			Func: func(ic *interop.Context) error {
				a := ic.VM.Estack().Pop().BigInt()
				b := ic.VM.Estack().Pop().BigInt()
				ic.VM.Estack().PushVal(new(big.Int).Add(a, b))
				return nil
			},
			ParamCount: 2,
		},
		{
			ID:               0x53,
			Func:             func(_ *interop.Context) error { return nil },
			DisallowCallback: true,
		},
	})

	t.Run("Good", func(t *testing.T) {
		args := stackitem.NewArray([]stackitem.Item{stackitem.Make(12), stackitem.Make(30)})
		loadScript(ic, []byte{byte(opcode.RET)}, args, 0x42)
		require.NoError(t, callback.CreateFromSyscall(ic))
		require.NoError(t, callback.Invoke(ic))
		require.Equal(t, 1, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Item().Value())
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Run("InvalidParameterCount", func(t *testing.T) {
			args := stackitem.NewArray([]stackitem.Item{stackitem.Make(12)})
			loadScript(ic, []byte{byte(opcode.RET)}, args, 0x42)
			require.NoError(t, callback.CreateFromSyscall(ic))
			require.Error(t, callback.Invoke(ic))
		})
		t.Run("MissingSyscall", func(t *testing.T) {
			loadScript(ic, []byte{byte(opcode.RET)}, stackitem.NewArray(nil), 0x43)
			require.Error(t, callback.CreateFromSyscall(ic))
		})
		t.Run("Disallowed", func(t *testing.T) {
			loadScript(ic, []byte{byte(opcode.RET)}, stackitem.NewArray(nil), 0x53)
			require.Error(t, callback.CreateFromSyscall(ic))
		})
	})
}

func TestRuntimeCheckWitness(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	script := []byte{byte(opcode.RET)}
	scriptHash := hash.Hash160(script)
	check := func(t *testing.T, ic *interop.Context, arg interface{}, shouldFail bool, expected ...bool) {
		ic.VM.Estack().PushVal(arg)
		err := runtime.CheckWitness(ic)
		if shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, expected)
			actual, ok := ic.VM.Estack().Pop().Value().(bool)
			require.True(t, ok)
			require.Equal(t, expected[0], actual)
		}
	}
	t.Run("error", func(t *testing.T) {
		t.Run("not a hash or key", func(t *testing.T) {
			check(t, ic, []byte{1, 2, 3}, true)
		})
		t.Run("script container is not a transaction", func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, script, scriptHash, smartcontract.AllowStates)
			check(t, ic, random.Uint160().BytesBE(), true)
		})
		t.Run("check scope", func(t *testing.T) {
			t.Run("CustomGroups, missing AllowStates flag", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:       hash,
							Scopes:        transaction.CustomGroups,
							AllowedGroups: []*keys.PublicKey{},
						},
					},
				}
				ic.Container = tx
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, smartcontract.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), smartcontract.AllowCall)
				check(t, ic, hash.BytesBE(), true)
			})
			t.Run("CustomGroups, unknown contract", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:       hash,
							Scopes:        transaction.CustomGroups,
							AllowedGroups: []*keys.PublicKey{},
						},
					},
				}
				ic.Container = tx
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, smartcontract.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), smartcontract.AllowStates)
				check(t, ic, hash.BytesBE(), true)
			})
		})
	})
	t.Run("positive", func(t *testing.T) {
		t.Run("calling scripthash", func(t *testing.T) {
			t.Run("hashed witness", func(t *testing.T) {
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, smartcontract.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), smartcontract.All)
				check(t, ic, callingScriptHash.BytesBE(), false, true)
			})
			t.Run("keyed witness", func(t *testing.T) {
				pk, err := keys.NewPrivateKey()
				require.NoError(t, err)
				callingScriptHash := pk.PublicKey().GetScriptHash()
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, smartcontract.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), smartcontract.All)
				check(t, ic, pk.PublicKey().Bytes(), false, true)
			})
		})
		t.Run("check scope", func(t *testing.T) {
			t.Run("Global", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.Global,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, smartcontract.AllowStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CalledByEntry", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.CalledByEntry,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, smartcontract.AllowStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CustomContracts", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:          hash,
							Scopes:           transaction.CustomContracts,
							AllowedContracts: []util.Uint160{scriptHash},
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, smartcontract.AllowStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CustomGroups", func(t *testing.T) {
				t.Run("empty calling scripthash", func(t *testing.T) {
					hash := random.Uint160()
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account:       hash,
								Scopes:        transaction.CustomGroups,
								AllowedGroups: []*keys.PublicKey{},
							},
						},
					}
					loadScriptWithHashAndFlags(ic, script, scriptHash, smartcontract.AllowStates)
					ic.Container = tx
					check(t, ic, hash.BytesBE(), false, false)
				})
				t.Run("positive", func(t *testing.T) {
					targetHash := random.Uint160()
					pk, err := keys.NewPrivateKey()
					require.NoError(t, err)
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account:       targetHash,
								Scopes:        transaction.CustomGroups,
								AllowedGroups: []*keys.PublicKey{pk.PublicKey()},
							},
						},
					}
					contractScript := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
					contractScriptHash := hash.Hash160(contractScript)
					contractState := &state.Contract{
						ID:     15,
						Hash:   contractScriptHash,
						Script: contractScript,
						Manifest: manifest.Manifest{
							Groups: []manifest.Group{{PublicKey: pk.PublicKey()}},
						},
					}
					require.NoError(t, ic.DAO.PutContractState(contractState))
					loadScriptWithHashAndFlags(ic, contractScript, contractScriptHash, smartcontract.All)
					ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), smartcontract.AllowStates)
					ic.Container = tx
					check(t, ic, targetHash.BytesBE(), false, true)
				})
			})
			t.Run("bad scope", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.None,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, smartcontract.AllowStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, false)
			})
		})
	})
}
