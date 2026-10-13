package migratev3

import (
	"context"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestPauseAutoscaling(t *testing.T) {
	scaledObject := func(name, target string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata":   map[string]any{"name": name, "namespace": "dev"},
			"spec":       map[string]any{"scaleTargetRef": map[string]any{"name": target}},
		}}
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{scaledObjects: "ScaledObjectList"},
		scaledObject("play-7-core", "play-7-core"), scaledObject("other-7-core", "other-7-core"))
	cluster := &Cluster{client: &kube.Client{Dynamic: dynamicClient}, namespace: "dev"}
	annotation := func(name string) (string, bool) {
		object, err := dynamicClient.Resource(scaledObjects).Namespace("dev").Get(context.Background(), name, metav1.GetOptions{})
		require.NoError(t, err)
		value, ok := object.GetAnnotations()[pausedReplicasAnnotation]
		return value, ok
	}

	paused, err := cluster.pauseAutoscaling(context.Background(), []string{"play-7-core"})

	require.NoError(t, err)
	assert.Equal(t, []string{"play-7-core"}, paused)
	value, ok := annotation("play-7-core")
	assert.True(t, ok)
	assert.Equal(t, "0", value)
	_, ok = annotation("other-7-core")
	assert.False(t, ok, "a scaled object of another deployment is left alone")

	require.NoError(t, cluster.resumeAutoscaling(context.Background(), paused))

	_, ok = annotation("play-7-core")
	assert.False(t, ok, "resuming hands the scaled object back to KEDA")
}
