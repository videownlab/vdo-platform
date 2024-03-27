package account

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"vdo-platform/internal/service/account/entity"
	"vdo-platform/pkg/utils"

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

func (t *AccountService) CreateByPrivateWallet(walletAddress string) (*entity.Account, error) {
	network, pubkey, err := subkey.SS58Decode(walletAddress)
	if err != nil {
		return nil, err
	}
	if network != t.chainId {
		walletAddress = subkey.SS58Encode(pubkey, t.chainId)
	}
	acc := &entity.Account{
		WalletAddress: walletAddress,
		Kind:          entity.AK_PRIVATE_OWN,
		PublicKey:     hex.EncodeToString(pubkey),
		CreatedAt:     time.Now(),
	}
	err = t.gorm.Create(acc).Error
	return acc, err
}

func VerifyWalletSign(walletAddress string, timestamp int64, sign string) error {
	signBytes, err := hex.DecodeString(sign)
	if err != nil {
		return err
	}
	if utils.Abs(time.Now().Unix()-timestamp) > 300 {
		return errors.New("invalid timestamp")
	}
	_, pubkeyBytes, err := subkey.SS58Decode(walletAddress)
	if err != nil {
		return err
	}
	pubkey, err := sr25519.Scheme{}.FromPublicKey(pubkeyBytes)
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("<Bytes>")
	sb.WriteString(walletAddress)
	sb.WriteString(strconv.FormatInt(timestamp, 10))
	sb.WriteString("</Bytes>")
	if !pubkey.Verify([]byte(sb.String()), signBytes) {
		return errors.New("invalid wallet sign")
	}
	return nil
}
