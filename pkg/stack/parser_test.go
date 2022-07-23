package stack

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO check if I can declare the assert/require before subtests and reuse them in there

func TestParserStacks(t *testing.T) {
	// The instance manager will fail at startup if parsing fails. Having this test is to
	// ensures that we fail even earlier when introducing a syntax error into our stack helmfiles.

	assert := assert.New(t)
	require := require.New(t)

	dir := "../../stacks/"
	entries, err := os.ReadDir(dir)
	require.NoError(err, "error reading stack folder")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		t.Run(name, func(_ *testing.T) {
			path := fmt.Sprintf("%s/%s/helmfile.yaml", dir, name)
			file, err := os.ReadFile(path)
			require.NoError(err, "error reading stack %q helmfile", name)

			tmpl := &tmpl{}
			err = tmpl.parse(string(file))

			assert.NoError(err, "error parsing stack %q helmfile", name)
		})
	}
}

func TestParserYamlMetadata(t *testing.T) {
	t.Run("SuccessWithAllMetadata", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		in := `instanceManager:
 consumedParameters:
    - DATABASE_USERNAME
    - DATABASE_PASSWORD
    - DATABASE_NAME
 hostnameVariable: DATABASE_HOSTNAME
 hostnamePattern: "%s-postgresql.%s.svc"
 stackParameters:
    - GOOGLE_AUTH_PROJECT_ID
    - GOOGLE_AUTH_CLIENT_ID`
		want := &tmpl{
			hostnameVariable: "DATABASE_HOSTNAME",
			hostnamePattern:  "%s-postgresql.%s.svc",
			consumedParameters: []string{
				"DATABASE_USERNAME",
				"DATABASE_PASSWORD",
				"DATABASE_NAME",
			},
			stackParameters: []string{
				"GOOGLE_AUTH_PROJECT_ID",
				"GOOGLE_AUTH_CLIENT_ID",
			},
		}

		tmpl := &tmpl{}
		err := tmpl.parse(in)

		require.NoError(err)
		assert.Equal(want, tmpl)
	})

	t.Run("SuccessWithPartialMetadata", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		in := `instanceManager:
 hostnameVariable: DATABASE_HOSTNAME
 stackParameters:
    - GOOGLE_AUTH_PROJECT_ID
    - GOOGLE_AUTH_CLIENT_ID`
		want := &tmpl{
			hostnameVariable: "DATABASE_HOSTNAME",
			stackParameters: []string{
				"GOOGLE_AUTH_PROJECT_ID",
				"GOOGLE_AUTH_CLIENT_ID",
			},
		}

		tmpl := &tmpl{}
		err := tmpl.parse(in)

		require.NoError(err)
		assert.Equal(want, tmpl)
	})

	t.Run("FailsWithInvalidStructure", func(t *testing.T) {
		require := require.New(t)

		in := `instanceManager:
 hostnameVariable:
	- DATABASE_HOSTNAME
	- DATABASE_PORT`

		tmpl := &tmpl{}
		err := tmpl.parse(in)

		require.Error(err)
	})
}

func TestParserRequiredEnv(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		in := `{{requiredEnv "INSTANCE_NAME"}}`
		want := []string{"INSTANCE_NAME"}

		tmpl := &tmpl{}
		err := tmpl.parse(`releases:
  - name: ` + in)

		require.NoError(err)
		assert.Equal(want, tmpl.requiredEnvs)
	})

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

			tmpl := &tmpl{}
			err := tmpl.parse(`releases:
  - name: ` + te.in)

			if te.wantErr != "" {
				assert.ErrorContains(err, te.wantErr)
			} else {
				assert.Error(err)
			}
		})
	}
}

func TestParserEnv(t *testing.T) {
	tt := map[string]struct {
		in   string
		want map[string]any
	}{
		"Success": {
			in: `{{env "INSTANCE_NAME"}}`,
			want: map[string]any{
				"INSTANCE_NAME": "",
			},
		},
		"SuccessWithDefaultString": {
			in: `{{env "INSTANCE_NAME" | default "DHIS2"}}`,
			want: map[string]any{
				"INSTANCE_NAME": "DHIS2",
			},
		},
		"SuccessWithDefaultNumber": {
			in: `{{env "CHART_VERSION" | default 2}}`,
			want: map[string]any{
				"CHART_VERSION": 2,
			},
		},
	}

	for n, tt := range tt {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			tmpl := &tmpl{}
			err := tmpl.parse(`releases:
  - name: ` + tt.in)

			require.NoError(err)
			assert.Equal(tt.want, tmpl.envs)
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

			tmpl := &tmpl{}
			err := tmpl.parse(`releases:
  - name: ` + te.in)

			if te.wantErr != "" {
				assert.ErrorContains(err, te.wantErr)
			} else {
				assert.Error(err)
			}
		})
	}
}

// TODO test readFile
