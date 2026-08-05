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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// allStacks are the deployable stack definitions; every one must declare its components. The
// job-runner is exempt: it is an old jobs experiment that only labels pods, and jobs get their
// own design in a separate task.
var allStacks = func() []Stack {
	var stacks []Stack
	for _, s := range All {
		if s.Name == IMJobRunner.Name {
			continue
		}
		stacks = append(stacks, s)
	}
	return stacks
}()

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
		// CNPGPostgresComponent is absent: its restart patches the Cluster custom resource, covered
		// by TestCNPGComponentRestartPatchesCluster.
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

// TestCNPGComponentRestartPatchesCluster asserts the CNPG component restarts through the Cluster
// custom resource, named by the cluster pattern, rather than touching the operator's pods.
func TestCNPGComponentRestartPatchesCluster(t *testing.T) {
	instance := &model.DeploymentInstance{ID: 1, Name: "myinstance", Group: &model.Group{ID: 7, Namespace: "ns"}}
	gvr := schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]any{"name": "myinstance-7-dhis2-postgresql", "namespace": "ns"},
	}}
	scheme := runtime.NewScheme()
	client := &kube.Client{Dynamic: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{gvr: "ClusterList"}, cluster)}

	component := CNPGPostgresComponent{BaseComponent: kube.BaseComponent{Name: "db"}, ClusterPattern: "%s-dhis2-postgresql"}
	require.NoError(t, component.Restart(context.Background(), client, instance))

	got, err := client.Dynamic.Resource(gvr).Namespace("ns").Get(context.Background(), "myinstance-7-dhis2-postgresql", metav1.GetOptions{})
	require.NoError(t, err)
	annotations := got.GetAnnotations()
	assert.NotEmpty(t, annotations["kubectl.kubernetes.io/restartedAt"])
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

func TestPostgresPod(t *testing.T) {
	instance := &model.DeploymentInstance{ID: 1, Name: "myinstance", Group: &model.Group{ID: 7, Namespace: "ns"}}

	t.Run("BitnamiViaImLabels", func(t *testing.T) {
		component := BitnamiPostgresComponent{BaseComponent: kube.BaseComponent{Name: "db"}}
		pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance-database-0",
			Namespace: "ns",
			Labels:    map[string]string{"im-id": "1", "im-type": "db"},
		}}
		client := &kube.Client{Clientset: fake.NewSimpleClientset(pod)}

		name, container, err := component.PostgresPod(context.Background(), client, instance)

		require.NoError(t, err)
		assert.Equal(t, "myinstance-database-0", name)
		assert.Equal(t, "postgresql", container)
	})

	t.Run("BitnamiNoPod", func(t *testing.T) {
		component := BitnamiPostgresComponent{BaseComponent: kube.BaseComponent{Name: "db"}}
		client := &kube.Client{Clientset: fake.NewSimpleClientset()}

		_, _, err := component.PostgresPod(context.Background(), client, instance)

		require.ErrorContains(t, err, "no postgres pod found")
	})

	t.Run("CNPGViaPrimaryRoleLabel", func(t *testing.T) {
		component := CNPGPostgresComponent{BaseComponent: kube.BaseComponent{Name: "db"}, ClusterPattern: "%s-dhis2-postgresql"}
		primary := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance-7-dhis2-postgresql-1",
			Namespace: "ns",
			Labels:    map[string]string{"cnpg.io/cluster": "myinstance-7-dhis2-postgresql", "cnpg.io/instanceRole": "primary"},
		}}
		replica := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance-7-dhis2-postgresql-2",
			Namespace: "ns",
			Labels:    map[string]string{"cnpg.io/cluster": "myinstance-7-dhis2-postgresql", "cnpg.io/instanceRole": "replica"},
		}}
		client := &kube.Client{Clientset: fake.NewSimpleClientset(primary, replica)}

		name, container, err := component.PostgresPod(context.Background(), client, instance)

		require.NoError(t, err)
		assert.Equal(t, "myinstance-7-dhis2-postgresql-1", name)
		assert.Equal(t, "postgres", container)
	})

	t.Run("CNPGNoPrimary", func(t *testing.T) {
		component := CNPGPostgresComponent{BaseComponent: kube.BaseComponent{Name: "db"}, ClusterPattern: "%s-dhis2-postgresql"}
		client := &kube.Client{Clientset: fake.NewSimpleClientset()}

		_, _, err := component.PostgresPod(context.Background(), client, instance)

		require.ErrorContains(t, err, "expected one primary pod")
	})
}

func TestFindPostgresAccess(t *testing.T) {
	for _, s := range []Stack{DHIS2DB, DHIS2, DHIS2V2, ChapDB} {
		_, err := kube.FindPostgresAccess(s.Components)
		assert.NoErrorf(t, err, "stack %q should have a postgres component", s.Name)
	}

	_, err := kube.FindPostgresAccess(WhoamiGo.Components)
	require.ErrorContains(t, err, "no postgres component found")
}

func TestDatabaseSaveCapability(t *testing.T) {
	params := model.DeploymentInstanceParameters{}
	for _, s := range []Stack{DHIS2DB, DHIS2, DHIS2V2} {
		access, err := kube.FindPostgresAccess(s.Components)
		require.NoErrorf(t, err, "stack %q", s.Name)
		component := access.(kube.Component)
		assert.Containsf(t, component.SupportedOperations(params), kube.OperationDatabaseSave, "stack %q should advertise databaseSave", s.Name)
	}

	// chap-db does not advertise databaseSave yet: saving CHAP is planned, but needs its
	// parameter names mapped in the dump config and a seed/restore path first. Enabling it then
	// is just adding the capability to its component.
	chapAccess, err := kube.FindPostgresAccess(ChapDB.Components)
	require.NoError(t, err)
	assert.NotContains(t, chapAccess.(kube.Component).SupportedOperations(params), kube.OperationDatabaseSave)
}
