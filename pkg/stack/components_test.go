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

// allStacks are the deployable stack definitions; every one must declare its components.
var allStacks = All

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
		{ValkeyComponent{kube.BaseComponent{Name: "valkey"}}, false},
		{ChapAPIComponent{kube.BaseComponent{Name: "api"}}, false},
		{ChapWorkerComponent{kube.BaseComponent{Name: "worker"}}, false},
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

// TestDorisComponentRestartPatchesCluster asserts the Doris component restarts through the
// DorisCluster custom resource, stamping both tiers so the operator rolls frontends and backends,
// rather than touching the statefulsets it owns.
func TestDorisComponentRestartPatchesCluster(t *testing.T) {
	instance := &model.DeploymentInstance{ID: 1, Name: "myinstance", Group: &model.Group{ID: 7, Namespace: "ns"}}
	gvr := schema.GroupVersionResource{Group: "doris.selectdb.com", Version: "v1", Resource: "dorisclusters"}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "doris.selectdb.com/v1",
		"kind":       "DorisCluster",
		"metadata":   map[string]any{"name": "myinstance-7-doris", "namespace": "ns"},
	}}
	scheme := runtime.NewScheme()
	client := &kube.Client{Dynamic: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{gvr: "DorisClusterList"}, cluster)}

	component := DorisComponent{BaseComponent: kube.BaseComponent{Name: "doris"}, ClusterPattern: "%s-doris"}
	require.NoError(t, component.Restart(context.Background(), client, instance))

	got, err := client.Dynamic.Resource(gvr).Namespace("ns").Get(context.Background(), "myinstance-7-doris", metav1.GetOptions{})
	require.NoError(t, err)
	annotations := got.GetAnnotations()
	assert.NotEmpty(t, annotations["apache.doris.fe/restartedAt"])
	assert.NotEmpty(t, annotations["apache.doris.be/restartedAt"])
}

// TestDHIS2V2DorisComponentPresence asserts the doris component only exists when the analytics
// database is enabled, mirroring the chart's dorisCluster.enabled condition.
func TestDHIS2V2DorisComponentPresence(t *testing.T) {
	present := func(params model.DeploymentInstanceParameters) bool {
		for _, c := range kube.PresentComponents(DHIS2V2.Components, params) {
			if c.ComponentName() == "doris" {
				return true
			}
		}
		return false
	}

	assert.True(t, present(model.DeploymentInstanceParameters{"ENABLE_DORIS": {Value: "true"}}))
	assert.False(t, present(model.DeploymentInstanceParameters{"ENABLE_DORIS": {Value: "false"}}))
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
		// chap postdates the map too; the CNPG cluster labels its own volumes and the valkey
		// subchart's PVC is qualified by chart name since it shares the release's instance label.
		"chap": {
			"cnpg.io/cluster=%s-chap-db",
			"app.kubernetes.io/instance=%s-chap,app.kubernetes.io/name=valkey",
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
	for _, s := range []Stack{DHIS2DB, DHIS2, DHIS2V2, Chap} {
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

	// chap's db does not advertise databaseSave yet: saving CHAP is planned, but needs its
	// parameter names mapped in the dump config and a seed/restore path first. Enabling it then
	// is just adding the capability to its component.
	chapAccess, err := kube.FindPostgresAccess(Chap.Components)
	require.NoError(t, err)
	assert.NotContains(t, chapAccess.(kube.Component).SupportedOperations(params), kube.OperationDatabaseSave)
}

// TestParameterGroupsBelongToComponents asserts every parameter group can be attributed to a
// component, either by naming one or, when it is conditional, by hanging off a parameter that lives
// in a group that does. The instance details page attributes an instance's parameters to components
// exactly this way, so a group satisfying neither leaves its parameters in a list dangling below the
// components rather than under the thing they configure.
func TestParameterGroupsBelongToComponents(t *testing.T) {
	for _, s := range allStacks {
		components := make(map[string]bool, len(s.Components))
		for _, component := range s.Components {
			components[component.ComponentName()] = true
		}

		for _, group := range s.ParameterGroups {
			if components[group.Name] {
				continue
			}
			require.NotNilf(t, group.When, "stack %q group %q names no component and has no condition to attribute it by", s.Name, group.Name)
			owner := s.Parameters[group.When.Parameter].Group
			assert.Truef(t, components[owner], "stack %q group %q is conditional on %q, which lives in group %q, which names no component", s.Name, group.Name, group.When.Parameter, owner)
		}
	}
}
