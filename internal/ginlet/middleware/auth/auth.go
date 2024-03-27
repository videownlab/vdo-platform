package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"vdo-platform/internal/ginlet/resp"
	"vdo-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

const BEARER_PREFIX = "Bearer "
const ctxKeyLoginInfo = "loginInfo"

type LoginInfo struct {
	Claims    *auth.CustomClaims
	ClientIp  string
	LoginTime time.Time
}

func jwtBearerRequired(c *gin.Context) (bool, *auth.CustomClaims) {
	bearer := c.Request.Header.Get("Authorization")
	if bearer == "" {
		resp.ErrorWithHttpStatus(c, errors.New("authorization required"), http.StatusUnauthorized)
		c.Abort()
		return false, nil
	}
	jwtStr := strings.TrimPrefix(bearer, BEARER_PREFIX)
	// decode jwt => claims
	claims, err := auth.ParseJwtToken(jwtStr)
	if err != nil {
		resp.ErrorWithHttpStatus(c, err, http.StatusUnauthorized)
		c.Abort()
		return false, nil
	}
	return true, claims
}

func AuthRequired(c *gin.Context) {
	if ok, claims := jwtBearerRequired(c); ok && claims != nil {
		storeClaims(c, claims)
	}
}

func storeClaims(c *gin.Context, claims *auth.CustomClaims) {
	loginInfo := LoginInfo{
		Claims:    claims,
		ClientIp:  c.ClientIP(),
		LoginTime: time.Now(),
	}
	c.Set(ctxKeyLoginInfo, &loginInfo)
}

func GetLoginInfo(c *gin.Context) *LoginInfo {
	if value, ok := c.Get(ctxKeyLoginInfo); ok {
		if loginInfo, ok := value.(*LoginInfo); ok {
			return loginInfo
		}
	}
	return nil
}
