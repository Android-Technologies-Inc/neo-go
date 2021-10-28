package request

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// Param represents a param either passed to
	// the server or to send to a server using
	// the client.
	Param struct {
		json.RawMessage
	}

	paramType int
	// FuncParam represents a function argument parameter used in the
	// invokefunction RPC method.
	FuncParam struct {
		Type  smartcontract.ParamType `json:"type"`
		Value Param                   `json:"value"`
	}
	// BlockFilter is a wrapper structure for block event filter. The only
	// allowed filter is primary index.
	BlockFilter struct {
		Primary int `json:"primary"`
	}
	// TxFilter is a wrapper structure for transaction event filter. It
	// allows to filter transactions by senders and signers.
	TxFilter struct {
		Sender *util.Uint160 `json:"sender,omitempty"`
		Signer *util.Uint160 `json:"signer,omitempty"`
	}
	// NotificationFilter is a wrapper structure representing filter used for
	// notifications generated during transaction execution. Notifications can
	// be filtered by contract hash and by name.
	NotificationFilter struct {
		Contract *util.Uint160 `json:"contract,omitempty"`
		Name     *string       `json:"name,omitempty"`
	}
	// ExecutionFilter is a wrapper structure used for transaction execution
	// events. It allows to choose failing or successful transactions based
	// on their VM state.
	ExecutionFilter struct {
		State string `json:"state"`
	}
	// SignerWithWitness represents transaction's signer with the corresponding witness.
	SignerWithWitness struct {
		transaction.Signer
		transaction.Witness
	}
)

// These are parameter types accepted by RPC server.
const (
	defaultT paramType = iota
	StringT
	NumberT
	BooleanT
	ArrayT
	FuncParamT
	BlockFilterT
	TxFilterT
	NotificationFilterT
	ExecutionFilterT
	SignerWithWitnessT
)

var (
	errMissingParameter = errors.New("parameter is missing")
	errNotAString       = errors.New("not a string")
	errNotAnInt         = errors.New("not an integer")
	errNotABool         = errors.New("not a boolean")
)

func (p Param) String() string {
	return string(p.RawMessage)
}

// GetString returns string value of the parameter.
func (p *Param) GetString() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	var s string
	err := json.Unmarshal(p.RawMessage, &s)
	if err != nil {
		return "", errNotAString
	}
	return s, nil
}

// AsString returns string value of the parameter or tries to cast parameter to a string value.
func (p *Param) AsString() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	var s string
	err := json.Unmarshal(p.RawMessage, &s)
	if err == nil {
		return s, nil
	}
	var i int
	err = json.Unmarshal(p.RawMessage, &i)
	if err == nil {
		return strconv.Itoa(i), nil
	}
	var b bool
	err = json.Unmarshal(p.RawMessage, &b)
	if err == nil {
		switch b {
		case true:
			return "true", nil
		case false:
			return "false", nil
		}
	}
	// TODO: array can also be converted to a string in C#
	return "", errNotAString
}

// GetBool returns boolean value of the parameter.
func (p *Param) GetBool() (bool, error) {
	if p == nil {
		return false, errMissingParameter
	}
	var b bool
	err := json.Unmarshal(p.RawMessage, &b)
	if err != nil {
		return b, errNotABool
	}
	return b, nil
}

// AsBool returns boolean value of the parameter or tries to cast parameter to a bool value.
func (p *Param) AsBool() (bool, error) {
	if p == nil {
		return false, errMissingParameter
	}
	var b bool
	err := json.Unmarshal(p.RawMessage, &b)
	if err == nil {
		return b, nil
	}
	var s string
	err = json.Unmarshal(p.RawMessage, &s)
	if err == nil {
		return s != "", nil
	}
	var i int
	err = json.Unmarshal(p.RawMessage, &i)
	if err == nil {
		return i != 0, nil
	}
	return false, errNotABool
}

// GetInt returns int value of the parameter if the parameter is integer.
func (p *Param) GetInt() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	var i int
	err := json.Unmarshal(p.RawMessage, &i)
	if err != nil {
		return i, errNotAnInt
	}
	return i, nil
}

// AsInt returns int value of the parameter or tries to cast parameter to an int value.
func (p *Param) AsInt() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	var i int
	err := json.Unmarshal(p.RawMessage, &i)
	if err == nil {
		return i, nil
	}
	var s string
	err = json.Unmarshal(p.RawMessage, &s)
	if err == nil {
		return strconv.Atoi(s)
	}
	var b bool
	err = json.Unmarshal(p.RawMessage, &b)
	if err == nil {
		i = 0
		if b {
			i = 1
		}
		return i, nil
	}
	return 0, errNotAnInt
}

// GetArray returns a slice of Params stored in the parameter.
func (p *Param) GetArray() ([]Param, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	a := []Param{}
	err := json.Unmarshal(p.RawMessage, &a)
	if err != nil {
		return nil, errors.New("not an array")
	}
	return a, nil
}

// GetUint256 returns Uint256 value of the parameter.
func (p *Param) GetUint256() (util.Uint256, error) {
	s, err := p.AsString()
	if err != nil {
		return util.Uint256{}, err
	}

	return util.Uint256DecodeStringLE(strings.TrimPrefix(s, "0x"))
}

// GetUint160FromHex returns Uint160 value of the parameter encoded in hex.
func (p *Param) GetUint160FromHex() (util.Uint160, error) {
	s, err := p.AsString()
	if err != nil {
		return util.Uint160{}, err
	}
	if len(s) == 2*util.Uint160Size+2 && s[0] == '0' && s[1] == 'x' {
		s = s[2:]
	}

	return util.Uint160DecodeStringLE(s)
}

// GetUint160FromAddress returns Uint160 value of the parameter that was
// supplied as an address.
func (p *Param) GetUint160FromAddress() (util.Uint160, error) {
	s, err := p.AsString()
	if err != nil {
		return util.Uint160{}, err
	}

	return address.StringToUint160(s)
}

// GetUint160FromAddressOrHex returns Uint160 value of the parameter that was
// supplied either as raw hex or as an address.
func (p *Param) GetUint160FromAddressOrHex() (util.Uint160, error) {
	u, err := p.GetUint160FromHex()
	if err == nil {
		return u, err
	}
	return p.GetUint160FromAddress()
}

// GetFuncParam returns current parameter as a function call parameter.
func (p *Param) GetFuncParam() (FuncParam, error) {
	if p == nil {
		return FuncParam{}, errMissingParameter
	}
	fp := FuncParam{}
	err := json.Unmarshal(p.RawMessage, &fp)
	return fp, err
}

// GetBytesHex returns []byte value of the parameter if
// it is a hex-encoded string.
func (p *Param) GetBytesHex() ([]byte, error) {
	s, err := p.AsString()
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

// GetBytesBase64 returns []byte value of the parameter if
// it is a base64-encoded string.
func (p *Param) GetBytesBase64() ([]byte, error) {
	s, err := p.AsString()
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(s)
}

// GetSignerWithWitness returns SignerWithWitness value of the parameter.
func (p *Param) GetSignerWithWitness() (SignerWithWitness, error) {
	aux := new(signerWithWitnessAux)
	err := json.Unmarshal(p.RawMessage, aux)
	if err != nil {
		return SignerWithWitness{}, errors.New("not a signer")
	}
	accParam := Param{[]byte(fmt.Sprintf(`"%s"`, aux.Account))}
	acc, err := accParam.GetUint160FromAddressOrHex()
	if err != nil {
		return SignerWithWitness{}, errors.New("not a signer")
	}
	c := SignerWithWitness{
		Signer: transaction.Signer{
			Account:          acc,
			Scopes:           aux.Scopes,
			AllowedContracts: aux.AllowedContracts,
			AllowedGroups:    aux.AllowedGroups,
		},
		Witness: transaction.Witness{
			InvocationScript:   aux.InvocationScript,
			VerificationScript: aux.VerificationScript,
		},
	}
	return c, nil
}

// GetSignersWithWitnesses returns a slice of SignerWithWitness with CalledByEntry
// scope from array of Uint160 or array of serialized transaction.Signer stored
// in the parameter.
func (p Param) GetSignersWithWitnesses() ([]transaction.Signer, []transaction.Witness, error) {
	hashes, err := p.GetArray()
	if err != nil {
		return nil, nil, err
	}
	signers := make([]transaction.Signer, len(hashes))
	witnesses := make([]transaction.Witness, len(hashes))
	// try to extract hashes first
	for i, h := range hashes {
		var u util.Uint160
		u, err = h.GetUint160FromHex()
		if err != nil {
			break
		}
		signers[i] = transaction.Signer{
			Account: u,
			Scopes:  transaction.CalledByEntry,
		}
	}
	if err != nil {
		for i, h := range hashes {
			signerWithWitness, err := h.GetSignerWithWitness()
			if err != nil {
				return nil, nil, err
			}
			signers[i] = signerWithWitness.Signer
			witnesses[i] = signerWithWitness.Witness
		}
	}
	return signers, witnesses, nil
}

// signerWithWitnessAux is an auxiluary struct for JSON marshalling. We need it because of
// DisallowUnknownFields JSON marshaller setting.
type signerWithWitnessAux struct {
	Account            string                   `json:"account"`
	Scopes             transaction.WitnessScope `json:"scopes"`
	AllowedContracts   []util.Uint160           `json:"allowedcontracts,omitempty"`
	AllowedGroups      []*keys.PublicKey        `json:"allowedgroups,omitempty"`
	InvocationScript   []byte                   `json:"invocation,omitempty"`
	VerificationScript []byte                   `json:"verification,omitempty"`
}

// MarshalJSON implements json.Unmarshaler interface.
func (s *SignerWithWitness) MarshalJSON() ([]byte, error) {
	signer := &signerWithWitnessAux{
		Account:            s.Account.StringLE(),
		Scopes:             s.Scopes,
		AllowedContracts:   s.AllowedContracts,
		AllowedGroups:      s.AllowedGroups,
		InvocationScript:   s.InvocationScript,
		VerificationScript: s.VerificationScript,
	}
	return json.Marshal(signer)
}
