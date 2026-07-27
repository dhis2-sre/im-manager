package kube

import (
	"context"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func componentTestInstance() *model.DeploymentInstance {
	return &model.DeploymentInstance{
		ID:    1,
		Name:  "mydb",
		Group: &model.Group{ID: 7, Namespace: "ns"},
	}
}

func componentLabels(componentName string) map[string]string {
	return map[string]string{"im-id": "1", "im-type": componentName}
}

func TestDeploymentComponentRestartPatchesDeployment(t *testing.T) {
	instance := componentTestInstance()
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "core", Namespace: "ns", Labels: componentLabels("dhis2")}}
	c := &Client{Clientset: fake.NewSimpleClientset(dep)}

	component := DeploymentComponent{BaseComponent{Name: "dhis2"}}
	require.NoError(t, component.Restart(context.Background(), c, instance))

	got, err := c.Clientset.AppsV1().Deployments("ns").Get(context.TODO(), "core", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"])
}

func TestStatefulSetComponentRestartPatchesStatefulSet(t *testing.T) {
	instance := componentTestInstance()
	set := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns", Labels: componentLabels("db")}}
	c := &Client{Clientset: fake.NewSimpleClientset(set)}

	component := StatefulSetComponent{BaseComponent{Name: "db"}}
	require.NoError(t, component.Restart(context.Background(), c, instance))

	got, err := c.Clientset.AppsV1().StatefulSets("ns").Get(context.TODO(), "db", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"])
}

func TestPVCSelectors(t *testing.T) {
	instance := componentTestInstance()

	component := BaseComponent{PVCPatterns: []string{
		"app.kubernetes.io/instance=%s-database",
		"app.kubernetes.io/instance=%s-redis",
	}}
	assert.Equal(t, []string{
		"app.kubernetes.io/instance=mydb-7-database",
		"app.kubernetes.io/instance=mydb-7-redis",
	}, component.PVCSelectors(instance))

	assert.Empty(t, BaseComponent{}.PVCSelectors(instance))
}

func componentTestPod(name, componentName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "ns",
			Labels:            componentLabels(componentName),
			CreationTimestamp: metav1.NewTime(time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)),
		},
		Status: v1.PodStatus{
			Phase:             v1.PodRunning,
			Conditions:        []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
			ContainerStatuses: []v1.ContainerStatus{{RestartCount: 2}, {RestartCount: 1}},
		},
	}
}

func TestSupportedOperations(t *testing.T) {
	assert.Equal(t, []Operation{OperationRestart, OperationRestartReplica},
		BaseComponent{Name: "db"}.SupportedOperations(nil))

	storageTypeIsFilesystem := func(params model.DeploymentInstanceParameters) bool {
		return params["STORAGE_TYPE"].Value == "filesystem"
	}
	component := BaseComponent{Name: "dhis2", Capabilities: []Capability{
		{Operation: OperationFilestoreBackup, When: storageTypeIsFilesystem},
		{Operation: Operation("alwaysOn")},
	}}

	filesystemParams := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "filesystem"}}
	assert.Equal(t, []Operation{OperationRestart, OperationRestartReplica, OperationFilestoreBackup, Operation("alwaysOn")},
		component.SupportedOperations(filesystemParams))

	minioParams := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "minio"}}
	assert.Equal(t, []Operation{OperationRestart, OperationRestartReplica, Operation("alwaysOn")},
		component.SupportedOperations(minioParams))
}

func TestReplicas(t *testing.T) {
	instance := componentTestInstance()
	pod := componentTestPod("core-1", "dhis2")
	notReady := componentTestPod("core-2", "dhis2")
	notReady.Status.Conditions = []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionFalse}}
	evicted := componentTestPod("core-evicted", "dhis2")
	evicted.Status.Phase = v1.PodFailed
	evicted.Status.Reason = "Evicted"
	otherComponent := componentTestPod("db-1", "db")
	c := &Client{Clientset: fake.NewSimpleClientset(pod, notReady, evicted, otherComponent)}

	replicas, err := BaseComponent{Name: "dhis2"}.Replicas(context.Background(), c, instance)
	require.NoError(t, err)

	assert.ElementsMatch(t, []Replica{
		{Name: "core-1", Phase: "Running", Ready: true, Restarts: 3, CreatedAt: pod.CreationTimestamp.Time},
		{Name: "core-2", Phase: "Running", Ready: false, Restarts: 3, CreatedAt: pod.CreationTimestamp.Time},
	}, replicas)
}

func TestRestartReplicaDeletesOwnedPod(t *testing.T) {
	instance := componentTestInstance()
	pod := componentTestPod("core-1", "dhis2")
	c := &Client{Clientset: fake.NewSimpleClientset(pod)}

	require.NoError(t, BaseComponent{Name: "dhis2"}.RestartReplica(context.Background(), c, instance, "core-1"))

	_, err := c.Clientset.CoreV1().Pods("ns").Get(context.TODO(), "core-1", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestRestartReplicaRejectsPodOfOtherComponent(t *testing.T) {
	instance := componentTestInstance()
	pod := componentTestPod("db-1", "db")
	c := &Client{Clientset: fake.NewSimpleClientset(pod)}

	err := BaseComponent{Name: "dhis2"}.RestartReplica(context.Background(), c, instance, "db-1")
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err))

	_, err = c.Clientset.CoreV1().Pods("ns").Get(context.TODO(), "db-1", metav1.GetOptions{})
	assert.NoError(t, err, "pod of another component must not be deleted")
}

func TestRestartReplicaMissingPod(t *testing.T) {
	instance := componentTestInstance()
	c := &Client{Clientset: fake.NewSimpleClientset()}

	err := BaseComponent{Name: "dhis2"}.RestartReplica(context.Background(), c, instance, "core-1")
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err))
}

func TestFindComponent(t *testing.T) {
	components := []Component{
		DeploymentComponent{BaseComponent{Name: "dhis2"}},
		StatefulSetComponent{BaseComponent{Name: "db"}},
	}

	found, err := FindComponent(components, "db")
	require.NoError(t, err)
	assert.Equal(t, "db", found.ComponentName())

	_, err = FindComponent(components, "missing")
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err))
}
