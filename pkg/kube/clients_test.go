package kube

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestClientsCachePerCluster(t *testing.T) {
	builds := 0
	clients := NewClients(slog.Default())
	clients.build = func(cluster model.Cluster) (*Client, error) {
		builds++
		return &Client{Clientset: fake.NewSimpleClientset()}, nil
	}

	updated := time.Now()
	clusterA := model.Cluster{ID: 1, UpdatedAt: updated}
	clusterB := model.Cluster{ID: 2, UpdatedAt: updated}

	first, err := clients.For(clusterA)
	require.NoError(t, err)
	second, err := clients.For(clusterA)
	require.NoError(t, err)
	assert.Same(t, first, second)
	assert.Equal(t, 1, builds)

	_, err = clients.For(clusterB)
	require.NoError(t, err)
	assert.Equal(t, 2, builds)

	// Saving the cluster bumps UpdatedAt, invalidating the cached client.
	clusterA.UpdatedAt = updated.Add(time.Minute)
	third, err := clients.For(clusterA)
	require.NoError(t, err)
	assert.NotSame(t, first, third)
	assert.Equal(t, 3, builds)
	assert.Len(t, clients.byCluster, 2, "the stale entry for the updated cluster is dropped")

	select {
	case <-first.pods.stop:
	default:
		t.Fatal("the stale client's informer should be stopped")
	}
}
