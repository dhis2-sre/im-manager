package instance

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

const kindComponentStatus = "component-status"

// componentStatusEvent is the transient wire event for a single replica transition. The frontend
// merges it into the component view it loaded from the components endpoint; Deleted removes the
// replica.
type componentStatusEvent struct {
	DeploymentID uint         `json:"deploymentId"`
	InstanceID   uint         `json:"instanceId"`
	Component    string       `json:"component"`
	Replica      kube.Replica `json:"replica"`
	Deleted      bool         `json:"deleted,omitempty"`
}

type transientPublisher interface {
	PublishTransient(ctx context.Context, groupName, kind string, payload any)
}

type instanceLookup interface {
	FindDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error)
}

// ComponentStatusWatcher turns pod informer events into transient component-status events on the
// notification stream, scoped to the owning instance's group. It implements
// cache.ResourceEventHandler and is registered on every cluster's pod informer.
type ComponentStatusWatcher struct {
	logger    *slog.Logger
	instances instanceLookup
	publisher transientPublisher
}

func NewComponentStatusWatcher(logger *slog.Logger, instances instanceLookup, publisher transientPublisher) *ComponentStatusWatcher {
	return &ComponentStatusWatcher{logger: logger, instances: instances, publisher: publisher}
}

var _ cache.ResourceEventHandler = &ComponentStatusWatcher{}

func (w *ComponentStatusWatcher) OnAdd(obj any, isInInitialList bool) {
	// The initial list is the informer syncing what already runs; clients load that state through
	// the components endpoint, so only genuine additions are pushed.
	if isInInitialList {
		return
	}
	if pod, ok := w.podFrom(obj); ok {
		w.publish(pod, false)
	}
}

func (w *ComponentStatusWatcher) OnUpdate(oldObj, newObj any) {
	newPod, ok := w.podFrom(newObj)
	if !ok {
		return
	}
	// Writes that leave the replica view untouched, e.g. an unrelated annotation, are not worth an
	// event. Without the previous pod there is nothing to compare, so the event goes out.
	if oldPod, ok := w.podFrom(oldObj); ok && kube.NewReplica(*oldPod) == kube.NewReplica(*newPod) {
		return
	}
	w.publish(newPod, false)
}

func (w *ComponentStatusWatcher) OnDelete(obj any) {
	if pod, ok := w.podFrom(obj); ok {
		w.publish(pod, true)
	}
}

// podFrom recovers the pod from an informer event payload, unwrapping the tombstone the informer
// delivers when a delete went unobserved. Anything else is logged rather than dropped in silence,
// since that would leave clients stale with nothing pointing at why.
func (w *ComponentStatusWatcher) podFrom(obj any) (*v1.Pod, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	pod, ok := obj.(*v1.Pod)
	if !ok {
		w.logger.Warn("Ignoring pod informer event of unexpected type", "type", fmt.Sprintf("%T", obj))
		return nil, false
	}
	return pod, true
}

func (w *ComponentStatusWatcher) publish(pod *v1.Pod, deleted bool) {
	component := pod.Labels["im-type"]
	instanceID, err := strconv.ParseUint(pod.Labels["im-id"], 10, strconv.IntSize)
	if component == "" || err != nil {
		return
	}

	ctx := context.Background()
	instance, err := w.instances.FindDeploymentInstanceById(ctx, uint(instanceID))
	if err != nil {
		// Pods of instances IM no longer knows, e.g. lingering after a delete, are not an error.
		w.logger.DebugContext(ctx, "Skipping component status for unknown instance", "instanceId", instanceID, "pod", pod.Name)
		return
	}

	w.publisher.PublishTransient(ctx, instance.GroupName, kindComponentStatus, componentStatusEvent{
		DeploymentID: instance.DeploymentID,
		InstanceID:   instance.ID,
		Component:    component,
		Replica:      kube.NewReplica(*pod),
		Deleted:      deleted,
	})
}
