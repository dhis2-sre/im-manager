package stack_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	provider := model.ProviderFunc(func(instance model.Instance) (string, error) {
		return "1", nil
	})

	t.Run("Success", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Parameters: model.StackParameters{
				"a_param": {},
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"b_param": {},
			},
			Providers: model.Providers{
				"b_param_provided": provider,
			},
		}
		c := model.Stack{
			Name: "c",
			Parameters: model.StackParameters{
				"a_param": {
					Consumed: true,
				},
				"b_param_provided": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a, b},
		}

		stacks, err := stack.New(a, b, c)
		require.NoError(t, err)

		for _, stackName := range []string{"a", "b", "c"} {
			assert.Contains(t, stacks, stackName, "stack should be part of stacks")
		}
	})

	t.Run("FailGivenStackIfConsumedParameterIsNotProvidedByRequiredStack", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Providers: model.Providers{
				"a_param_provided": provider,
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"a_param": {
					Consumed: true,
				},
				"a_param_provided": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfConsumedParameterIsNotProvidedByProvider", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Parameters: model.StackParameters{
				"a_param": {},
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"a_param": {
					Consumed: true,
				},
				"a_param_provided": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `no provider for stack "b" parameter "a_param_provided"`)
	})

	t.Run("FailGivenStackIfThereAreMultipleStacksProvidingTheSameConsumedParameter", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Providers: model.Providers{
				"a_param_provided": provider,
			},
		}
		b := model.Stack{
			Name: "b",
			Providers: model.Providers{
				"a_param_provided": provider,
			},
		}
		c := model.Stack{
			Name: "c",
			Parameters: model.StackParameters{
				"a_param_provided": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a, b},
		}

		_, err := stack.New(a, b, c)

		require.ErrorContains(t, err, `stack "c" parameter "a_param_provided"`)
	})

	t.Run("FailGivenStackIfThereAreMultipleProvidersForOneConsumedParameter", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Parameters: model.StackParameters{
				"a_param": {},
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"a_param": {},
			},
		}
		c := model.Stack{
			Name: "c",
			Parameters: model.StackParameters{
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a, b},
		}

		_, err := stack.New(a, b, c)

		require.ErrorContains(t, err, `stack "c" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfARequiredStackProvidesTheSameConsumedParameterTwice", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Parameters: model.StackParameters{
				"a_param": {},
			},
			Providers: model.Providers{
				"a_param": provider,
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfItContainsDuplicateRequiredStacks", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Parameters: model.StackParameters{
				"a_param": {},
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{a, a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" requires "a" more than once`)
	})

	t.Run("FailGivenStackIfARequiredStackDoesNotProvideAnyOfItsConsumedParameters", func(t *testing.T) {
		a := model.Stack{
			Name: "a",
			Parameters: model.StackParameters{
				"a_param": {},
			},
		}
		b := model.Stack{
			Name: "b",
			Parameters: model.StackParameters{
				"b_param_1": {},
				"b_param_2": {},
			},
			Requires: []model.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" requires "a" but does not consume from "a"`)
	})
}
