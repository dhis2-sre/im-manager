package helper

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAccessToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	user := &model.User{
		Email:    "email",
		Password: "pass",
	}

	_, err = GenerateAccessToken(user, key, 12)
	assert.NoError(t, err)
	// TODO
	//	assert.WithinDuration(t, , , 5*time.Second)

	// TODO: Assert... Something
	//	println(token)
	//	println(expectedToken)
	//	assert.Equal(t, expectedToken, token)
}

func TestValidateAccessToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	user := &model.User{
		Email:    "email",
		Password: "pass",
	}

	token, err := GenerateAccessToken(user, privateKey, 12)
	assert.NoError(t, err)

	claims, err := ValidateAccessToken(token, &privateKey.PublicKey)
	assert.NoError(t, err)

	// TODO: Assert more
	assert.Equal(t, "email", claims.User.Email)
}

func TestGenerateRefreshToken(t *testing.T) {
	user := &model.User{}
	user.ID = 1

	secretKey := "secret"
	expiration := 12
	signedStringPrefix := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9."

	tokenData, err := GenerateRefreshToken(user, secretKey, expiration)
	assert.NoError(t, err)

	assert.Equal(t, expiration, int(tokenData.ExpiresIn.Seconds()))
	assert.True(t, strings.HasPrefix(tokenData.SignedString, signedStringPrefix))
	// TODO: Assert more
}

func TestValidateRefreshToken(t *testing.T) {
	user := &model.User{}
	user.ID = 1

	secretKey := "secret"

	expiration := 12

	tokenData, err := GenerateRefreshToken(user, secretKey, expiration)
	assert.NoError(t, err)

	refreshTokenData, err := ValidateRefreshToken(tokenData.SignedString, secretKey)
	assert.NoError(t, err)

	assert.Equal(t, user.ID, refreshTokenData.UserId)
	assert.WithinDuration(t, time.Unix(int64(expiration), 0), time.Unix(int64(refreshTokenData.ExpiresIn.Seconds()), 0), 1*time.Second)
	assert.WithinDuration(t, time.Now().Add(time.Duration(expiration)), time.Unix(refreshTokenData.IssuedAt, 0), 1*time.Second)
}
