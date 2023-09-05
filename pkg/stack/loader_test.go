package stack

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

func TestStackDefinitionsAreInSyncWithHelmfile(t *testing.T) {
	// assert every stack defined in Go has a helmfile
	// assert every stack helmfile has a stack definition in Go
	// assert that the parameters their default value and whether they are consumed are in sync
	dir := "../../stacks"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	helmfileParameters := make(map[string]model.StackParameters)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stackName := entry.Name()
		t.Logf("parsing stack: %q", stackName)
		st, err := parseStack(dir, stackName)
		require.NoError(t, err, "failed to parse stack %q", stackName)

		consumedParameter := make(map[string]struct{})
		for _, p := range st.consumedParameters {
			consumedParameter[p] = struct{}{}
		}

		t.Logf("stack %q: %#v", stackName, st)
		parameters := make(model.StackParameters)
		for name, value := range st.parameters {
			_, consumed := consumedParameter[name]
			parameters[name] = model.StackParameter{DefaultValue: value, Consumed: consumed}
		}
		helmfileParameters[stackName] = parameters
	}

	// helmfileParameters will not contain Go Validator or Provider functions. We therefore need to
	// create map of stack name to parameters with parameters only containing. DefaultValue and
	// Consumed as we cannot ignore fields in the assertions we use.
	stacks := map[string]model.StackParameters{
		"dhis2-db":      DHIS2DB.Parameters,
		"dhis2-core":    DHIS2Core.Parameters,
		"dhis2":         DHIS2.Parameters,
		"pgadmin":       PgAdmin.Parameters,
		"whoami-go":     WhoamiGo.Parameters,
		"im-job-runner": IMJobRunner.Parameters,
	}
	stackDefinitions := make(map[string]model.StackParameters)
	for stackName, stackParameters := range stacks {
		stackDefinitionParameters := make(map[string]model.StackParameter, len(stackParameters))
		for parameterName, parameter := range stackParameters {
			stackDefinitionParameters[parameterName] = model.StackParameter{DefaultValue: parameter.DefaultValue, Consumed: parameter.Consumed}
		}
		stackDefinitions[stackName] = stackDefinitionParameters
	}
	require.NoError(t, err)

	for name, parameters := range helmfileParameters {
		stackDefinition, ok := stackDefinitions[name]
		require.Truef(t, ok, "stack %q has a helmfile but no static stack definition", name)
		assert.Equalf(t, parameters, stackDefinition, "parameters for stack %q don't match", name)
		delete(stackDefinitions, name)
	}

	assert.Empty(t, stackDefinitions, "all stack definitions should have a helmfile, these don't")
}

func TestIsSystemParameterPositive(t *testing.T) {
	const instanceId = "INSTANCE_ID"

	parameter := isSystemParameter(instanceId)

	assert.True(t, parameter)
}

func TestIsSystemParameterNegative(t *testing.T) {
	const instanceId = "some-random-parameter-name"

	parameter := isSystemParameter(instanceId)

	assert.False(t, parameter)
}
