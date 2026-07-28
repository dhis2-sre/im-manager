package kube

import (
	"context"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// testComponent is a minimal concrete Component; the real technology-named types live in pkg/stack.
type testComponent struct {
	BaseComponent
}

func (c testComponent) Restart(context.Context, *Client, *model.DeploymentInstance) error {
	return nil
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

func TestPresentComponents(t *testing.T) {
	always := testComponent{BaseComponent{Name: "always"}}
	whenMinio := testComponent{BaseComponent{Name: "minio", When: func(params model.DeploymentInstanceParameters) bool {
		return params["STORAGE_TYPE"].Value == "minio"
	}}}
	components := []Component{always, whenMinio}

	assert.True(t, always.Present(nil))

	minioParams := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "minio"}}
	assert.Len(t, PresentComponents(components, minioParams), 2)

	s3Params := model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "s3"}}
	present := PresentComponents(components, s3Params)
	require.Len(t, present, 1)
	assert.Equal(t, "always", present[0].ComponentName())
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
		testComponent{BaseComponent{Name: "dhis2"}},
		testComponent{BaseComponent{Name: "db"}},
	}

	found, err := FindComponent(components, "db")
	require.NoError(t, err)
	assert.Equal(t, "db", found.ComponentName())

	_, err = FindComponent(components, "missing")
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err))
}
