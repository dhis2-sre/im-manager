package token

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token/helper"
	"github.com/gofrs/uuid"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(c config.Config, tokenRepository repository) (*tokenService, error) {
	privateKey, err := c.Authentication.Keys.GetPrivateKey()
	if err != nil {
		return nil, err
	}

	publicKey, err := c.Authentication.Keys.GetPublicKey()
	if err != nil {
		return nil, err
	}

	return &tokenService{
		tokenRepository,
		privateKey,
		publicKey,
		c.Authentication.AccessTokenExpirationSeconds,
		c.Authentication.RefreshTokenSecretKey,
		c.Authentication.RefreshTokenExpirationSeconds,
	}, nil
}

type repository interface {
	setRefreshToken(userId uint, tokenId string, expiresIn time.Duration) error
	deleteRefreshToken(userId uint, previousTokenId string) error
	deleteRefreshTokens(userId uint) error
}

// Tokens domain object defining user tokens
// swagger:model
type Tokens struct {
	AccessToken  string `json:"accessToken"`
	TokenType    string `json:"tokenType"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    uint   `json:"expiresIn"`
}

type RefreshTokenData struct {
	SignedToken string
	ID          uuid.UUID
	UserId      uint
}

type tokenService struct {
	repository                    repository
	privateKey                    *rsa.PrivateKey
	publicKey                     *rsa.PublicKey
	accessTokenExpirationSeconds  int
	refreshTokenSecretKey         string
	refreshTokenExpirationSeconds int
}

func (t tokenService) GetTokens(user *model.User, previousRefreshTokenId string) (*Tokens, error) {
	if previousRefreshTokenId != "" {
		if err := t.repository.deleteRefreshToken(user.ID, previousRefreshTokenId); err != nil {
			return nil, fmt.Errorf("could not delete previous refreshToken for user.Id: %d, tokenId: %s", user.ID, previousRefreshTokenId)
		}
	}

	accessToken, err := helper.GenerateAccessToken(user, t.privateKey, t.accessTokenExpirationSeconds)
	if err != nil {
		return nil, fmt.Errorf("error generating accessToken for user: %+v\nError: %s", user, err)
	}

	refreshToken, err := helper.GenerateRefreshToken(user, t.refreshTokenSecretKey, t.refreshTokenExpirationSeconds)
	if err != nil {
		return nil, fmt.Errorf("error generating refreshToken for user: %+v\nError: %s", user, err)
	}

	if err := t.repository.setRefreshToken(user.ID, refreshToken.TokenId.String(), refreshToken.ExpiresIn); err != nil {
		return nil, fmt.Errorf("error storing token: %d\nError: %s", user.ID, err)
	}

	return &Tokens{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		RefreshToken: refreshToken.SignedString,
		ExpiresIn:    uint(t.accessTokenExpirationSeconds),
	}, nil
}

func (t tokenService) ValidateAccessToken(tokenString string) (*model.User, error) {
	tokenClaims, err := helper.ValidateAccessToken(tokenString, t.publicKey)
	if err != nil {
		log.Printf("Unable to verify token: %s\n", err)
		return nil, errors.New("unable to verify token")
	}

	return tokenClaims.User, nil
}

func (t tokenService) ValidateRefreshToken(tokenString string) (*RefreshTokenData, error) {
	claims, err := helper.ValidateRefreshToken(tokenString, t.refreshTokenSecretKey)
	if err != nil {
		log.Printf("Unable to validate token: %s\n%s\n", tokenString, err)
		return nil, errors.New("unable to verify refresh token")
	}

	tokenId, err := uuid.FromString(claims.ID)
	if err != nil {
		log.Printf("Couldn't parse token id: %s\n%s\n", claims.ID, err)
		return nil, errors.New("unable to verify refresh token")
	}

	return &RefreshTokenData{
		SignedToken: tokenString,
		ID:          tokenId,
		UserId:      claims.UserId,
	}, nil
}

func (t tokenService) SignOut(userId uint) error {
	return t.repository.deleteRefreshTokens(userId)
}
