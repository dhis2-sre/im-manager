package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserRequiredEnv(t *testing.T) {
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
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			tmpl, err := parse(tt.in)

			require.NoError(err)
			assert.Equal(tt.want, tmpl.requiredEnvs)
		})
	}

	te := map[string]struct {
		in      string
		wantErr string
	}{
		"MissingEnv": {
			in: `{{requiredEnv}}`,
		},
		"BlankEnv": {
			in:      `{{requiredEnv "   "}}`,
			wantErr: "must provide name",
		},
	}

	for n, te := range te {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			tmpl, err := parse(te.in)

			assert.Nil(tmpl)
			if te.wantErr != "" {
				require.ErrorContains(err, te.wantErr)
			} else {
				require.Error(err)
			}
		})
	}
}
