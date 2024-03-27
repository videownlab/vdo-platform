package account

import (
	"encoding/json"
	"errors"

	"vdo-platform/internal/app/ctx"
	"vdo-platform/internal/dto"
	"vdo-platform/internal/service/account/entity"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
)

func (t *AccountService) SignTx(req *dto.SignTxReq) (string, error) {
	logger.Info("sign tx request", "req", req)
	acc, err := t.FetchByWalletAddress(req.WalletAddress)
	if err != nil {
		return "", err
	}
	if acc == nil {
		return "", errors.New("the wallet account not exist")
	}
	if acc.Kind != entity.AK_EMAIL_GEN {
		return "", errors.New("only sign tx for email wallet")
	}
	if acc.Seed == nil || len(*acc.Seed) == 0 {
		return "", errors.New("the seed of the email wallet has been lost")
	}

	var ext types.Extrinsic
	if ext.UnmarshalJSON([]byte(req.Extrinsic)) != nil {
		bytes, _ := json.Marshal(req.Extrinsic)
		if err := ext.UnmarshalJSON(bytes); err != nil {
			logger.Error(err, "")
			return "", err
		}
	}
	logger.Info("", "extrinsic", ext)
	keyring, err := signature.KeyringPairFromSecret(*acc.Seed, 11330)
	if err != nil {
		return "", err
	}
	nonce, err := ctx.ChainClient.GetAccountNonce(keyring.PublicKey)
	if err != nil {
		return "", err
	}
	// Sign the transaction
	err = ext.Sign(keyring, ctx.ChainClient.MakeSignatureOptions(nonce))
	if err != nil {
		return "", err
	}

	data, err := codec.EncodeToHex(ext)
	if err != nil {
		return "", err
	}
	return data, nil
}
