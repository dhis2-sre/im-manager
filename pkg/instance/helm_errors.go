package instance

import (
	"regexp"

	"github.com/dhis2-sre/im-manager/internal/errdef"
)

// helmErrorPattern maps a regex over helm/helmfile stderr to a constructor for a
// typed errdef error with a user-actionable message. Patterns are evaluated in
// order; the first match wins.
type helmErrorPattern struct {
	re     *regexp.Regexp
	format func(match []string) error
}

var helmErrorPatterns = []helmErrorPattern{
	{
		// helm 3.x against a missing namespace when --create-namespace is not set:
		//   Error: ... namespaces "whoami" not found
		re: regexp.MustCompile(`namespaces "([^"]+)" not found`),
		format: func(m []string) error {
			return errdef.NewBadRequest("namespace %q does not exist on the target cluster; the group needs to be set up before instances can be deployed to it", m[1])
		},
	},
	{
		// helm 3.x with --create-namespace under a ServiceAccount that lacks the perm:
		//   namespaces is forbidden: User "system:serviceaccount:..." cannot create resource "namespaces" in API group "" at the cluster scope
		re: regexp.MustCompile(`namespaces is forbidden:[^\n]*cannot create resource "namespaces"`),
		format: func(m []string) error {
			return errdef.NewServiceUnavailable("the cluster does not permit im-manager to create namespaces; the stack's helmfile must set helmDefaults.createNamespace: false")
		},
	},
}

// classifyHelmfileError inspects helm/helmfile stderr from a failed sync and
// returns a typed errdef error with an actionable message for known patterns,
// or nil if no pattern matched (the caller should fall back to the generic 500).
func classifyHelmfileError(stderr string) error {
	for _, p := range helmErrorPatterns {
		if m := p.re.FindStringSubmatch(stderr); m != nil {
			return p.format(m)
		}
	}
	return nil
}
