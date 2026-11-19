package user_test

import (
	"context"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignInWithSSO(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	userService := user.NewService("", 10, user.NewRepository(db), fakeDialer{t})

	t.Run("ReturnsGroupsOfExistingUser", func(t *testing.T) {
		existing, group := user.CreateUserWithGroup(t, db, "sso-group", "sso.dhis2.org", "sso", "sso-existing@dhis2.org")

		signedIn, err := userService.SignInWithSSO(context.Background(), existing.Email)
		require.NoError(t, err)

		assert.Equal(t, existing.ID, signedIn.ID)
		assert.True(t, signedIn.Validated)
		require.Len(t, signedIn.Groups, 1)
		assert.Equal(t, group.Name, signedIn.Groups[0].Name)
	})

	t.Run("CreatesValidatedUserWithoutGroups", func(t *testing.T) {
		signedIn, err := userService.SignInWithSSO(context.Background(), "sso-new@dhis2.org")
		require.NoError(t, err)

		assert.NotZero(t, signedIn.ID)
		assert.True(t, signedIn.Validated)
		assert.Empty(t, signedIn.Groups)
	})
}
