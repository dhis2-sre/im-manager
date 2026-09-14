package kube

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"k8s.io/client-go/tools/cache"
)

// Clients caches one Client per cluster so the SOPS kubeconfig decrypt, REST config and client
// construction happen once per cluster instead of once per request. Entries are keyed by cluster
// id and UpdatedAt, so saving a cluster (e.g. rotating its kubeconfig) naturally invalidates the
// cached client without any explicit hook.
type Clients struct {
	mu          sync.Mutex
	logger      *slog.Logger
	byCluster   map[string]*Client
	build       func(model.Cluster) (*Client, error)
	podHandlers []cache.ResourceEventHandler
}

// RegisterPodHandler attaches the handler to every cluster's pod informer, present and future.
// Handlers registered after a cache started still receive adds for everything already in the
// store, so registration order and cache construction order do not matter.
func (c *Clients) RegisterPodHandler(handler cache.ResourceEventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.podHandlers = append(c.podHandlers, handler)
	for _, client := range c.byCluster {
		if client.pods != nil {
			if err := client.pods.addHandler(handler); err != nil {
				panic(fmt.Sprintf("failed to register pod event handler on a running cache: %v", err))
			}
		}
	}
}

func NewClients(logger *slog.Logger) *Clients {
	return &Clients{logger: logger, byCluster: map[string]*Client{}, build: NewClient}
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
	if client.Clientset != nil {
		client.pods = newPodCache(c.logger.With("clusterId", cluster.ID, "clusterName", cluster.Name, "clusterHost", restHost(client)), client.Clientset)
		for _, handler := range c.podHandlers {
			if err := client.pods.addHandler(handler); err != nil {
				return nil, fmt.Errorf("failed to register pod event handler: %v", err)
			}
		}
	}

	// Drop any stale entry for the same cluster under an older UpdatedAt.
	for existing := range c.byCluster {
		var id uint
		var updated int64
		if _, err := fmt.Sscanf(existing, "%d-%d", &id, &updated); err == nil && id == cluster.ID && existing != key {
			if stale := c.byCluster[existing]; stale.pods != nil {
				stale.pods.close()
			}
			delete(c.byCluster, existing)
		}
	}

	c.byCluster[key] = client
	c.logger.Info("Built kube client", "clusterId", cluster.ID, "clusterName", cluster.Name, "clusterHost", restHost(client), "podHandlers", len(c.podHandlers))
	return client, nil
}

// restHost identifies which API server a client and its cache belong to. A group can carry a
// zero-value cluster, and then the id and name say nothing about where pods are really watched.
func restHost(client *Client) string {
	if client.RestConfig == nil {
		return ""
	}
	return client.RestConfig.Host
}
