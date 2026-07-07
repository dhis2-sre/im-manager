package token_test

import (
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/stretchr/testify/require"
)

func TestDeleteRefreshTokensOnlyDeletesTokensOfGivenUser(t *testing.T) {
	redis := inttest.SetupRedis(t)
	repository := token.NewRepository(redis)

	require.NoError(t, repository.SetRefreshToken(1, "token-user-1", time.Minute))
	require.NoError(t, repository.SetRefreshToken(10, "token-user-10", time.Minute))

	require.NoError(t, repository.DeleteRefreshTokens(1))

	require.Error(t, repository.DeleteRefreshToken(1, "token-user-1"), "user 1's refresh token should have been deleted")
	require.NoError(t, repository.DeleteRefreshToken(10, "token-user-10"), "user 10's refresh token should still exist")
}
