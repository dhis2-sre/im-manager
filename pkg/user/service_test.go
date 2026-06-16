package user

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/scrypt"
)

func TestHashPassword(t *testing.T) {
	t.Run("produces argon2id format", func(t *testing.T) {
		hash, err := hashPassword("oneoneoneoneoneoneone111")

		require.NoError(t, err)
		require.True(t, strings.HasPrefix(hash, "$argon2id$v=19$"))
	})

	t.Run("includes configured parameters", func(t *testing.T) {
		hash, err := hashPassword("oneoneoneoneoneoneone111")
		require.NoError(t, err)

		parts := strings.Split(hash, "$")
		require.Len(t, parts, 6)
		require.Equal(t, "argon2id", parts[1])
		require.Equal(t, fmt.Sprintf("m=%d,t=%d,p=%d", argon2idMemory, argon2idIterations, argon2idThreads), parts[3])
	})

	t.Run("produces a unique hash each call", func(t *testing.T) {
		hash1, err := hashPassword("oneoneoneoneoneoneone111")
		require.NoError(t, err)

		hash2, err := hashPassword("oneoneoneoneoneoneone111")
		require.NoError(t, err)

		require.NotEqual(t, hash1, hash2)
	})
}

func TestComparePasswords_Argon2id(t *testing.T) {
	password := "oneoneoneoneoneoneone111"
	hash, err := hashPassword(password)
	require.NoError(t, err)

	t.Run("matches the correct password", func(t *testing.T) {
		match, err := comparePasswords(hash, password)
		require.NoError(t, err)
		require.True(t, match)
	})

	t.Run("rejects the wrong password", func(t *testing.T) {
		match, err := comparePasswords(hash, "wrongwrongwrongwrongwrong")
		require.NoError(t, err)
		require.False(t, match)
	})

	t.Run("fails on malformed hash", func(t *testing.T) {
		_, err := comparePasswords("$argon2id$v=19$bad$salt$hash", password)
		require.ErrorContains(t, err, "invalid argon2id parameters")
	})
}

func TestComparePasswords_LegacyScrypt(t *testing.T) {
	password := "legacypasswordlegacypassword"
	hash := legacyScryptHash(t, password)

	t.Run("recognises legacy scrypt hashes", func(t *testing.T) {
		require.True(t, isLegacyHash(hash))
	})

	t.Run("matches the correct password against a legacy hash", func(t *testing.T) {
		match, err := comparePasswords(hash, password)
		require.NoError(t, err)
		require.True(t, match)
	})

	t.Run("rejects the wrong password against a legacy hash", func(t *testing.T) {
		match, err := comparePasswords(hash, "wrongwrongwrongwrongwrong")
		require.NoError(t, err)
		require.False(t, match)
	})

	t.Run("fails on malformed legacy hash", func(t *testing.T) {
		_, err := comparePasswords("notalegacyhash", password)
		require.ErrorContains(t, err, "invalid legacy password hash")
	})
}

// legacyScryptHash reproduces the pre-Argon2id encoding so tests can exercise
// the migration path without keeping a removed code path around.
func legacyScryptHash(t *testing.T, password string) string {
	t.Helper()
	salt := []byte("0123456789abcdef0123456789abcdef")
	hash, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	require.NoError(t, err)
	return fmt.Sprintf("%s.%s", hex.EncodeToString(hash), hex.EncodeToString(salt))
}
