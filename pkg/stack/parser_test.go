package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserRequiredEnv(t *testing.T) {
	tt := map[string]struct {
		template    string
		stackParams map[string]struct{}
		want        map[string]struct{}
	}{
		"Ok": {
			template: `releases:
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "DATABASE_PORT"}}`,
			want: map[string]struct{}{
				"DATABASE_NAME": {},
				"DATABASE_PORT": {},
			},
		},
		"OkWithoutSystemParameters": {
			template: `releases:
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
		"OkWithoutStackParameters": {
			template: `releases:
- name: {{requiredEnv "DATABASE_NAME"}}
- name: {{requiredEnv "DATABASE_MANAGER_URL"}}`,
			stackParams: map[string]struct{}{
				"DATABASE_MANAGER_URL": {},
			},
			want: map[string]struct{}{
				"DATABASE_NAME": {},
			},
		},
	}

	for n, tt := range tt {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			tmpl := newTmpl("test", tt.stackParams)
			err := tmpl.parse(tt.template)

			require.NoError(err)
			assert.Equal(tt.want, tmpl.requiredEnvs)
		})
	}

	te := map[string]struct {
		template string
		wantErr  string
	}{
		"MissingEnv": {
			template: `{{requiredEnv}}`,
		},
		"BlankEnv": {
			template: `{{requiredEnv "   "}}`,
			wantErr:  "must provide name",
		},
	}

	for n, te := range te {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)

			tmpl := &tmpl{}
			err := tmpl.parse(`releases:
  - name: ` + te.template)

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
		template    string
		stackParams map[string]struct{}
		want        map[string]any
	}{
		"Ok": {
			template: `releases:
- name: {{env "IMAGE_REPOSITORY"}}
- name: {{env "IMAGE_REPOSITORY"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "",
			},
		},
		"OkWithoutSystemParameters": {
			template: `releases:
- name: {{env "IMAGE_REPOSITORY"}}
- name: {{env "INSTANCE_ID"}}
- name: {{env "INSTANCE_NAME"}}
- name: {{env "INSTANCE_HOSTNAME"}}
- name: {{env "IM_ACCESS_TOKEN"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "",
			},
		},
		"OkWithoutStackParameters": {
			template: `releases:
- name: {{env "IMAGE_REPOSITORY"}}
- name: {{env "DATABASE_MANAGER_URL"}}`,
			stackParams: map[string]struct{}{
				"DATABASE_MANAGER_URL": {},
			},
			want: map[string]any{
				"IMAGE_REPOSITORY": "",
			},
		},
		"OkWithDefaultString": {
			template: `releases:
- name: {{env "IMAGE_REPOSITORY" | default "dockerhub"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "dockerhub",
			},
		},
		"OkMultipleDefaultsOverride": {
			template: `releases:
- name: {{env "IMAGE_REPOSITORY" | default "dockerhub"}}
- name: {{env "IMAGE_REPOSITORY" | default "azurehub"}}`,
			want: map[string]any{
				"IMAGE_REPOSITORY": "azurehub",
			},
		},
		"OkWithDefaultNumber": {
			template: `releases:
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

			tmpl := newTmpl("test", tt.stackParams)
			err := tmpl.parse(tt.template)

			require.NoError(err)
			assert.Equal(tt.want, tmpl.envs)
		})
	}

	te := map[string]struct {
		template string
		wantErr  string
	}{
		"MissingEnv": {
			template: `{{requiredEnv}}`,
		},
		"BlankEnv": {
			template: `{{requiredEnv "   "}}`,
			wantErr:  "must provide name",
		},
	}

	for n, te := range te {
		t.Run(n, func(t *testing.T) {
			assert := assert.New(t)

			tmpl := newTmpl("test", map[string]struct{}{})
			err := tmpl.parse(`releases:
  - name: ` + te.template)

			if te.wantErr != "" {
				assert.ErrorContains(err, te.wantErr)
			} else {
				assert.Error(err)
			}
		})
	}
}

// TODO test readFile
