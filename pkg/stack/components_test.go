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

// expectedStacks is the set of stacks that must have components declared.
var expectedStacks = []string{
	"dhis2-db", "minio", "dhis2-core", "dhis2", "pgadmin", "whoami-go",
	"im-job-runner", "chap-db", "chap-valkey", "chap-worker", "chap-core",
}

func TestEveryStackHasUniqueNamedComponents(t *testing.T) {
	for _, name := range expectedStacks {
		comps, ok := components[name]
		require.Truef(t, ok, "stack %q has no components", name)
		require.NotEmptyf(t, comps, "stack %q has no components", name)

		seen := map[string]bool{}
		for _, c := range comps {
			require.Falsef(t, seen[c.ComponentName()], "stack %q has duplicate component %q", name, c.ComponentName())
			seen[c.ComponentName()] = true
		}
	}

	assert.Len(t, components, len(expectedStacks), "components registry has an unexpected number of stacks")
}

// TestComponentNamesMatchHelmfileImType asserts every declared component name is an im-type label
// applied by the stack's helmfile, so operations never target a label no workload carries.
func TestComponentNamesMatchHelmfileImType(t *testing.T) {
	for _, name := range expectedStacks {
		imTypes := helmfileImTypes(t, name)
		for _, c := range components[name] {
			assert.Containsf(t, imTypes, c.ComponentName(),
				"stack %q component %q is not an im-type in its helmfile", name, c.ComponentName())
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

	for _, name := range expectedStacks {
		var want []string
		for _, pattern := range oldMap[name] {
			want = append(want, replacePlaceholder(pattern, "mydb-7"))
		}

		var got []string
		for _, c := range components[name] {
			got = append(got, c.PVCSelectors(instance)...)
		}

		assert.Equalf(t, want, got, "PVC selector parity mismatch for stack %q", name)
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
