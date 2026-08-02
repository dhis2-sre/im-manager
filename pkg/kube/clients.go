package kube

import (
	"fmt"
	"sync"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Clients caches one Client per cluster so the SOPS kubeconfig decrypt, REST config and client
// construction happen once per cluster instead of once per request. Entries are keyed by cluster
// id and UpdatedAt, so saving a cluster (e.g. rotating its kubeconfig) naturally invalidates the
// cached client without any explicit hook.
type Clients struct {
	mu        sync.Mutex
	byCluster map[string]*Client
	build     func(model.Cluster) (*Client, error)
}

func NewClients() *Clients {
	return &Clients{byCluster: map[string]*Client{}, build: NewClient}
}

// For returns the cached client for the cluster, building it on first use.
func (c *Clients) For(cluster model.Cluster) (*Client, error) {
	key := fmt.Sprintf("%d-%d", cluster.ID, cluster.UpdatedAt.UnixNano())

	c.mu.Lock()
	defer c.mu.Unlock()

	if client, ok := c.byCluster[key]; ok {
		return client, nil
	}

	client, err := c.build(cluster)
	if err != nil {
		return nil, err
	}

	// Drop any stale entry for the same cluster under an older UpdatedAt.
	for existing := range c.byCluster {
		var id uint
		var updated int64
		if _, err := fmt.Sscanf(existing, "%d-%d", &id, &updated); err == nil && id == cluster.ID && existing != key {
			delete(c.byCluster, existing)
		}
	}

	c.byCluster[key] = client
	return client, nil
}
