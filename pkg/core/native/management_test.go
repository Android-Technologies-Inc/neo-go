package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestDeployGetUpdateDestroyContract(t *testing.T) {
	mgmt := newManagement()
	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	script := []byte{1}
	sender := util.Uint160{1, 2, 3}
	h := state.CreateContractHash(sender, script)

	ne, err := nef.NewFile(script)
	require.NoError(t, err)
	manif := manifest.NewManifest("Test")
	require.NoError(t, err)

	contract, err := mgmt.Deploy(d, sender, ne, manif)
	require.NoError(t, err)
	require.Equal(t, int32(1), contract.ID)
	require.Equal(t, uint16(0), contract.UpdateCounter)
	require.Equal(t, h, contract.Hash)
	require.Equal(t, script, contract.Script)
	require.Equal(t, *manif, contract.Manifest)

	// Double deploy.
	_, err = mgmt.Deploy(d, sender, ne, manif)
	require.Error(t, err)

	// Different sender.
	sender2 := util.Uint160{3, 2, 1}
	contract2, err := mgmt.Deploy(d, sender2, ne, manif)
	require.NoError(t, err)
	require.Equal(t, int32(2), contract2.ID)
	require.Equal(t, uint16(0), contract2.UpdateCounter)
	require.Equal(t, state.CreateContractHash(sender2, script), contract2.Hash)
	require.Equal(t, script, contract2.Script)
	require.Equal(t, *manif, contract2.Manifest)

	refContract, err := mgmt.GetContract(d, h)
	require.NoError(t, err)
	require.Equal(t, contract, refContract)

	upContract, err := mgmt.Update(d, h, ne, manif)
	refContract.UpdateCounter++
	require.NoError(t, err)
	require.Equal(t, refContract, upContract)

	err = mgmt.Destroy(d, h)
	require.NoError(t, err)
	_, err = mgmt.GetContract(d, h)
	require.Error(t, err)
}

func TestManagement_Initialize(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
		ic := &interop.Context{DAO: dao.NewCached(d)}
		mgmt := newManagement()
		require.NoError(t, mgmt.Initialize(ic))
	})
	t.Run("invalid contract state", func(t *testing.T) {
		d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
		ic := &interop.Context{DAO: dao.NewCached(d)}
		mgmt := newManagement()
		require.NoError(t, ic.DAO.PutStorageItem(mgmt.ContractID, []byte{prefixContract}, &state.StorageItem{Value: []byte{0xFF}}))
		require.Error(t, mgmt.Initialize(ic))
	})
}
