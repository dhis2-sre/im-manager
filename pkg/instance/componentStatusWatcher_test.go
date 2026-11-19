package instance

import (
	"context"
	"log/slog"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type recordedEvent struct {
	group   string
	kind    string
	payload any
}

type recordingPublisher struct {
	events []recordedEvent
}

func (r *recordingPublisher) PublishTransient(ctx context.Context, groupName, kind string, payload any) {
	r.events = append(r.events, recordedEvent{groupName, kind, payload})
}

type stubLookup struct {
	instance *model.DeploymentInstance
}

func (s stubLookup) FindDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	if s.instance == nil {
		return nil, errdef.NewNotFound("instance not found")
	}
	return s.instance, nil
}

func watcherPod(phase v1.PodPhase, ready bool) *v1.Pod {
	readyStatus := v1.ConditionFalse
	if ready {
		readyStatus = v1.ConditionTrue
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "db-0",
			Labels: map[string]string{"im-id": "42", "im-type": "db"},
		},
		Status: v1.PodStatus{
			Phase:      phase,
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: readyStatus}},
		},
	}
}

func TestComponentStatusWatcher(t *testing.T) {
	logger := slog.Default()
	instance := &model.DeploymentInstance{ID: 42, DeploymentID: 7, GroupName: "whoami"}

	t.Run("PublishesOnMeaningfulUpdate", func(t *testing.T) {
		publisher := &recordingPublisher{}
		watcher := NewComponentStatusWatcher(logger, stubLookup{instance: instance}, publisher)

		watcher.OnUpdate(watcherPod(v1.PodPending, false), watcherPod(v1.PodRunning, true))

		require.Len(t, publisher.events, 1)
		assert.Equal(t, "whoami", publisher.events[0].group)
		assert.Equal(t, "component-status", publisher.events[0].kind)
		event := publisher.events[0].payload.(componentStatusEvent)
		assert.Equal(t, uint(7), event.DeploymentID)
		assert.Equal(t, uint(42), event.InstanceID)
		assert.Equal(t, "db", event.Component)
		assert.Equal(t, "Running", event.Replica.Phase)
		assert.True(t, event.Replica.Ready)
		assert.False(t, event.Deleted)
	})

	t.Run("SkipsNoopUpdate", func(t *testing.T) {
		publisher := &recordingPublisher{}
		watcher := NewComponentStatusWatcher(logger, stubLookup{instance: instance}, publisher)

		watcher.OnUpdate(watcherPod(v1.PodRunning, true), watcherPod(v1.PodRunning, true))

		assert.Empty(t, publisher.events)
	})

	t.Run("SkipsInitialListAdds", func(t *testing.T) {
		publisher := &recordingPublisher{}
		watcher := NewComponentStatusWatcher(logger, stubLookup{instance: instance}, publisher)

		watcher.OnAdd(watcherPod(v1.PodRunning, true), true)
		assert.Empty(t, publisher.events)

		watcher.OnAdd(watcherPod(v1.PodPending, false), false)
		assert.Len(t, publisher.events, 1)
	})

	t.Run("DeleteCarriesDeletedFlag", func(t *testing.T) {
		publisher := &recordingPublisher{}
		watcher := NewComponentStatusWatcher(logger, stubLookup{instance: instance}, publisher)

		watcher.OnDelete(watcherPod(v1.PodRunning, true))

		require.Len(t, publisher.events, 1)
		assert.True(t, publisher.events[0].payload.(componentStatusEvent).Deleted)
	})

	t.Run("SkipsUnknownInstanceAndUnlabelledPods", func(t *testing.T) {
		publisher := &recordingPublisher{}
		watcher := NewComponentStatusWatcher(logger, stubLookup{}, publisher)
		watcher.OnAdd(watcherPod(v1.PodRunning, true), false)

		unlabelled := watcherPod(v1.PodRunning, true)
		unlabelled.Labels = map[string]string{}
		known := NewComponentStatusWatcher(logger, stubLookup{instance: instance}, publisher)
		known.OnAdd(unlabelled, false)

		assert.Empty(t, publisher.events)
	})
}

func TestComponentStatusWatcherIgnoresUnexpectedTypes(t *testing.T) {
	publisher := &recordingPublisher{}
	instance := &model.DeploymentInstance{ID: 42, DeploymentID: 7, GroupName: "whoami"}
	watcher := NewComponentStatusWatcher(slog.Default(), stubLookup{instance: instance}, publisher)

	// Nothing here is a pod, so none of it may panic or publish.
	watcher.OnAdd("not a pod", false)
	watcher.OnUpdate(42, "still not a pod")
	watcher.OnDelete(nil)
	assert.Empty(t, publisher.events)

	// A tombstone wrapping a real pod is a genuine delete.
	watcher.OnDelete(cache.DeletedFinalStateUnknown{Key: "ns/db-0", Obj: watcherPod(v1.PodRunning, true)})
	require.Len(t, publisher.events, 1)
	assert.True(t, publisher.events[0].payload.(componentStatusEvent).Deleted)

	// An update whose previous object is not a pod still publishes, there is nothing to compare to.
	watcher.OnUpdate("not a pod", watcherPod(v1.PodRunning, true))
	assert.Len(t, publisher.events, 2)
}
