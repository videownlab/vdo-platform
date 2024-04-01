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
	"encoding/json"
	"math/big"
	"time"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/retriever"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/state"
	"github.com/centrifuge/go-substrate-rpc-client/v4/rpc/author"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2"
	"golang.org/x/crypto/blake2b"
)

type ChainClient struct {
	api             *gsrpc.SubstrateAPI
	metadata        *types.Metadata
	runtimeVersion  *types.RuntimeVersion
	genesisHash     types.Hash
	keyring         signature.KeyringPair
	rpcAddr         string
	timeForBlockOut time.Duration
	networkId       uint16
	tokenDecimals   uint32
	retriver        retriever.EventRetriever
}

func NewChainClient(rpcAddr, secret string, networkId uint16) (*ChainClient, error) {
	api, err := gsrpc.NewSubstrateAPI(rpcAddr)
	if err != nil {
		return nil, err
	}
	retv, err := retriever.NewDefaultEventRetriever(state.NewEventProvider(api.RPC.State), api.RPC.State)
	if err != nil {
		return nil, err
	}
	metadata, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}
	props, err := api.RPC.System.Properties()
	if err != nil {
		return nil, err
	}
	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return nil, err
	}
	runtimeVersion, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return nil, err
	}
	keyring, err := signature.KeyringPairFromSecret(secret, 0)
	if err != nil {
		return nil, err
	}
	//FIXME: api.RPC.System.Properties got empty value
	tokenDecimals := uint32(props.AsTokenDecimals)
	if tokenDecimals <= 0 {
		tokenDecimals = 18
	}
	cli := ChainClient{
		api:             api,
		metadata:        metadata,
		genesisHash:     genesisHash,
		runtimeVersion:  runtimeVersion,
		keyring:         keyring,
		retriver:        retv,
		rpcAddr:         rpcAddr,
		networkId:       networkId,
		tokenDecimals:   tokenDecimals,
		timeForBlockOut: 15 * time.Second,
	}
	return &cli, nil
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

func (c *ChainClient) NewAccountId(pubkey []byte) (*types.AccountID, error) {
	return types.NewAccountID(pubkey)
}

type multiTargetAddr struct {
	ss58addr string
	pubkey   []byte
}

func (c *ChainClient) TransferBySs58Address(target string, amount *big.Int) (*types.Hash, *types.Hash, error) {
	_, pubkey, err := subkey.SS58Decode(target)
	if err != nil {
		return nil, nil, err
	}
	ta := multiTargetAddr{ss58addr: target, pubkey: pubkey}
	return c.doTransfer(ta, amount)
}

func (c *ChainClient) Transfer(target types.AccountID, amount *big.Int) (*types.Hash, *types.Hash, error) {
	ss58addr := subkey.SS58Encode(target[:], c.networkId)
	ta := multiTargetAddr{ss58addr: ss58addr, pubkey: target[:]}
	return c.doTransfer(ta, amount)
}

func (c *ChainClient) doTransfer(target multiTargetAddr, amount *big.Int) (*types.Hash, *types.Hash, error) {
	logger := logger.WithName("transfer").WithValues("target", target.ss58addr, "amount", amount)
	// 1 unit of transfer
	if c.tokenDecimals > 0 {
		var i, e = big.NewInt(10), big.NewInt(int64(c.tokenDecimals))
		amount = amount.Mul(amount, i.Exp(i, e, nil))
		logger = logger.WithValues("amount", amount)
	}
	logger.V(1).Info("begin balance transfer")
	targetAddr, err := types.NewMultiAddressFromAccountID(target.pubkey)
	if err != nil {
		return nil, nil, err
	}
	call, err := types.NewCall(c.metadata, "Balances.transfer", targetAddr, types.NewUCompact(amount))
	if err != nil {
		return nil, nil, err
	}
	submiter := LoggerContexedExtrinsicSubmiter{logger, c}
	blockHash, txHash, err := submiter.submitAndWatchExtrinsicUtilSuccess(call)
	if err != nil {
		submiter.logger.Error(err, "balance transfer failed")
	} else {
		submiter.logger.Info("balance transfer success")
	}
	return blockHash, txHash, err
}

type LoggerContexedExtrinsicSubmiter struct {
	logger logr.Logger
	cc     *ChainClient
}

func (t *LoggerContexedExtrinsicSubmiter) submitAndWatchExtrinsicUtilSuccess(call types.Call) (*types.Hash, *types.Hash, error) {
	logger := t.logger
	defer func() {
		t.logger = logger
	}()
	c := t.cc
	api := c.api
	accountKey, err := types.CreateStorageKey(c.metadata, "System", "Account", c.keyring.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(accountKey, &accountInfo)
	if err != nil {
		return nil, nil, err
	}
	var nonce uint32
	if !ok {
		nonce = 1
	} else {
		nonce = uint32(accountInfo.Nonce)
	}

	var sub *author.ExtrinsicStatusSubscription
	var txHash *types.Hash
	for i := 0; i < 3; i++ {
		options := types.SignatureOptions{
			BlockHash:          c.genesisHash,
			GenesisHash:        c.genesisHash,
			Era:                types.ExtrinsicEra{IsMortalEra: false},
			Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
			Tip:                types.NewUCompactFromUInt(0),
			SpecVersion:        c.runtimeVersion.SpecVersion,
			TransactionVersion: c.runtimeVersion.TransactionVersion,
		}

		ext := types.NewExtrinsic(call)
		err = ext.Sign(c.keyring, options)
		if err != nil {
			return nil, nil, errors.Wrap(err, "sign a call error")
		}
		txHash, err = figureExtrinsicHash(&ext)
		if err != nil {
			return nil, nil, err
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
	logger = logger.WithValues("txHash", txHash)
	const timeoutSecs = 18
	timeout := time.After(timeoutSecs * time.Second)
	for {
		select {
		case status := <-sub.Chan():
			if status.IsInBlock {
				blockHash := status.AsInBlock
				logger = logger.WithValues("blockHash", blockHash)
				if err != nil {
					logger.Error(err, "SubmitAndWatchExtrinsic() error")
					return &blockHash, txHash, err
				}
				evts, err := c.retriver.GetEvents(blockHash)
				if err != nil {
					return &blockHash, txHash, err
				}
				txIndex, err := c.retrieve_extrinsic_index_from_block(blockHash, *txHash)
				if err != nil {
					return &blockHash, txHash, err
				}
				for _, evt := range evts {
					if evt.Phase.IsApplyExtrinsic && evt.Phase.AsApplyExtrinsic == uint32(txIndex) {
						if evt.Name == "System.ExtrinsicFailed" {
							for _, f := range evt.Fields {
								if f.Name == "sp_runtime.DispatchError.dispatch_error" {
									ss, err := json.Marshal(f.Value)
									if err != nil {
										logger.Error(err, "json marshal dispatch error value error")
									}
									err = errors.Errorf("extrinsic failed on chain: %s, blockHask: %s, txHash: %s", ss, blockHash.Hex(), txHash.Hex())
									return &blockHash, txHash, err
								}
							}
						}
					}
				}
				return &blockHash, txHash, nil
			}
		case <-timeout:
			err = errors.Errorf("timeout of %d seconds reached without getting finalized status for extrinsic", timeoutSecs)
			logger.Error(err, "")
			return nil, txHash, err
		}
	}
}

func figureExtrinsicHash(ext *types.Extrinsic) (*types.Hash, error) {
	b, err := codec.Encode(ext)
	if err != nil {
		return nil, errors.Wrap(err, "scale encode extrinsic error")
	}
	a := blake2b.Sum256(b)
	h := types.NewHash(a[:])
	return &h, nil
}

func (t *ChainClient) retrieve_extrinsic_index_from_block(block_hash, tx_hash types.Hash) (uint32, error) {
	block, err := t.api.RPC.Chain.GetBlock(block_hash)
	if err != nil {
		return 0, err
	}
	for i, ext := range block.Block.Extrinsics {
		b, err := codec.Encode(ext)
		if err != nil {
			return 0, err
		}
		a := blake2b.Sum256(b)
		h := types.NewHash(a[:])
		if tx_hash == h {
			return uint32(i), nil
		}
	}
	return 0, errors.New("ExtrinsicNotFound")
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
