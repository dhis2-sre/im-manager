package stack

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// TODO other success case? or another failure test if the arg is not a string?
	tt := map[string]struct {
		in   string
		want []string
	}{
		"Success": {
			in:   `{{requiredEnv "INSTANCE_NAME"}}`,
			want: []string{"INSTANCE_NAME"},
		},
	}

	for n, tt := range tt {
		t.Run(n, func(_ *testing.T) {
			tmpl, err := parse(tt.in)

			require.NoError(err)
			assert.Equal(tt.want, tmpl.requiredEnvs)
		})
	}

	te := map[string]struct {
		in string
	}{
		"MissingEnv": {
			in: `{{requiredEnv}}`,
		},
		"BlankEnv": {
			in: `{{requiredEnv "   "}}`,
		},
	}

	for n, te := range te {
		t.Run(n, func(_ *testing.T) {
			// TODO assert tmpl is nil
			_, err := parse(te.in)
			fmt.Println(err)

			// TODO better way?
			// TODO assert "must provide name" is in error
			require.Error(err)
		})
	}
}
