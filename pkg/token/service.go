package token

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token/helper"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(
	logger *slog.Logger,
	tokenRepository repository,
	privateKey *rsa.PrivateKey,
	accessTokenExpirationSeconds int,
	refreshTokenSecretKey string,
	refreshTokenExpirationSeconds int,
	refreshTokenRememberMeExpirationSeconds int,
) (*tokenService, error) {
	return &tokenService{
		logger:                                  logger,
		repository:                              tokenRepository,
		privateKey:                              privateKey,
		accessTokenExpirationSeconds:            accessTokenExpirationSeconds,
		refreshTokenSecretKey:                   refreshTokenSecretKey,
		refreshTokenExpirationSeconds:           refreshTokenExpirationSeconds,
		refreshTokenRememberMeExpirationSeconds: refreshTokenRememberMeExpirationSeconds,
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
	logger                                  *slog.Logger
	repository                              repository
	privateKey                              *rsa.PrivateKey
	accessTokenExpirationSeconds            int
	refreshTokenSecretKey                   string
	refreshTokenExpirationSeconds           int
	refreshTokenRememberMeExpirationSeconds int
}

func (t tokenService) GetTokens(user *model.User, previousRefreshTokenId string, rememberMe bool) (*Tokens, error) {
	if previousRefreshTokenId != "" {
		if err := t.repository.DeleteRefreshToken(user.ID, previousRefreshTokenId); err != nil {
			return nil, errdef.NewUnauthorized("could not delete previous refreshToken for user.Id: %d, tokenId: %s", user.ID, previousRefreshTokenId)
		}
	}

	accessToken, err := helper.GenerateAccessToken(user, t.privateKey, t.accessTokenExpirationSeconds)
	if err != nil {
		return nil, fmt.Errorf("error generating accessToken for user: %+v\nError: %s", user, err)
	}

	expiration := t.refreshTokenExpirationSeconds
	if rememberMe {
		expiration = t.refreshTokenRememberMeExpirationSeconds
	}

	refreshToken, err := helper.GenerateRefreshToken(user, t.refreshTokenSecretKey, expiration)
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
		ExpiresIn:    uint(t.accessTokenExpirationSeconds),
	}, nil
}

func (t tokenService) ValidateRefreshToken(ctx context.Context, tokenString string) (*RefreshTokenData, error) {
	claims, err := helper.ValidateRefreshToken(tokenString, t.refreshTokenSecretKey)
	if err != nil {
		t.logger.ErrorContext(ctx, "Unable to validate token", "error", err, "token", tokenString)
		return nil, errors.New("unable to verify refresh token")
	}

	tokenId, err := uuid.Parse(claims.ID)
	if err != nil {
		t.logger.ErrorContext(ctx, "Couldn't parse token id", "error", err, "claimsId", claims.ID)
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
