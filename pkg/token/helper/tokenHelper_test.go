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

func TestRefreshAccessToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	user := &model.User{
		Email: "test@example.com",
	}

	// Test case: token with remaining time > 60 seconds, should return the same token
	longExpirationToken, err := GenerateAccessToken(user, key, 120)
	require.NoError(t, err)
	refreshed, err := RefreshAccessToken(longExpirationToken, key)
	assert.NoError(t, err)
	assert.Equal(t, longExpirationToken, refreshed)

	// Test case: token with remaining time <= 60 seconds, should generate a new token
	shortExpirationToken, err := GenerateAccessToken(user, key, 30)
	require.NoError(t, err)
	refreshed2, err := RefreshAccessToken(shortExpirationToken, key)
	assert.NoError(t, err)
	assert.NotEqual(t, shortExpirationToken, refreshed2)

	// Verify the new token has an expiration close to 60 seconds from now
	_, exp, err := ValidateAccessToken(refreshed2, &key.PublicKey)
	require.NoError(t, err)
	now := time.Now().Unix()
	assert.True(t, exp-now <= 60 && exp-now > 50, "new token expiration should be around 60 seconds")
}
