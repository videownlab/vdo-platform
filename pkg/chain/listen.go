package chain

import (
	"vdo-platform/pkg/log"

	"github.com/go-logr/logr"
)

const rpcConnNum = 16 * 1024

var logger logr.Logger
var rpcWorkPool chan struct{}

func init() {
	logger = log.Logger.WithName("chain")
}

func InitRpcWorkPool() {
	if rpcWorkPool == nil {
		rpcWorkPool = make(chan struct{}, rpcConnNum)
	}
}

func (c *ChainClient) SendTx1(signtx string) (string, error) {
	return "", nil
}

func (c *ChainClient) SendTx2(signtx string) (string, error) {
	panic("xxx")
}
