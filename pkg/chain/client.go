/*
   Copyright 2022 CESS scheduler authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package chain

import (
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/rpc/author"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2"
	"golang.org/x/crypto/blake2b"
)

type ChainClient struct {
	lock            *sync.Mutex
	api             *gsrpc.SubstrateAPI
	chainState      *atomic.Bool
	metadata        *types.Metadata
	runtimeVersion  *types.RuntimeVersion
	eventsKey       types.StorageKey
	genesisHash     types.Hash
	keyring         signature.KeyringPair
	rpcAddr         string
	timeForBlockOut time.Duration
	networkId       uint16
}

func NewChainClient(rpcAddr, secret string, networkId uint16) (*ChainClient, error) {
	var (
		err error
		cli = &ChainClient{}
	)
	cli.api, err = gsrpc.NewSubstrateAPI(rpcAddr)
	if err != nil {
		return nil, err
	}
	cli.metadata, err = cli.api.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}
	cli.genesisHash, err = cli.api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return nil, err
	}
	cli.runtimeVersion, err = cli.api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return nil, err
	}
	cli.eventsKey, err = types.CreateStorageKey(
		cli.metadata,
		"System",
		"Events",
		nil,
	)
	if err != nil {
		return nil, err
	}
	if secret != "" {
		cli.keyring, err = signature.KeyringPairFromSecret(secret, 0)
		if err != nil {
			return nil, err
		}
	}
	cli.lock = new(sync.Mutex)
	cli.chainState = &atomic.Bool{}
	cli.chainState.Store(true)
	cli.timeForBlockOut = 15 * time.Second
	cli.rpcAddr = rpcAddr
	cli.networkId = networkId
	return cli, nil
}

func (c *ChainClient) IsChainClientOk() bool {
	err := healthchek(c.api)
	if err != nil {
		c.api = nil
		cli, err := reconnectChainClient(c.rpcAddr)
		if err != nil {
			return false
		}
		c.api = cli
		return true
	}
	return true
}

func (c *ChainClient) MakeSignatureOptions(nonce uint64) types.SignatureOptions {
	return types.SignatureOptions{
		BlockHash:          c.genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        c.genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        c.runtimeVersion.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: c.runtimeVersion.TransactionVersion,
	}
}

func (c *ChainClient) SetChainState(state bool) {
	c.chainState.Store(state)
}

func (c *ChainClient) GetChainState() bool {
	return c.chainState.Load()
}

func (c *ChainClient) NewAccountId(pubkey []byte) (*types.AccountID, error) {
	return types.NewAccountID(pubkey)
}

type multiTargetAddr struct {
	ss58addr string
	pubkey   []byte
}

func (c *ChainClient) TransferBySs58Address(target string, amount uint64) (string, error) {
	_, pubkey, err := subkey.SS58Decode(target)
	if err != nil {
		return "", err
	}
	ta := multiTargetAddr{ss58addr: target, pubkey: pubkey}
	return c.doTransfer(ta, amount)
}

func (c *ChainClient) Transfer(target types.AccountID, amount uint64) (string, error) {
	ss58addr := subkey.SS58Encode(target[:], c.networkId)
	ta := multiTargetAddr{ss58addr: ss58addr, pubkey: target[:]}
	return c.doTransfer(ta, amount)
}

func (c *ChainClient) doTransfer(target multiTargetAddr, amount uint64) (string, error) {
	logger := logger.WithName("transfer").WithValues("target", target.ss58addr, "amount", amount)
	targetAddr, err := types.NewMultiAddressFromAccountID(target.pubkey)
	if err != nil {
		return "", err
	}
	api := c.api
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return "", err
	}

	call, err := types.NewCall(meta, "Balances.transfer", targetAddr, types.NewUCompactFromUInt(amount))
	if err != nil {
		return "", err
	}

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return "", err
	}

	accountKey, err := types.CreateStorageKey(meta, "System", "Account", c.keyring.PublicKey)
	if err != nil {
		return "", err
	}

	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(accountKey, &accountInfo)
	if err != nil {
		return "", err
	}
	var nonce uint32
	if !ok {
		nonce = 1
	} else {
		nonce = uint32(accountInfo.Nonce)
	}

	var sub *author.ExtrinsicStatusSubscription
	for i := 0; i < 3; i++ {
		options := types.SignatureOptions{
			BlockHash:          c.genesisHash,
			Era:                types.ExtrinsicEra{IsMortalEra: false},
			GenesisHash:        c.genesisHash,
			Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
			SpecVersion:        rv.SpecVersion,
			Tip:                types.NewUCompactFromUInt(0),
			TransactionVersion: rv.TransactionVersion,
		}

		ext := types.NewExtrinsic(call)
		err = ext.Sign(c.keyring, options)
		if err != nil {
			return "", errors.Wrap(err, "sign a call error")
		}
		b, err := codec.Encode(ext)
		if err == nil {
			txHash := blake2b.Sum256(b)
			logger.Info("", "txHash", hex.EncodeToString(txHash[:]))
		}

		sub, err = api.RPC.Author.SubmitAndWatchExtrinsic(ext)
		if err != nil {
			nonce++
			logger.Error(err, "try again later", "nonce", nonce)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	defer func() {
		if sub != nil {
			sub.Unsubscribe()
		}
	}()
	const timeoutSecs = 18
	timeout := time.After(timeoutSecs * time.Second)
	for {
		select {
		case status := <-sub.Chan():
			if status.IsInBlock {
				blockHash := status.AsInBlock.Hex()
				logger.Info("transfer tx in block", "blockHash", blockHash)
				// txhash, _ = codec.EncodeToHex(status.AsInBlock)
				events := types.EventRecords{}
				blockData, err := api.RPC.State.GetStorageRaw(c.eventsKey, status.AsInBlock)
				if err != nil {
					logger.Error(err, "send tx error", "blockHash", blockHash)
					return blockHash, err
				}
				types.EventRecordsRaw(*blockData).DecodeEventRecords(c.metadata, &events)
				for _, evt := range events.System_ExtrinsicFailed {
					logger.Info("failed", "evt", evt)
				}
				// logger.Info("", "events", events)
				if len(events.System_ExtrinsicFailed) > 0 {
					err := errors.New("system.ExtrinsicFailed")
					//logger.Error(err, "send tx error", "tx", txhash)
					return blockHash, err
				}
				if len(events.System_ExtrinsicSuccess) > 0 {
					//logger.V(1).Info("send tx success", "tx", txhash)
					logger.Info("transfer success", "blockHash", blockHash)
					return blockHash, nil
				}
				// } else if status.IsFinalized {
				// block, err := api.RPC.Chain.GetBlock(status.AsInBlock)
				// for _, t := range block.Block.Extrinsics {
				// 	if t.Method == call {

				// 	}
				// }
			}
		case <-timeout:
			err = errors.Errorf("timeout of %d seconds reached without getting finalized status for extrinsic", timeoutSecs)
			logger.Error(err, "")
			return "", err
		}
	}
}

func reconnectChainClient(rpcAddr string) (*gsrpc.SubstrateAPI, error) {
	return gsrpc.NewSubstrateAPI(rpcAddr)
}

func healthchek(a *gsrpc.SubstrateAPI) error {
	defer func() {
		recover()
	}()
	_, err := a.RPC.System.Health()
	return err
}
