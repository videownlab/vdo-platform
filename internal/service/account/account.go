package account

import (
	"encoding/hex"
	"time"

	"gorm.io/gorm"

	"vdo-platform/internal/service/account/entity"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vedhavyas/go-subkey/v2"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
)

var logger logr.Logger

type AccountService struct {
	gorm    *gorm.DB
	chainId uint16
}

func NewService(gorm *gorm.DB, chainId uint16, lg logr.Logger) *AccountService {
	logger = lg
	as := &AccountService{
		gorm,
		chainId,
	}
	return as
}

func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(
		&entity.Account{},
	)
}

func (t *AccountService) FetchByWalletAddress(walletAddress string) (*entity.Account, error) {
	acc := entity.Account{WalletAddress: walletAddress}
	if err := t.gorm.Take(&acc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &acc, nil
}

func (t *AccountService) FetchByEmail(email string) (*entity.Account, error) {
	acc := entity.Account{}
	err := t.gorm.Where("email=?", email).Take(&acc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &acc, nil
}

func (t *AccountService) CreateByEmail(email string) (*entity.Account, error) {
	kr, err := sr25519.Scheme{}.Generate()
	if err != nil {
		return nil, err
	}
	seed := hex.EncodeToString(kr.Seed())
	acc := &entity.Account{
		WalletAddress: kr.SS58Address(t.chainId),
		Email:         &email,
		Kind:          entity.AK_EMAIL_GEN,
		PublicKey:     hex.EncodeToString(kr.Public()),
		Seed:          &seed,
		CreatedAt:     time.Now(),
	}
	err = t.gorm.Create(acc).Error
	return acc, err
}

func (t *AccountService) CreateByPrivateDotWallet(walletAddress string) (*entity.Account, error) {
	network, pubkey, err := subkey.SS58Decode(walletAddress)
	if err != nil {
		return nil, err
	}
	if network != t.chainId {
		walletAddress = subkey.SS58Encode(pubkey, t.chainId)
	}
	acc := &entity.Account{
		WalletAddress: walletAddress,
		Kind:          entity.AK_PRIVATE_DOT,
		PublicKey:     hex.EncodeToString(pubkey),
		CreatedAt:     time.Now(),
	}
	err = t.gorm.Create(acc).Error
	return acc, err
}

func (t *AccountService) CreateByPrivateEthWallet(dotWalletAddress, ethWalletAddress string) (*entity.Account, error) {
	network, pubkey, err := subkey.SS58Decode(dotWalletAddress)
	if err != nil {
		return nil, err
	}
	if network != t.chainId {
		dotWalletAddress = subkey.SS58Encode(pubkey, t.chainId)
	}
	acc := &entity.Account{
		WalletAddress: dotWalletAddress,
		Kind:          entity.AK_PRIVATE_ETH,
		PublicKey:     hex.EncodeToString(pubkey),
		EthAddress:    &ethWalletAddress,
		CreatedAt:     time.Now(),
	}
	err = t.gorm.Create(acc).Error
	return acc, err
}
