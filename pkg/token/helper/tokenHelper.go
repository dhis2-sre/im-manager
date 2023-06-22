package helper

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gofrs/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
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

	signed, err := jwt.Sign(token, jwa.RS256, key)
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

type AccessTokenClaims struct {
	User      *model.User `json:"user"`
	IssuedAt  int64       `json:"iat"`
	ExpiresIn int64       `json:"exp"`
}

func ValidateAccessToken(tokenString string, key *rsa.PublicKey) (*AccessTokenClaims, error) {
	token, err := jwt.Parse(
		[]byte(tokenString),
		jwt.WithValidate(true),
		jwt.WithVerify(jwa.RS256, key),
	)
	if err != nil {
		return nil, err
	}

	userData, ok := token.Get("user")
	if !ok {
		return nil, errors.New("user not found in claims")
	}

	userMap, ok := userData.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to parse user data")
	}

	id := userMap["ID"].(float64)
	email := userMap["Email"].(string)

	user := &model.User{
		ID:    uint(id),
		Email: email,
	}

	return &AccessTokenClaims{
		user,
		token.IssuedAt().Unix(),
		token.Expiration().Unix(),
	}, nil
}

type refreshToken struct {
	SignedString string
	TokenId      uuid.UUID
	ExpiresIn    time.Duration
}

func GenerateRefreshToken(user *model.User, secretKey string, expirationInSeconds int) (*refreshToken, error) {
	currentTime := time.Now()
	tokenExpiration := currentTime.Add(time.Duration(expirationInSeconds) * time.Second)

	tokenId, err := uuid.NewV4()
	if err != nil {
		log.Println("Failed to generate refresh token id")
		return nil, err
	}

	token := jwt.New()

	err = token.Set("userId", user.ID)
	if err != nil {
		return nil, err
	}

	err = token.Set(jwt.JwtIDKey, tokenId.String())
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

	signed, err := jwt.Sign(token, jwa.HS256, []byte(secretKey))
	if err != nil {
		log.Printf("Failed to sign token: %s", err)
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

func ValidateRefreshToken(tokenString string, secretKey string) (*refreshTokenClaims, error) {
	token, err := jwt.Parse(
		[]byte(tokenString),
		jwt.WithValidate(true),
		jwt.WithVerify(jwa.HS256, []byte(secretKey)),
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
