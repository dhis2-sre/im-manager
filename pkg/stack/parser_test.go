package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	in := `{{requiredEnv "INSTANCE_NAME"}}`
	// tmpl := `{{env "INSTANCE_NAME"}}`

	tmpl, err := parse(in)

	require.NoError(err)
	// want := map[string]struct{}{"INSTANCE_NAME": {}}
	want := []string{"INSTANCE_NAME"}
	assert.Equal(want, tmpl.requiredEnvs)
}
