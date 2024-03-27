package auth

import (
	"errors"
	"time"

	accountEntity "vdo-platform/internal/service/account/entity"

	"github.com/golang-jwt/jwt/v4"
)

var (
	ErrTokenExpired     error = errors.New("token is expired")
	ErrTokenNotValidYet error = errors.New("token not active yet")
	ErrTokenMalformed   error = errors.New("that's not even a token")
	ErrTokenInvalid     error = errors.New("couldn't handle this token")
)

type JwtHelper struct {
	JwtKey        []byte
	ValidDuration time.Duration
}

type CustomClaims struct {
	WalletAddress string                    `json:"wa,omitempty"`
	AccountKind   accountEntity.AccountKind `json:"ak,omitempty"`
	jwt.RegisteredClaims
}

func (j *JwtHelper) GenerateToken(account accountEntity.Account) (string, time.Time, error) {
	now := time.Now()
	expired := now.Add(j.ValidDuration)
	claims := CustomClaims{
		account.WalletAddress,
		account.Kind,
		jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(now.Add(time.Second * -5)),
			ExpiresAt: jwt.NewNumericDate(expired),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(j.JwtKey)
	return tokenStr, expired, err
}

func (j *JwtHelper) GenerateTokenByClaims(claims CustomClaims) (string, time.Time, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(j.JwtKey)
	return tokenStr, claims.ExpiresAt.Time, err
}

func (j *JwtHelper) ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.JwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrTokenInvalid
}

func (j *JwtHelper) RefreshToken(tokenString string) (string, *time.Time, error) {
	jwt.TimeFunc = func() time.Time {
		return time.Unix(0, 0)
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.JwtKey, nil
	})
	if err != nil {
		return "", nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		jwt.TimeFunc = time.Now
		claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
		tokenStr, time, err := j.GenerateTokenByClaims(*claims)
		return tokenStr, &time, err
	}
	return "", nil, ErrTokenInvalid
}
