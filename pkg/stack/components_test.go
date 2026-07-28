package stack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// allStacks are the deployable stack definitions; every one must declare its components. The
// job-runner is exempt: it is an old jobs experiment that only labels pods, and jobs get their
// own design in a separate task.
var allStacks = []Stack{
	DHIS2DB, MINIO, DHIS2Core, DHIS2, DHIS2V2, PgAdmin, WhoamiGo,
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

	assert.Empty(t, IMJobRunner.Components, "im-job-runner deliberately has no components until jobs are redesigned")
}

// TestComponentRestartTargetsExpectedWorkload asserts each technology component patches the
// Kubernetes workload kind its chart actually deploys.
func TestComponentRestartTargetsExpectedWorkload(t *testing.T) {
	instance := &model.DeploymentInstance{ID: 1, Name: "myinstance", Group: &model.Group{ID: 7, Namespace: "ns"}}

	tests := []struct {
		component   kube.Component
		statefulSet bool
	}{
		{DHIS2CoreComponent{kube.BaseComponent{Name: "dhis2"}}, false},
		{BitnamiPostgresComponent{kube.BaseComponent{Name: "db"}}, true},
		{CNPGPostgresComponent{kube.BaseComponent{Name: "chap-db"}}, true},
		{MinioComponent{kube.BaseComponent{Name: "minio"}}, false},
		{PgAdminComponent{kube.BaseComponent{Name: "pgadmin"}}, true},
		{WhoamiComponent{kube.BaseComponent{Name: "whoami"}}, false},
		{ValkeyComponent{kube.BaseComponent{Name: "chap-valkey"}}, true},
		{ChapWorkerComponent{kube.BaseComponent{Name: "chap-worker"}}, false},
		{ChapCoreComponent{kube.BaseComponent{Name: "chap-core"}}, false},
	}

	for _, test := range tests {
		name := test.component.ComponentName()
		t.Run(fmt.Sprintf("%T", test.component), func(t *testing.T) {
			labels := map[string]string{"im-id": "1", "im-type": name}
			meta := metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: labels}

			var client *kube.Client
			if test.statefulSet {
				client = &kube.Client{Clientset: fake.NewSimpleClientset(&appsv1.StatefulSet{ObjectMeta: meta})}
			} else {
				client = &kube.Client{Clientset: fake.NewSimpleClientset(&appsv1.Deployment{ObjectMeta: meta})}
			}

			require.NoError(t, test.component.Restart(context.Background(), client, instance))

			var annotations map[string]string
			if test.statefulSet {
				got, err := client.Clientset.AppsV1().StatefulSets("ns").Get(context.TODO(), name, metav1.GetOptions{})
				require.NoError(t, err)
				annotations = got.Spec.Template.Annotations
			} else {
				got, err := client.Clientset.AppsV1().Deployments("ns").Get(context.TODO(), name, metav1.GetOptions{})
				require.NoError(t, err)
				annotations = got.Spec.Template.Annotations
			}
			assert.NotEmpty(t, annotations["kubectl.kubernetes.io/restartedAt"])
		})
	}
}

// TestDHIS2V2MinioComponentPresence asserts the minio component only exists when the file store
// lives in the bundled MinIO, mirroring the chart's minio.enabled condition.
func TestDHIS2V2MinioComponentPresence(t *testing.T) {
	names := func(params model.DeploymentInstanceParameters) []string {
		var names []string
		for _, c := range kube.PresentComponents(DHIS2V2.Components, params) {
			names = append(names, c.ComponentName())
		}
		return names
	}

	minioParams := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "minio"}}
	assert.Equal(t, []string{"dhis2", "db", "minio"}, names(minioParams))

	filesystemParams := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "filesystem"}}
	assert.Equal(t, []string{"dhis2", "db"}, names(filesystemParams))

	s3Params := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "s3"}}
	assert.Equal(t, []string{"dhis2", "db"}, names(s3Params))
}

// TestDHIS2CoreAdvertisesFilestoreBackup asserts the capability listing: the dhis2-core component
// supports filestore backup for every storage backend, while other stacks only expose the base
// operations.
func TestDHIS2CoreAdvertisesFilestoreBackup(t *testing.T) {
	core, err := kube.FindComponent(DHIS2Core.Components, "dhis2")
	require.NoError(t, err)
	assert.Contains(t, core.SupportedOperations(nil), kube.OperationFilestoreBackup)

	whoami, err := kube.FindComponent(WhoamiGo.Components, "whoami")
	require.NoError(t, err)
	assert.Equal(t, []kube.Operation{kube.OperationRestart, kube.OperationRestartReplica}, whoami.SupportedOperations(nil))
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
		// dhis2-v2 postdates the hardcoded map; the release's own PVCs share its instance label so
		// selectors are qualified by chart name, and the CNPG cluster labels its volumes itself.
		"dhis2-v2": {
			"app.kubernetes.io/instance=%s,app.kubernetes.io/name=dhis2",
			"cnpg.io/cluster=%s-dhis2-postgresql",
			"app.kubernetes.io/instance=%s,app.kubernetes.io/name=minio",
		},
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
