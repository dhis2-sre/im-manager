package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateRequiredEnv(t *testing.T) {
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
