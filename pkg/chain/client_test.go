package chain

import (
	"testing"
	"time"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vedhavyas/go-subkey/v2"
)

var (
	phrase = "hire useless peanut engine amused fuel wet toddler list party salmon dream"
)

func TestTransfer(t *testing.T) {
	require := require.New(t)
	c, err := NewChainClient("ws://221.122.79.5:9944/", phrase, 11330)
	require.NoError(err)
	blockHash, err := c.TransferBySs58Address("cXie7akQvDPoWqwcaPUHx1GqugFHQKn3a9wPyRUYZQL4scmgU", 100)
	require.NoError(err)
	t.Log(blockHash)
}

func TestTransferRaw(t *testing.T) {
	_, pubkey, err := subkey.SS58Decode("cXie7akQvDPoWqwcaPUHx1GqugFHQKn3a9wPyRUYZQL4scmgU")
	assert.NoError(t, err)

	target, err := types.NewMultiAddressFromAccountID(pubkey)
	assert.NoError(t, err)

	from, err := signature.KeyringPairFromSecret(phrase, 11330)
	assert.NoError(t, err)

	api, err := gsrpc.NewSubstrateAPI("ws://221.122.79.5:9944/")
	assert.NoError(t, err)

	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	assert.NoError(t, err)

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	assert.NoError(t, err)

	meta, err := api.RPC.State.GetMetadataLatest()
	assert.NoError(t, err)

	call, err := types.NewCall(meta, "Balances.transfer", target, types.NewUCompactFromUInt(6969))
	assert.NoError(t, err)

	accountKey, err := types.CreateStorageKey(meta, "System", "Account", from.PublicKey)
	assert.NoError(t, err)

	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(accountKey, &accountInfo)
	assert.NoError(t, err)
	assert.True(t, ok)

	ext := types.NewExtrinsic(call)
	nonce := uint32(accountInfo.Nonce)
	o := types.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	err = ext.Sign(from, o)
	assert.NoError(t, err)

	sub, err := api.RPC.Author.SubmitAndWatchExtrinsic(ext)
	require.NoError(t, err)

	defer sub.Unsubscribe()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case status := <-sub.Chan():
			if status.IsInBlock {
				t.Logf("blockHash: %s\n", status.AsInBlock.Hex())
				assert.False(t, codec.Eq(status.AsInBlock, types.ExtrinsicStatus{}.AsInBlock),
					"expected AsFinalized not to be empty")
				return
			}
		case <-timeout:
			assert.FailNow(t, "timeout of 10 seconds reached without getting finalized status for extrinsic")
			return
		}
	}

}
