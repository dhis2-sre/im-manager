package instance

import (
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
)

func TestClassifyHelmfileError(t *testing.T) {
	tests := []struct {
		name      string
		stderr    string
		wantNil   bool
		wantCheck func(error) bool
		wantSub   string
	}{
		{
			name:      "namespace forbidden create",
			stderr:    `namespaces is forbidden: User "system:serviceaccount:instance-manager-dev:im-manager-dev" cannot create resource "namespaces" in API group "" at the cluster scope`,
			wantCheck: errdef.IsServiceUnavailable,
			wantSub:   "helmDefaults.createNamespace: false",
		},
		{
			name:      "namespace not found",
			stderr:    `Error: INSTALLATION FAILED: 1 error occurred: * namespaces "whoami" not found`,
			wantCheck: errdef.IsBadRequest,
			wantSub:   `"whoami"`,
		},
		{
			name:    "unrelated error returns nil",
			stderr:  `Error: failed to download "bitnami/postgresql"`,
			wantNil: true,
		},
		{
			name:    "empty stderr returns nil",
			stderr:  "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHelmfileError(tt.stderr)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("classifyHelmfileError returned %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("classifyHelmfileError returned nil, want non-nil")
			}
			if !tt.wantCheck(got) {
				t.Fatalf("classifyHelmfileError returned %v, did not match the expected errdef predicate", got)
			}
			if tt.wantSub != "" && !strings.Contains(got.Error(), tt.wantSub) {
				t.Fatalf("classifyHelmfileError returned %q, want substring %q", got.Error(), tt.wantSub)
			}
		})
	}
}
