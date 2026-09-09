package kube

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// Resync is disabled: it is not a freshness mechanism (the watch keeps the store current in real
// time, and client-go relists by itself when a watch breaks), it only replays the in-memory store
// to registered event handlers, of which this cache has none. When event handlers arrive with the
// status push, a deliberate resync period can come with them.
const cacheResyncPeriod = 0

// podCache is a shared informer over the pods IM manages, those carrying the im-id label, serving
// pod reads from memory once synced. Until the initial sync completes, and if the watch ever
// breaks, readers fall back to live LISTs, so the cache degrades to the uncached behavior instead
// of becoming an outage.
type podCache struct {
	informer cache.SharedIndexInformer
	lister   corelisters.PodLister
	synced   cache.InformerSynced
	stop     chan struct{}
	logger   *slog.Logger
}

func newPodCache(logger *slog.Logger, clientset kubernetes.Interface) *podCache {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, cacheResyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "im-id"
		}))
	informer := factory.Core().V1().Pods()
	stop := make(chan struct{})
	c := &podCache{informer: informer.Informer(), lister: informer.Lister(), synced: informer.Informer().HasSynced, stop: stop, logger: logger}

	// Has to be installed before the informer runs. Without it a rejected or broken watch is only
	// reported through klog, which this service does not wire into slog, so the cache would go
	// quiet and readers would silently fall back to listing the API server.
	if err := c.informer.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		logger.Error("Pod informer watch failed, component status pushes stop until it recovers", "error", err)
	}); err != nil {
		logger.Error("Failed to install the pod informer watch error handler", "error", err)
	}

	logger.Info("Starting pod informer cache")
	factory.Start(stop)

	go func() {
		if !cache.WaitForCacheSync(stop, c.synced) {
			logger.Warn("Pod informer cache stopped before it finished syncing")
			return
		}
		logger.Info("Pod informer cache synced", "pods", len(c.informer.GetStore().List()))
	}()

	return c
}

func (p *podCache) ready() bool {
	return p != nil && p.synced()
}

// logNotReady records that a read is going to the API server rather than the cache. A nil cache is
// a client that was not built through Clients, which is the normal case in tests.
func (p *podCache) logNotReady(ctx context.Context) {
	if p == nil {
		return
	}
	p.logger.DebugContext(ctx, "Pod cache is not synced, listing from the API server instead")
}

// list returns the cached pods matching the selector, sorted by name to mirror the API server's
// ordering; an empty namespace means all namespaces.
func (p *podCache) list(namespace, selector string) ([]v1.Pod, error) {
	parsed, err := labels.Parse(selector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector %q: %v", selector, err)
	}

	var pods []*v1.Pod
	if namespace == "" {
		pods, err = p.lister.List(parsed)
	} else {
		pods, err = p.lister.Pods(namespace).List(parsed)
	}
	if err != nil {
		return nil, err
	}

	result := make([]v1.Pod, 0, len(pods))
	for _, pod := range pods {
		result = append(result, *pod)
	}
	slices.SortFunc(result, func(a, b v1.Pod) int { return cmp.Compare(a.Name, b.Name) })
	return result, nil
}

func (p *podCache) addHandler(handler cache.ResourceEventHandler) error {
	_, err := p.informer.AddEventHandler(handler)
	return err
}

func (p *podCache) close() {
	close(p.stop)
}
