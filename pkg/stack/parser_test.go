package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserRequiredEnv(t *testing.T) {
	tt := map[string]struct {
		in   string
		want map[string]struct{}
	}{
		"Ok": {
			in: `releases:
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "DATABASE_PORT"}}`,
			want: map[string]struct{}{
				"DATABASE_NAME": {},
				"DATABASE_PORT": {},
			},
		},
		"OkWithoutSystemParameters": {
			in: `releases:
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "INSTANCE_ID"}}
- name: {{requiredEnv "INSTANCE_NAME"}}
- name: {{requiredEnv "INSTANCE_HOSTNAME"}}
- name: {{requiredEnv "IM_ACCESS_TOKEN"}}
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "DATABASE_PORT"}}`,
			want: map[string]struct{}{
				"DATABASE_NAME": {},
				"DATABASE_PORT": {},
			},
		},
	}

	for n, tt := range tt {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			tmpl := newTmpl("test", []string{})
			err := tmpl.parse(tt.in)

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
		"Ok": {
			in: `releases:
- name: {{env "IMAGE_REPOSITORY"}}
- name: {{env "IMAGE_REPOSITORY"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "",
			},
		},
		"OkWithoutSystemParameters": {
			in: `releases:
- name: {{env "IMAGE_REPOSITORY"}}
- name: {{env "INSTANCE_ID"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "",
			},
		},
		"OkWithDefaultString": {
			in: `releases:
- name: {{env "IMAGE_REPOSITORY" | default "dockerhub"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "dockerhub",
			},
		},
		"OkMultipleDefaultsOverride": {
			in: `releases:
- name: {{env "IMAGE_REPOSITORY" | default "dockerhub"}}
- name: {{env "IMAGE_REPOSITORY" | default "azurehub"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "azurehub",
			},
		},
		"OkWithDefaultNumber": {
			in: `releases:
- name: {{env "CHART_VERSION" | default 2}}`,
			want: map[string]any{
				"CHART_VERSION": 2,
			},
		},
	}

	for n, tt := range tt {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			tmpl := newTmpl("test", []string{})
			err := tmpl.parse(tt.in)

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

			tmpl := newTmpl("test", []string{})
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
