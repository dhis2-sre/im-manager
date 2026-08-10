package stack_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackHandler(t *testing.T) {
	t.Parallel()

	stacks, err := stack.New(
		stack.DHIS2DB,
		stack.DHIS2Core,
		stack.DHIS2,
		stack.DHIS2V2,
		stack.Chap,
		stack.PgAdmin,
		stack.WhoamiGo,
		stack.IMJobRunner,
	)
	require.NoError(t, err)

	stackService := stack.NewService(stacks)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		stackHandler := stack.NewHandler(stackService)
		stack.Routes(engine, func(ctx *gin.Context) {}, stackHandler)
	})

	t.Run("GetStack", func(t *testing.T) {
		t.Parallel()

		var dhis2 stack.StackResponse
		client.GetJSON(t, "/stacks/dhis2", &dhis2)

		assert.Equal(t, "dhis2", dhis2.Name)
		assert.NotEmpty(t, dhis2.Parameters)
	})

	// The deploy form decides whether to offer a companion from what this endpoint serves, so the
	// condition has to survive the trip rather than staying a backend-only detail.
	t.Run("GetStackServesCompanionConditions", func(t *testing.T) {
		t.Parallel()

		var dhis2v2 stack.StackResponse
		client.GetJSON(t, "/stacks/dhis2-v2", &dhis2v2)

		require.Len(t, dhis2v2.Companions, 1)
		assert.Equal(t, "chap", dhis2v2.Companions[0].Name)
		require.NotNil(t, dhis2v2.Companions[0].When)
		assert.Equal(t, "DEPLOY_CHAP", dhis2v2.Companions[0].When.Parameter)
		assert.Equal(t, "true", dhis2v2.Companions[0].When.Equals)
	})

	t.Run("GetAllStacks", func(t *testing.T) {
		t.Parallel()

		var stacks []stack.StackResponse
		client.GetJSON(t, "/stacks", &stacks)

		assert.NotEmpty(t, stacks)
		for _, s := range stacks {
			assert.NotEmpty(t, s.Name)
			for _, p := range s.Parameters {
				assert.NotEmpty(t, p.ParameterName, "parameter in stack %q must include parameterName", s.Name)
			}
		}
	})
}
