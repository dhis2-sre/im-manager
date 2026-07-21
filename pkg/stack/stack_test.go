package stack_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	provider := stack.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
		return "1", nil
	})

	t.Run("Success", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"b_param": {},
			},
			ParameterProviders: stack.ParameterProviders{
				"b_param_provided": provider,
			},
		}
		c := stack.Stack{
			Name: "c",
			Parameters: stack.StackParameters{
				"a_param": {
					Consumed: true,
				},
				"b_param_provided": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a, b},
		}

		stacks, err := stack.New(a, b, c)
		require.NoError(t, err)

		for _, stackName := range []string{"a", "b", "c"} {
			assert.Contains(t, stacks, stackName, "stack should be part of stacks")
		}
	})

	t.Run("FailGivenStacksIfTheyHaveACycle", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
				"b_param": {
					Consumed: true,
				},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"b_param": {},
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a},
		}
		a.Requires = []stack.Stack{b}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `edge from stack "b" to stack "a" creates a cycle`)
	})

	t.Run("FailGivenStacksIfAStackHasASelfReferenceCycle", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		a.Requires = []stack.Stack{a}

		_, err := stack.New(a)

		require.ErrorContains(t, err, `edge from stack "a" to stack "a" creates a cycle`)
	})

	t.Run("FailGivenStackIfConsumedParameterIsNotProvidedByRequiredStack", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			ParameterProviders: stack.ParameterProviders{
				"a_param_provided": provider,
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"a_param": {
					Consumed: true,
				},
				"a_param_provided": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfConsumedParameterIsNotProvidedByProvider", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"a_param": {
					Consumed: true,
				},
				"a_param_provided": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `no provider for stack "b" parameter "a_param_provided"`)
	})

	t.Run("FailGivenStackIfConsumedParameterIsPointingToAnAlreadyConsumedParameter", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"a_param": {Consumed: true},
			},
			Requires: []stack.Stack{a},
		}
		c := stack.Stack{
			Name: "c",
			Parameters: stack.StackParameters{
				"a_param": {Consumed: true},
			},
			Requires: []stack.Stack{b},
		}

		_, err := stack.New(a, c, b)

		require.ErrorContains(t, err, `stack "c" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfThereAreMultipleStacksProvidingTheSameConsumedParameter", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			ParameterProviders: stack.ParameterProviders{
				"a_param_provided": provider,
			},
		}
		b := stack.Stack{
			Name: "b",
			ParameterProviders: stack.ParameterProviders{
				"a_param_provided": provider,
			},
		}
		c := stack.Stack{
			Name: "c",
			Parameters: stack.StackParameters{
				"a_param_provided": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a, b},
		}

		_, err := stack.New(a, b, c)

		require.ErrorContains(t, err, `stack "c" parameter "a_param_provided"`)
	})

	t.Run("FailGivenStackIfThereAreMultipleProvidersForOneConsumedParameter", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		c := stack.Stack{
			Name: "c",
			Parameters: stack.StackParameters{
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a, b},
		}

		_, err := stack.New(a, b, c)

		require.ErrorContains(t, err, `stack "c" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfARequiredStackProvidesTheSameConsumedParameterTwice", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
			ParameterProviders: stack.ParameterProviders{
				"a_param": provider,
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" parameter "a_param"`)
	})

	t.Run("FailGivenStackIfItContainsDuplicateRequiredStacks", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"a_param": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{a, a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" requires "a" more than once`)
	})

	t.Run("FailGivenStackIfARequiredStackDoesNotProvideAnyOfItsConsumedParameters", func(t *testing.T) {
		a := stack.Stack{
			Name: "a",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
		}
		b := stack.Stack{
			Name: "b",
			Parameters: stack.StackParameters{
				"b_param_1": {},
				"b_param_2": {},
			},
			Requires: []stack.Stack{a},
		}

		_, err := stack.New(a, b)

		require.ErrorContains(t, err, `stack "b" requires "a" but does not consume from "a"`)
	})
}

func TestValidatorOneOf(t *testing.T) {
	validator := stack.OneOf("ok", "not_ok")

	assert.NoError(t, validator("ok"))
	assert.NoError(t, validator("not_ok"))
	assert.ErrorContains(t, validator("maybe"), `"maybe" is not valid, only "ok", "not_ok" are allowed`)
}
