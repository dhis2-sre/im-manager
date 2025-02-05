package user

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	t.Run("basic hashing", func(t *testing.T) {
		password := "mySecurePassword123"
		hash, err := hashPassword(password)

		require.NoError(t, err)
		require.NotEmpty(t, hash)
		require.Contains(t, hash, "$argon2id$")
	})

	t.Run("hash format and components", func(t *testing.T) {
		password := "testPassword"
		hash, err := hashPassword(password)

		require.NoError(t, err)
		parts := strings.Split(hash, "$")
		require.Len(t, parts, 6)
		require.Equal(t, "argon2id", parts[1])
		require.Contains(t, parts[3], "m=131072")
		require.Contains(t, parts[3], "t=3")
		require.Contains(t, parts[3], "p=4")
	})

	t.Run("hash uniqueness", func(t *testing.T) {
		password := "samePassword"

		hash1, err := hashPassword(password)
		require.NoError(t, err)

		hash2, err := hashPassword(password)
		require.NoError(t, err)

		require.NotEqual(t, hash1, hash2)
	})

	t.Run("verification with comparePasswords", func(t *testing.T) {
		password := "verifyThisPassword"

		hash, err := hashPassword(password)
		require.NoError(t, err)

		match, err := comparePasswords(hash, password)
		require.NoError(t, err)
		require.True(t, match)
	})

	t.Run("empty password", func(t *testing.T) {
		hash, err := hashPassword("")

		require.NoError(t, err)
		require.NotEmpty(t, hash)
	})
}

func TestComparePasswords(t *testing.T) {
	t.Run("successful match", func(t *testing.T) {
		password := "correctPassword123"
		hash, _ := hashPassword(password)

		match, err := comparePasswords(hash, password)

		require.NoError(t, err)
		require.True(t, match)
	})

	t.Run("incorrect password", func(t *testing.T) {
		password := "correctPassword123"
		wrongPassword := "wrongPassword123"
		hash, _ := hashPassword(password)

		match, err := comparePasswords(hash, wrongPassword)

		require.NoError(t, err)
		require.False(t, match)
	})

	t.Run("invalid hash format", func(t *testing.T) {
		invalidHash := "invalidHash"

		match, err := comparePasswords(invalidHash, "anyPassword")

		require.Error(t, err)
		require.False(t, match)
		require.Contains(t, err.Error(), "invalid password hash")
	})

	t.Run("invalid parameters format", func(t *testing.T) {
		invalidHash := "$argon2id$v=19$invalid_params$salt$hash"

		match, err := comparePasswords(invalidHash, "anyPassword")

		require.Error(t, err)
		require.False(t, match)
		require.Contains(t, err.Error(), "invalid password parameters")
	})

	t.Run("invalid base64 salt", func(t *testing.T) {
		invalidHash := "$argon2id$v=19$m=128,t=3,p=4$invalid!!salt$hash"

		match, err := comparePasswords(invalidHash, "anyPassword")

		require.Error(t, err)
		require.False(t, match)
		require.Contains(t, err.Error(), "failed to decode salt")
	})
}
