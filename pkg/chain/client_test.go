package chain

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	phrase = "hire useless peanut engine amused fuel wet toddler list party salmon dream"
)

func TestTransfer(t *testing.T) {
	require := require.New(t)
	c, err := NewChainClient("ws://221.122.79.5:9944/", phrase, 11330)
	require.NoError(err)
	blockHash, txHash, err := c.TransferBySs58Address("cXie7akQvDPoWqwcaPUHx1GqugFHQKn3a9wPyRUYZQL4scmgU", big.NewInt(2))
	require.NoError(err)
	t.Log(blockHash, txHash)
}
