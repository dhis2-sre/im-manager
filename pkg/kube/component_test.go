package kube

import (
	"context"
	"testing"

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

func TestPodComponentRestartDeletesMatchingPods(t *testing.T) {
	instance := componentTestInstance()
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "job-abc", Namespace: "ns", Labels: componentLabels("job")}}
	other := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "ns", Labels: map[string]string{"im-id": "1", "im-type": "other"}}}
	c := &Client{Clientset: fake.NewSimpleClientset(pod, other)}

	component := PodComponent{BaseComponent{Name: "job"}}
	require.NoError(t, component.Restart(context.Background(), c, instance))

	remaining, err := c.Clientset.CoreV1().Pods("ns").List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, remaining.Items, 1)
	assert.Equal(t, "unrelated", remaining.Items[0].Name)
}

func TestPodComponentRestartNoMatchIsNoOp(t *testing.T) {
	instance := componentTestInstance()
	c := &Client{Clientset: fake.NewSimpleClientset()}

	component := PodComponent{BaseComponent{Name: "job"}}
	require.NoError(t, component.Restart(context.Background(), c, instance))
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
