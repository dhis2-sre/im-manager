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

	db := inttest.SetupDB(t)
	stackRepository := stack.NewRepository(db)
	stackService := stack.NewService(stackRepository)

	err := stack.LoadStacks("../../stacks", stackService)
	require.NoError(t, err, "failed to load stacks")

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		stackHandler := stack.NewHandler(stackService)
		stack.Routes(engine, func(ctx *gin.Context) {}, stackHandler)
	})

	t.Run("GetStack", func(t *testing.T) {
		t.Parallel()

		var dhis2 model.Stack
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

func TestStackModelHooksTransformParametersFromAndToMap(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	stackRepository := stack.NewRepository(db)

	st := &model.Stack{
		Name: "example",
		Parameters: map[string]model.StackParameter{
			"FIRST": {Consumed: true},
		},
	}

	err := stackRepository.Create(st)
	require.NoError(t, err)

	assert.EqualValues(t,
		[]model.StackParameter{{Name: "FIRST", StackName: "example", Consumed: true}},
		st.GormParameters)

	got, err := stackRepository.Find("example")
	require.NoError(t, err)

	assert.EqualValues(t,
		map[string]model.StackParameter{"FIRST": {Name: "FIRST", StackName: "example", Consumed: true}},
		got.Parameters)
}
