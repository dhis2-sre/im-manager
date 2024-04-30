package token

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/config"

	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token/helper"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(
	tokenRepository repository,
	privateKey *rsa.PrivateKey,
	publicKey *rsa.PublicKey,
	authentication config.Authentication,
) (*tokenService, error) {
	return &tokenService{
		repository:     tokenRepository,
		privateKey:     privateKey,
		publicKey:      publicKey,
		authentication: authentication,
	}, nil
}

type repository interface {
	SetRefreshToken(userId uint, tokenId string, expiresIn time.Duration) error
	DeleteRefreshToken(userId uint, previousTokenId string) error
	DeleteRefreshTokens(userId uint) error
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
	repository     repository
	privateKey     *rsa.PrivateKey
	publicKey      *rsa.PublicKey
	authentication config.Authentication
}

func (t tokenService) GetTokens(user *model.User, previousRefreshTokenId string, rememberMe bool) (*Tokens, error) {
	if previousRefreshTokenId != "" {
		if err := t.repository.DeleteRefreshToken(user.ID, previousRefreshTokenId); err != nil {
			return nil, errdef.NewUnauthorized("could not delete previous refreshToken for user.Id: %d, tokenId: %s", user.ID, previousRefreshTokenId)
		}
	}

	accessToken, err := helper.GenerateAccessToken(user, t.privateKey, t.authentication.AccessTokenExpirationSeconds)
	if err != nil {
		return nil, fmt.Errorf("error generating accessToken for user: %+v\nError: %s", user, err)
	}

	expiration := t.authentication.RefreshTokenExpirationSeconds
	if rememberMe {
		expiration = t.authentication.RefreshTokenRememberMeExpirationSeconds
	}

	refreshToken, err := helper.GenerateRefreshToken(user, t.authentication.RefreshTokenSecretKey, expiration)
	if err != nil {
		return nil, fmt.Errorf("error generating refreshToken for user: %+v\nError: %s", user, err)
	}

	if err := t.repository.SetRefreshToken(user.ID, refreshToken.TokenId, refreshToken.ExpiresIn); err != nil {
		return nil, fmt.Errorf("error storing token: %d\nError: %s", user.ID, err)
	}

	return &Tokens{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		RefreshToken: refreshToken.SignedString,
		ExpiresIn:    uint(t.authentication.AccessTokenExpirationSeconds),
	}, nil
}

func (t tokenService) ValidateRefreshToken(tokenString string) (*RefreshTokenData, error) {
	claims, err := helper.ValidateRefreshToken(tokenString, t.authentication.RefreshTokenSecretKey)
	if err != nil {
		log.Printf("Unable to validate token: %s\n%s\n", tokenString, err)
		return nil, errors.New("unable to verify refresh token")
	}

	tokenId, err := uuid.Parse(claims.ID)
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
	return t.repository.DeleteRefreshTokens(userId)
}
