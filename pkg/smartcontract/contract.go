package smartcontract

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
)

// CreateMultiSigRedeemScript creates a script runnable by the VM.
func CreateMultiSigRedeemScript(m int, publicKeys keys.PublicKeys) ([]byte, error) {
	if m <= 1 {
		return nil, fmt.Errorf("param m cannot be smaller or equal to 1 got %d", m)
	}
	if m > len(publicKeys) {
		return nil, fmt.Errorf("length of the signatures (%d) is higher then the number of public keys", m)
	}
	if m > 1024 {
		return nil, fmt.Errorf("public key count %d exceeds maximum of length 1024", len(publicKeys))
	}

	buf := new(bytes.Buffer)
	if err := vm.EmitInt(buf, int64(m)); err != nil {
		return nil, err
	}
	sort.Sort(publicKeys)
	for _, pubKey := range publicKeys {
		if err := vm.EmitBytes(buf, pubKey.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := vm.EmitInt(buf, int64(len(publicKeys))); err != nil {
		return nil, err
	}
	if err := vm.EmitOpcode(buf, opcode.CHECKMULTISIG); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func CreateBLSMultisigScript(m int, pubs [][]byte) ([]byte, error) {
	sort.Slice(pubs, func(i, j int) bool { return bytes.Compare(pubs[i], pubs[j]) == -1 })

	buf := new(bytes.Buffer)
	if err := vm.EmitInt(buf, int64(m)); err != nil {
		return nil, err
	}
	for i := range pubs {
		if err := vm.EmitBytes(buf, pubs[i]); err != nil {
			return nil, err
		}
	}

	if err := vm.EmitInt(buf, int64(len(pubs))); err != nil {
		return nil, err
	}
	if err := vm.EmitOpcode(buf, opcode.CHECKBLS); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}