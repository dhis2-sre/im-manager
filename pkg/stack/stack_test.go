package stack_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/kube"
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

	// A companion consumes from whichever stack offers it, so the provider is found by looking at the
	// stacks declaring it rather than at its own Requires, which a companion does not have.
	t.Run("GivenCompanionConsumingFromItsHost", func(t *testing.T) {
		companion := stack.Stack{
			Name: "companion",
			Parameters: stack.StackParameters{
				"a_param": {Consumed: true},
			},
		}
		host := stack.Stack{
			Name: "host",
			Parameters: stack.StackParameters{
				"a_param": {},
			},
			Companions: []stack.Companion{{Stack: companion}},
		}

		_, err := stack.New(host, companion)

		require.NoError(t, err)
	})

	t.Run("FailGivenCompanionWhoseHostDoesNotProvideItsConsumedParameter", func(t *testing.T) {
		companion := stack.Stack{
			Name: "companion",
			Parameters: stack.StackParameters{
				"a_param": {Consumed: true},
			},
		}
		host := stack.Stack{
			Name:       "host",
			Parameters: stack.StackParameters{},
			Companions: []stack.Companion{{Stack: companion}},
		}

		_, err := stack.New(host, companion)

		require.ErrorContains(t, err, `no provider for stack "companion" parameter "a_param"`)
	})

	// Each host provides on its own, since only one of them is deployed alongside the companion.
	// Summing across hosts would read two perfectly good hosts as a duplicate-provider ambiguity.
	t.Run("GivenCompanionOfferedBySeveralHostsEachProviding", func(t *testing.T) {
		companion := stack.Stack{
			Name: "companion",
			Parameters: stack.StackParameters{
				"a_param": {Consumed: true},
			},
		}
		hostA := stack.Stack{
			Name:       "host-a",
			Parameters: stack.StackParameters{"a_param": {}},
			Companions: []stack.Companion{{Stack: companion}},
		}
		hostB := stack.Stack{
			Name:       "host-b",
			Parameters: stack.StackParameters{"a_param": {}},
			Companions: []stack.Companion{{Stack: companion}},
		}

		_, err := stack.New(hostA, hostB, companion)

		require.NoError(t, err)
	})

	t.Run("FailGivenCompanionOfferedBySeveralHostsWhereOneDoesNotProvide", func(t *testing.T) {
		companion := stack.Stack{
			Name: "companion",
			Parameters: stack.StackParameters{
				"a_param": {Consumed: true},
			},
		}
		good := stack.Stack{
			Name:       "good-host",
			Parameters: stack.StackParameters{"a_param": {}},
			Companions: []stack.Companion{{Stack: companion}},
		}
		bad := stack.Stack{
			Name:       "bad-host",
			Parameters: stack.StackParameters{},
			Companions: []stack.Companion{{Stack: companion}},
		}

		_, err := stack.New(good, bad, companion)

		require.ErrorContains(t, err, `bad-host`)
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

func TestValidateCompanionConditions(t *testing.T) {
	t.Run("ConditionOnAnExistingParameter", func(t *testing.T) {
		offered := stack.Stack{Name: "offered"}
		offering := stack.Stack{
			Name:       "offering",
			Parameters: stack.StackParameters{"DEPLOY_IT": {}},
			Companions: []stack.Companion{{Stack: offered, When: &kube.Condition{Parameter: "DEPLOY_IT", Equals: "true"}}},
		}

		err := stack.ValidateCompanionConditions([]stack.Stack{offering, offered})

		require.NoError(t, err)
	})

	t.Run("UnconditionalCompanion", func(t *testing.T) {
		offered := stack.Stack{Name: "offered"}
		offering := stack.Stack{Name: "offering", Companions: []stack.Companion{{Stack: offered}}}

		err := stack.ValidateCompanionConditions([]stack.Stack{offering, offered})

		require.NoError(t, err)
	})

	// A condition naming a parameter the stack does not have never holds, so the companion would
	// silently never be offered instead of failing loudly at startup.
	t.Run("ConditionOnAMissingParameter", func(t *testing.T) {
		offered := stack.Stack{Name: "offered"}
		offering := stack.Stack{
			Name:       "offering",
			Companions: []stack.Companion{{Stack: offered, When: &kube.Condition{Parameter: "TYPO", Equals: "true"}}},
		}

		err := stack.ValidateCompanionConditions([]stack.Stack{offering, offered})

		require.ErrorContains(t, err, `stack "offering" offers companion "offered" conditional on parameter "TYPO" which it does not have`)
	})
}

// The deploy form decides whether to offer CHAP from this declaration, so it has to survive a
// rename of the parameter that gates it.
func TestChapIsOfferedOnDeployChap(t *testing.T) {
	for _, s := range []stack.Stack{stack.DHIS2Core, stack.DHIS2V2} {
		var found bool
		for _, companion := range s.Companions {
			if companion.Stack.Name != stack.Chap.Name {
				continue
			}
			found = true
			require.NotNilf(t, companion.When, "stack %q offers chap unconditionally", s.Name)
			assert.Equalf(t, "DEPLOY_CHAP", companion.When.Parameter, "stack %q", s.Name)
			assert.Equalf(t, "true", companion.When.Equals, "stack %q", s.Name)
		}
		assert.Truef(t, found, "stack %q does not offer chap as a companion", s.Name)
	}
}
