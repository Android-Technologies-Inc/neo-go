package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Call calls specified native contract method.
func Call(ic *interop.Context) error {
	version := ic.VM.Estack().Pop().BigInt().Int64()
	if version != 0 {
		return fmt.Errorf("native contract of version %d is not active", version)
	}
	var c interop.Contract
	for _, ctr := range ic.Natives {
		if ctr.Metadata().Hash == ic.VM.GetCurrentScriptHash() {
			c = ctr
			break
		}
	}
	if c == nil {
		return fmt.Errorf("native contract %d not found", version)
	}
	m, ok := c.Metadata().GetMethodByOffset(ic.VM.Context().IP())
	if !ok {
		return fmt.Errorf("method not found")
	}
	if !ic.VM.Context().GetCallFlags().Has(m.RequiredFlags) {
		return fmt.Errorf("missing call flags for native %d `%s` operation call: %05b vs %05b",
			version, m.MD.Name, ic.VM.Context().GetCallFlags(), m.RequiredFlags)
	}
	// Native contract prices are not multiplied by `BaseExecFee`.
	if !ic.VM.AddGas(m.Price) {
		return errors.New("gas limit exceeded")
	}
	ctx := ic.VM.Context()
	args := make([]stackitem.Item, len(m.MD.Parameters))
	for i := range args {
		args[i] = ic.VM.Estack().Pop().Item()
	}
	result := m.Func(ic, args)
	if m.MD.ReturnType != smartcontract.VoidType {
		ctx.Estack().PushVal(result)
	}
	return nil
}

// OnPersist calls OnPersist methods for all native contracts.
func OnPersist(ic *interop.Context) error {
	if ic.Trigger != trigger.OnPersist {
		return errors.New("onPersist must be trigered by system")
	}
	for _, c := range ic.Natives {
		err := c.OnPersist(ic)
		if err != nil {
			return err
		}
	}
	return nil
}

// PostPersist calls PostPersist methods for all native contracts.
func PostPersist(ic *interop.Context) error {
	if ic.Trigger != trigger.PostPersist {
		return errors.New("postPersist must be trigered by system")
	}
	for _, c := range ic.Natives {
		err := c.PostPersist(ic)
		if err != nil {
			return err
		}
	}
	return nil
}
