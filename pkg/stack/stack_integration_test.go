package stack_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
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

		// TODO: Don't use dto here... Use model.Stack once we actually return it
		var dhis2 stack.Stack
		client.GetJSON(t, "/stacks/dhis2", &dhis2)

		assert.Equal(t, "dhis2", dhis2.Name)
	})

	t.Run("GetAllStacks", func(t *testing.T) {
		t.Parallel()

		var stacks []model.Stack
		client.GetJSON(t, "/stacks", &stacks)

		assert.NotEmpty(t, stacks)
	})
}
