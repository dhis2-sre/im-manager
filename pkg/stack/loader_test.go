package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStacks(t *testing.T) {
	// The instance manager will fail at startup if parsing fails. Having this test is to
	// ensures that we fail even earlier when introducing a syntax error into our stack helmfiles.

	require := require.New(t)

	dir := "../../stacks/"
	_, err := parseStacks(dir)
	require.NoError(err, "error reading stack dir")

	// entries, err := os.ReadDir(dir)
	// require.NoError(err, "error reading stack dir")

	// for _, entry := range entries {
	// 	if !entry.IsDir() {
	// 		continue
	// 	}
	//
	// 	name := entry.Name()
	// 	t.Run(name, func(_ *testing.T) {
	// 		path := fmt.Sprintf("%s/%s/helmfile.yaml", dir, name)
	// 		file, err := os.ReadFile(path)
	// 		require.NoError(err, "error reading stack %q helmfile", name)
	//
	// 		tmpl := &tmpl{}
	// 		err = tmpl.parse(string(file))
	//
	// 		assert.NoError(err, "error parsing stack %q helmfile", name)
	// 	})
	// }
}

func TestExtractRequiredParameters(t *testing.T) {
	in := `
  - name: "{{ requiredEnv "INSTANCE_NAME" }}"
  - name: "{{ requiredEnv "DATABASE_NAME" }}"
  - name: "{{ requiredEnv "INSTANCE_ID" }}"
	`

	params := extractRequiredParameters([]byte(in), []string{})

	assert.Equal(t, []string{"DATABASE_NAME"}, params)
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
