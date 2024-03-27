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
	"sync"
	"sync/atomic"
	"time"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/rpc/author"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2"
)

type ChainClient struct {
	lock            *sync.Mutex
	api             *gsrpc.SubstrateAPI
	chainState      *atomic.Bool
	metadata        *types.Metadata
	runtimeVersion  *types.RuntimeVersion
	keyEvents       types.StorageKey
	genesisHash     types.Hash
	keyring         signature.KeyringPair
	rpcAddr         string
	timeForBlockOut time.Duration
}

func NewChainClient(rpcAddr, secret string, t time.Duration) (*ChainClient, error) {
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
	cli.keyEvents, err = types.CreateStorageKey(
		cli.metadata,
		pallet_System,
		events,
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
	cli.timeForBlockOut = t
	cli.rpcAddr = rpcAddr
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

func (c *ChainClient) TransferBySs58Address(target string, amount uint64) error {
	_, pubkey, err := subkey.SS58Decode(target)
	if err != nil {
		return err
	}
	acc, err := types.NewAccountID(pubkey)
	if err != nil {
		return err
	}
	return c.Transfer(*acc, amount)
}

func (c *ChainClient) Transfer(target types.AccountID, amount uint64) error {
	logger := logger.WithName("transfer")
	api := c.api
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return err
	}

	call, err := types.NewCall(meta, "Balances.transfer", target, types.NewUCompactFromUInt(amount))
	if err != nil {
		return err
	}

	ext := types.NewExtrinsic(call)

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return err
	}

	key, err := types.CreateStorageKey(meta, "System", "Account", c.keyring.PublicKey)
	if err != nil {
		return err
	}

	era := types.ExtrinsicEra{IsMortalEra: false}
	var sub *author.ExtrinsicStatusSubscription
	for {
		var accountInfo types.AccountInfo
		ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("get account info not ok")
		}

		nonce := uint32(accountInfo.Nonce)
		o := types.SignatureOptions{
			// BlockHash:   blockHash,
			BlockHash:          c.genesisHash, // BlockHash needs to == GenesisHash if era is immortal. // TODO: add an error?
			Era:                era,
			GenesisHash:        c.genesisHash,
			Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
			SpecVersion:        rv.SpecVersion,
			Tip:                types.NewUCompactFromUInt(0),
			TransactionVersion: rv.TransactionVersion,
		}

		err = ext.Sign(c.keyring, o)
		if err != nil {
			return errors.Wrap(err, "sign a call error")
		}

		sub, err = api.RPC.Author.SubmitAndWatchExtrinsic(ext)
		if err != nil {
			nonce++
			logger.Error(err, "try again later", "nonce", nonce)
			time.Sleep(5 * time.Second)
			continue
		}

		break
	}

	defer sub.Unsubscribe()
	const timeoutSecs = 24
	timeout := time.After(timeoutSecs * time.Second)
	for {
		select {
		case status := <-sub.Chan():
			logger.V(1).Info("subscribe transfer", "status", status)
			if status.IsInBlock {
				logger.Info("transfer TX in block", "blockHash", status.AsInBlock.Hex())
				return nil
			}
		case <-timeout:
			err = errors.Errorf("timeout of %d seconds reached without getting finalized status for extrinsic", timeoutSecs)
			logger.Error(err, "")
			return err
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
