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

	require.NoError(err, "error parsing stack dir")
}

// TODO move work to loader
// func TestParseYamlMetadata(t *testing.T) {
// 	t.Run("SuccessWithAllMetadata", func(t *testing.T) {
// 		assert := assert.New(t)
// 		require := require.New(t)
//
// 		in := `instanceManager:
//  consumedParameters:
//     - DATABASE_USERNAME
//     - DATABASE_PASSWORD
//     - DATABASE_NAME
//  hostnameVariable: DATABASE_HOSTNAME
//  hostnamePattern: "%s-postgresql.%s.svc"
//  stackParameters:
//     - GOOGLE_AUTH_PROJECT_ID
//     - GOOGLE_AUTH_CLIENT_ID`
// 		want := &tmpl{
// 			hostnameVariable: "DATABASE_HOSTNAME",
// 			hostnamePattern:  "%s-postgresql.%s.svc",
// 			consumedParameters: []string{
// 				"DATABASE_USERNAME",
// 				"DATABASE_PASSWORD",
// 				"DATABASE_NAME",
// 			},
// 			stackParameters: []string{
// 				"GOOGLE_AUTH_PROJECT_ID",
// 				"GOOGLE_AUTH_CLIENT_ID",
// 			},
// 		}
//
// 		tmpl := &tmpl{}
// 		err := tmpl.parse(in)
//
// 		require.NoError(err)
// 		assert.Equal(want, tmpl)
// 	})
//
// 	t.Run("SuccessWithPartialMetadata", func(t *testing.T) {
// 		assert := assert.New(t)
// 		require := require.New(t)
//
// 		in := `instanceManager:
// hostnameVariable: DATABASE_HOSTNAME
// stackParameters:
//   - GOOGLE_AUTH_PROJECT_ID
//   - GOOGLE_AUTH_CLIENT_ID`
// 		want := &tmpl{
// 			hostnameVariable: "DATABASE_HOSTNAME",
// 			stackParameters: []string{
// 				"GOOGLE_AUTH_PROJECT_ID",
// 				"GOOGLE_AUTH_CLIENT_ID",
// 			},
// 		}
//
// 		tmpl := &tmpl{}
// 		err := tmpl.parse(in)
//
// 		require.NoError(err)
// 		assert.Equal(want, tmpl)
// 	})
//
// 	t.Run("FailsWithInvalidStructure", func(t *testing.T) {
// 		require := require.New(t)
//
// 		in := `instanceManager:
// hostnameVariable:
//   - DATABASE_HOSTNAME
//   - DATABASE_PORT`
//
// 		tmpl := &tmpl{}
// 		err := tmpl.parse(in)
//
// 		require.Error(err)
// 	})
// }

// TODO remove this in the end, it was just for me to understand the old parser implementation
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
