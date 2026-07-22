package stack

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allStacks are the deployable stack definitions; every one must declare its components.
var allStacks = []Stack{
	DHIS2DB, MINIO, DHIS2Core, DHIS2, PgAdmin, WhoamiGo, IMJobRunner,
	ChapDB, ChapValkey, ChapWorker, ChapCore,
}

func TestEveryStackHasUniqueNamedComponents(t *testing.T) {
	for _, s := range allStacks {
		require.NotEmptyf(t, s.Components, "stack %q has no components", s.Name)

		seen := map[string]bool{}
		for _, c := range s.Components {
			require.Falsef(t, seen[c.ComponentName()], "stack %q has duplicate component %q", s.Name, c.ComponentName())
			seen[c.ComponentName()] = true
		}
	}
}

// TestComponentNamesMatchHelmfileImType asserts every declared component name is an im-type label
// applied by the stack's helmfile, so operations never target a label no workload carries.
func TestComponentNamesMatchHelmfileImType(t *testing.T) {
	for _, s := range allStacks {
		imTypes := helmfileImTypes(t, s.Name)
		for _, c := range s.Components {
			assert.Containsf(t, imTypes, c.ComponentName(),
				"stack %q component %q is not an im-type in its helmfile", s.Name, c.ComponentName())
		}
	}
}

// TestComponentPVCSelectorParity asserts the union of each stack's component PVC selectors equals
// the historic hardcoded map's output (empty for stacks that had no entry).
func TestComponentPVCSelectorParity(t *testing.T) {
	oldMap := map[string][]string{
		"dhis2":      {"app.kubernetes.io/instance=%s-database", "app.kubernetes.io/instance=%s-redis"},
		"dhis2-core": {"app.kubernetes.io/instance=%s", "app.kubernetes.io/instance=%s-minio"},
		"dhis2-db":   {"app.kubernetes.io/instance=%s-database"},
		"minio":      {"app.kubernetes.io/instance=%s-minio"},
	}

	instance := &model.DeploymentInstance{Name: "mydb", Group: &model.Group{ID: 7}}

	for _, s := range allStacks {
		var want []string
		for _, pattern := range oldMap[s.Name] {
			want = append(want, replacePlaceholder(pattern, "mydb-7"))
		}

		var got []string
		for _, c := range s.Components {
			got = append(got, c.PVCSelectors(instance)...)
		}

		assert.Equalf(t, want, got, "PVC selector parity mismatch for stack %q", s.Name)
	}
}

func replacePlaceholder(pattern, value string) string {
	return regexp.MustCompile(`%s`).ReplaceAllString(pattern, value)
}

func helmfileImTypes(t *testing.T, stackDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "stacks", stackDir, "helmfile.yaml.gotmpl"))
	require.NoError(t, err)

	re := regexp.MustCompile(`im-type:\s*"([^"]+)"`)
	var imTypes []string
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		imTypes = append(imTypes, match[1])
	}
	return imTypes
}
