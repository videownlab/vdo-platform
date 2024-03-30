package chain

import (
	"errors"
	"time"

	"vdo-platform/pkg/log"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/go-logr/logr"
	"github.com/goccy/go-json"
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
	var ext types.Extrinsic
	var txhash string
	if ext.UnmarshalJSON([]byte(signtx)) != nil {
		bytes, _ := json.Marshal(signtx)
		if err := ext.UnmarshalJSON(bytes); err != nil {
			logger.Error(err, "send tx error")
			return txhash, err
		}
	}
	rpcWorkPool <- struct{}{}
	defer func() {
		<-rpcWorkPool
	}()
	sub, err := c.api.RPC.Author.SubmitAndWatchExtrinsic(ext)
	if err != nil {
		logger.Error(err, "send tx error")
		return txhash, err
	}
	timeout := time.After(c.timeForBlockOut)
	for {
		select {
		case status := <-sub.Chan():
			if status.IsInBlock {
				txhash, _ = codec.EncodeToHex(status.AsInBlock)
				events := types.EventRecords{}
				h, err := c.api.RPC.State.GetStorageRaw(c.eventsKey, status.AsInBlock)
				if err != nil {
					logger.Error(err, "send tx error", "tx", txhash)
					return txhash, err
				}
				types.EventRecordsRaw(*h).DecodeEventRecords(c.metadata, &events)
				if len(events.System_ExtrinsicFailed) > 0 {
					err := errors.New("system.ExtrinsicFailed")
					logger.Error(err, "send tx error", "tx", txhash)
					return txhash, err
				}
				if len(events.System_ExtrinsicSuccess) > 0 {
					logger.V(1).Info("send tx success", "tx", txhash)
					return txhash, err
				}
			}
		case err := <-sub.Err():
			logger.Error(err, "send tx error")
			return txhash, err
		case <-timeout:
			err := errors.New("send tx timeout")
			logger.Error(err, "send tx timeout")
			return txhash, err
		}
	}
}
