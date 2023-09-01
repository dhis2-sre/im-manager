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

	helmfileParameters := make(map[string][]model.StackParameter)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		t.Logf("parsing stack: %q", name)
		st, err := parseStack(dir, name)
		require.NoError(t, err, "failed to parse stack %q", name)

		consumedParameter := make(map[string]struct{})
		for _, p := range st.consumedParameters {
			consumedParameter[p] = struct{}{}
		}

		t.Logf("stack %q: %#v", name, st)
		var parameters []model.StackParameter
		for name, value := range st.parameters {
			_, consumed := consumedParameter[name]
			parameter := model.StackParameter{Name: name, DefaultValue: value, Consumed: consumed}

			parameters = append(parameters, parameter)
		}
		helmfileParameters[name] = parameters
	}

	stackDefinitions := map[string][]model.StackParameter{
		"dhis2-db":      DHIS2DB.Parameters,
		"dhis2-core":    DHIS2Core.Parameters,
		"dhis2":         DHIS2.Parameters,
		"pgadmin":       PgAdmin.Parameters,
		"whoami-go":     WhoamiGo.Parameters,
		"im-job-runner": IMJobRunner.Parameters,
	}

	for n, p := range helmfileParameters {
		staticParameters, ok := stackDefinitions[n]
		require.Truef(t, ok, "stack %q has a helmfile but no static stack definition", n)
		assert.ElementsMatchf(t, p, staticParameters, "parameters for stack %q don't match", n)
		delete(stackDefinitions, n)
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
