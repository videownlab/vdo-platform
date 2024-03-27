package api

import (
	"vdo-platform/internal/dto"
	"vdo-platform/internal/ginlet/resp"
	"vdo-platform/internal/service"
	"vdo-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

type AuthAPI struct{}

func NewAuthAPI() AuthAPI {
	return AuthAPI{}
}

func (t AuthAPI) ApplyAuthCode(c *gin.Context) {
	var f dto.EmailAuthCodeReq
	if err := c.ShouldBind(&f); err != nil {
		resp.Error(c, err)
		return
	}
	err := auth.ApplyAuthCode(f)
	if err != nil {
		resp.Error(c, err)
		return
	}
	resp.Ok(c, nil)
}

func (t AuthAPI) LoginByEmail(c *gin.Context) {
	var f dto.EmailLoginReq
	if err := c.ShouldBind(&f); err != nil {
		resp.Error(c, err)
		return
	}
	ar, err := auth.LoginByEmail(f)
	if err != nil {
		resp.Error(c, err)
		return
	}
	resp.Ok(c, ar)
}

func (t AuthAPI) LoginByWallet(c *gin.Context) {
	var f dto.WalletLoginReq
	if err := c.ShouldBind(&f); err != nil {
		resp.Error(c, err)
		return
	}
	ar, err := auth.LoginByWallet(f)
	if err != nil {
		resp.Error(c, err)
		return
	}
	resp.Ok(c, ar)
}

func (t AuthAPI) SignTx(c *gin.Context) {
	var f dto.SignTxReq
	if err := c.ShouldBind(&f); err != nil {
		resp.Error(c, err)
		return
	}
	data, err := service.AccountService.SignTx(&f)
	if err != nil {
		resp.Error(c, err)
		return
	}
	resp.Ok(c, data)
}
