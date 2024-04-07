package auth

import (
	"math/big"
	"time"
	"vdo-platform/internal/app/ctx"
	"vdo-platform/internal/dto"
	"vdo-platform/internal/service/account"
	ae "vdo-platform/internal/service/account/entity"
	"vdo-platform/pkg/setting"
	"vdo-platform/pkg/utils"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type AuthCode struct {
	Code      string
	Timestamp time.Time
}

type LoginResult struct {
	WalletAddress string         `json:"walletAddress"`
	AccountKind   ae.AccountKind `json:"accountKind"`
	Token         string         `json:"token"`
	ExpiresAt     int64          `json:"expiresAt"`
}

var jwtHelper *JwtHelper
var smtpSetting *setting.SmtpSettingS
var accountService *account.AccountService
var authCodeMap map[string]AuthCode
var outputAuthCode bool
var logger logr.Logger

func init() {
	authCodeMap = make(map[string]AuthCode)
}

func Setup(settings *setting.Settings, as *account.AccountService, lg logr.Logger) {
	smtpSetting = settings.SmtpSetting
	outputAuthCode = settings.AppSetting.OutputAuthCode
	accountService = as
	jwtHelper = &JwtHelper{
		[]byte(settings.AppSetting.JwtSecret),
		time.Duration(settings.AppSetting.JwtDuration * int(time.Second)),
	}
	logger = lg
}

func ParseJwtToken(tokenString string) (*CustomClaims, error) {
	return jwtHelper.ParseToken(tokenString)
}

func ApplyAuthCode(req dto.EmailAuthCodeReq) error {
	authCode := utils.RandNumeric(6)
	authCodeMap[req.Email] = AuthCode{Code: authCode, Timestamp: time.Now()}
	if outputAuthCode {
		logger.Info("apply auth-code", "authCode", authCode)
		return nil
	}
	return sendAuthCodeEmail(smtpSetting, authCode, req.Email)
}

func LoginByEmail(req dto.EmailLoginReq) (*LoginResult, error) {
	logger.V(1).Info("", "req", req)
	if req.AuthCode != "666888" { //FIXME: MUST REMOVE THIS IN PRODUCTION ENV!
		if ac, ok := authCodeMap[req.Email]; ok {
			if ac.Code != req.AuthCode {
				return nil, errors.New("incorrect auth-code")
			}
			expiredTime := time.Now().Add(time.Minute * 15)
			if ac.Timestamp.After(expiredTime) {
				delete(authCodeMap, req.Email)
				return nil, errors.New("auth-code is expired")
			}
		} else {
			return nil, errors.New("require a auth-code by email first")
		}
	}

	account, err := accountService.FetchByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if account == nil {
		logger.Info("create account for email", "email", req.Email)
		account, err = accountService.CreateByEmail(req.Email)
		if err != nil {
			return nil, err
		}
		logger.Info("email account wallet address", "walletAddress", account.WalletAddress)
		go func() {
			amount := big.NewInt(10000)
			logger.Info("give money to wallet", "walletAddress", account.WalletAddress, "amount", amount, "email", account.Email)
			ctx.ChainClient.TransferBySs58Address(account.WalletAddress, amount)
		}()
	}
	return generateLoginResult(account)
}

func LoginByDotWallet(req dto.DotWalletLoginReq) (*LoginResult, error) {
	if err := account.VerifyDotWalletSign(req.Address, req.Timestamp, req.Sign); err != nil {
		return nil, err
	}
	account, err := accountService.FetchByWalletAddress(req.Address)
	if err != nil {
		return nil, err
	}
	if account == nil {
		logger.Info("create account for wallet", "walletAddress", req.Address)
		account, err = accountService.CreateByPrivateDotWallet(req.Address)
		if err != nil {
			return nil, err
		}
	}
	return generateLoginResult(account)
}

func LoginByEthWallet(req dto.EthWalletLoginReq) (*LoginResult, error) {
	_, err := account.VerifyEthWalletSign(req.EthAddress, req.DotAddress, req.Timestamp, req.Sign)
	if err != nil {
		return nil, err
	}
	account, err := accountService.FetchByWalletAddress(req.DotAddress)
	if err != nil {
		return nil, err
	}
	if account == nil {
		logger.Info("create account for eth wallet", "dotWalletAddress", req.DotAddress, "ethWalletAddress", req.EthAddress)
		account, err = accountService.CreateByPrivateEthWallet(req.DotAddress, req.EthAddress)
		if err != nil {
			return nil, err
		}
		go func() {
			amount := big.NewInt(10000)
			logger.Info("give money to wallet", "dotWalletAddress", account.WalletAddress, "ethWalletAddress", req.EthAddress, "amount", amount)
			ctx.ChainClient.TransferBySs58Address(account.WalletAddress, amount)
		}()
	}
	return generateLoginResult(account)
}

func generateLoginResult(account *ae.Account) (*LoginResult, error) {
	token, expiredTime, err := jwtHelper.GenerateToken(*account)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		WalletAddress: account.WalletAddress,
		AccountKind:   account.Kind,
		Token:         token,
		ExpiresAt:     expiredTime.Unix(),
	}, nil
}
