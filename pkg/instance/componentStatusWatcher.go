package instance

import (
	"context"
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

func (w *ComponentStatusWatcher) OnAdd(obj interface{}, isInInitialList bool) {
	// The initial list is the informer syncing what already runs; clients load that state through
	// the components endpoint, so only genuine additions are pushed.
	if isInInitialList {
		return
	}
	w.publish(obj, false)
}

func (w *ComponentStatusWatcher) OnUpdate(oldObj, newObj interface{}) {
	oldPod, okOld := oldObj.(*v1.Pod)
	newPod, okNew := newObj.(*v1.Pod)
	if okOld && okNew && kube.NewReplica(*oldPod) == kube.NewReplica(*newPod) {
		return
	}
	w.publish(newObj, false)
}

func (w *ComponentStatusWatcher) OnDelete(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = unknown.Obj
	}
	w.publish(obj, true)
}

func (w *ComponentStatusWatcher) publish(obj interface{}, deleted bool) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	component := pod.Labels["im-type"]
	instanceID, err := strconv.ParseUint(pod.Labels["im-id"], 10, 64)
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
