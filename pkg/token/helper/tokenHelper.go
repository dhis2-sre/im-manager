package helper

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func GenerateAccessToken(user *model.User, key *rsa.PrivateKey, expirationInSeconds int) (string, error) {
	unixTime := time.Now().Unix()
	tokenExpiration := unixTime + int64(expirationInSeconds)

	token := jwt.New()

	err := token.Set(jwt.IssuedAtKey, unixTime)
	if err != nil {
		return "", err
	}

	err = token.Set(jwt.ExpirationKey, tokenExpiration)
	if err != nil {
		return "", err
	}

	err = token.Set("user", user)
	if err != nil {
		return "", err
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

type refreshToken struct {
	SignedString string
	TokenId      string
	ExpiresIn    time.Duration
}

//goland:noinspection GoExportedFuncWithUnexportedType
func GenerateRefreshToken(user *model.User, secretKey string, expirationInSeconds int) (*refreshToken, error) {
	currentTime := time.Now()
	tokenExpiration := currentTime.Add(time.Duration(expirationInSeconds) * time.Second)

	token := jwt.New()

	err := token.Set("userId", user.ID)
	if err != nil {
		return nil, err
	}

	tokenId := uuid.NewString()
	err = token.Set(jwt.JwtIDKey, tokenId)
	if err != nil {
		return nil, err
	}

	err = token.Set(jwt.ExpirationKey, tokenExpiration.Unix())
	if err != nil {
		return nil, err
	}

	err = token.Set(jwt.IssuedAtKey, currentTime.Unix())
	if err != nil {
		return nil, err
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256, []byte(secretKey)))
	if err != nil {
		return nil, err
	}

	return &refreshToken{
		SignedString: string(signed),
		TokenId:      tokenId,
		ExpiresIn:    tokenExpiration.Sub(currentTime),
	}, nil
}

type refreshTokenClaims struct {
	UserId    uint          `json:"uid"`
	ID        string        `json:"jti"`
	ExpiresIn time.Duration `json:"exp"`
	IssuedAt  int64         `json:"iat"`
}

//goland:noinspection GoExportedFuncWithUnexportedType
func ValidateRefreshToken(tokenString string, secretKey string) (*refreshTokenClaims, error) {
	token, err := jwt.Parse(
		[]byte(tokenString),
		jwt.WithKey(jwa.HS256, []byte(secretKey)),
	)
	if err != nil {
		return nil, err
	}

	userId, ok := token.Get("userId")
	if !ok {
		return nil, errors.New("UserId not found in claims")
	}

	id, ok := token.Get(jwt.JwtIDKey)
	if !ok {
		return nil, fmt.Errorf("%s not found in claims", jwt.JwtIDKey)
	}

	tokenExpiration, ok := token.Get(jwt.ExpirationKey)
	if !ok {
		return nil, fmt.Errorf("%s not found in claims", jwt.ExpirationKey)
	}

	issuedAt, ok := token.Get(jwt.IssuedAtKey)
	if !ok {
		return nil, fmt.Errorf("%s not found in claims", jwt.IssuedAtKey)
	}

	return &refreshTokenClaims{
		UserId:    uint(userId.(float64)),
		ID:        fmt.Sprintf("%v", id),
		ExpiresIn: time.Until(tokenExpiration.(time.Time)),
		IssuedAt:  issuedAt.(time.Time).Unix(),
	}, nil
}
