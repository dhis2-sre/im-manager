package kube

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func imPod(name, namespace string, labels map[string]string) *v1.Pod {
	labels["im-id"] = "1"
	return &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}
}

func waitSynced(t *testing.T, cache *podCache) {
	require.Eventually(t, cache.ready, 5*time.Second, 10*time.Millisecond, "informer should sync")
}

func TestPodCacheServesListPods(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		imPod("db-a", "ns1", map[string]string{"im-type": "db"}),
		imPod("db-b", "ns2", map[string]string{"im-type": "db"}),
		imPod("web-a", "ns1", map[string]string{"im-type": "web"}),
	)
	cache := newPodCache(slog.Default(), clientset)
	defer cache.close()
	waitSynced(t, cache)

	client := &Client{Clientset: clientset, pods: cache}

	pods, err := client.ListPods(context.Background(), "ns1", "im-type=db")
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "db-a", pods[0].Name)

	all, err := client.ListPods(context.Background(), "", "im-type=db")
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "db-a", all[0].Name, "results are name sorted")
	assert.Equal(t, "db-b", all[1].Name)
}

func TestPodCacheSeesUpdates(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cache := newPodCache(slog.Default(), clientset)
	defer cache.close()
	waitSynced(t, cache)

	client := &Client{Clientset: clientset, pods: cache}
	_, err := clientset.CoreV1().Pods("ns1").Create(context.Background(), imPod("late", "ns1", map[string]string{"im-type": "db"}), metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		pods, err := client.ListPods(context.Background(), "ns1", "im-type=db")
		return err == nil && len(pods) == 1
	}, 5*time.Second, 10*time.Millisecond, "watch should deliver the new pod")
}

func TestListPodsFallsBackWithoutCache(t *testing.T) {
	clientset := fake.NewSimpleClientset(imPod("db-a", "ns1", map[string]string{"im-type": "db"}))
	client := &Client{Clientset: clientset}

	pods, err := client.ListPods(context.Background(), "ns1", "im-type=db")

	require.NoError(t, err)
	require.Len(t, pods, 1)
}
