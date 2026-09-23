package deployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/instance"
)

func TestEditRedeploysOnlyWhatChanged(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{
		deployment: deployment,
		changes:    &instance.DeploymentChanges{Deployment: deployment, Redeploy: deployment.Instances[1:]},
	}
	publisher := &recordingPublisher{}

	edited, err := newTestService(instanceService, publisher).EditDeployment(context.Background(), "token", deployment.ID, instance.Edit{}, 1)

	require.NoError(t, err)
	assert.Equal(t, deployment.ID, edited.ID)
	awaitUnlocked(t, instanceService)
	assert.Equal(t, []string{"pgadmin"}, instanceService.deployedStacks(), "the instance the edit left alone is not redeployed")
	assert.Empty(t, instanceService.destroyedStacks())
}

func TestEditDestroysACompanionBeforeRedeploying(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{
		deployment: deployment,
		changes: &instance.DeploymentChanges{
			Deployment: deployment,
			Redeploy:   deployment.Instances[:1],
			Destroy:    deployment.Instances[1:],
		},
	}

	_, err := newTestService(instanceService, &recordingPublisher{}).EditDeployment(context.Background(), "token", deployment.ID, instance.Edit{}, 1)

	require.NoError(t, err)
	awaitUnlocked(t, instanceService)
	assert.Equal(t, []string{"pgadmin"}, instanceService.destroyedStacks())
	assert.Equal(t, []string{"pgadmin"}, instanceService.deletedStacks(), "the row goes once the cluster is rid of it")
	assert.Equal(t, []string{"dhis2-v2"}, instanceService.deployedStacks())
}

func TestEditWithNoClusterWorkReleasesTheLockBeforeReturning(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{deployment: deployment, changes: &instance.DeploymentChanges{Deployment: deployment}}
	publisher := &recordingPublisher{}

	_, err := newTestService(instanceService, publisher).EditDeployment(context.Background(), "token", deployment.ID, instance.Edit{TTL: ptrTo(uint(172800))}, 1)

	require.NoError(t, err)
	assert.False(t, instanceService.isLocked())
	assert.Empty(t, instanceService.deployedStacks(), "a TTL change is not worth rolling anything for")
	assert.Empty(t, publisher.recorded(), "nothing happened in the cluster, so there is nothing to report")
}

func TestEditPersistsOnlyTheTerminalEvent(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{
		deployment: deployment,
		changes:    &instance.DeploymentChanges{Deployment: deployment, Redeploy: deployment.Instances},
	}
	publisher := &recordingPublisher{}

	_, err := newTestService(instanceService, publisher).EditDeployment(context.Background(), "token", deployment.ID, instance.Edit{}, 1)

	require.NoError(t, err)
	awaitUnlocked(t, instanceService)

	var persisted []recordedEvent
	for _, event := range publisher.recorded() {
		if !event.transient {
			persisted = append(persisted, event)
		}
	}
	require.Len(t, persisted, 1)
	assert.Equal(t, "success", persisted[0].payload.Status)
	assert.Zero(t, persisted[0].payload.InstanceID, "the bell gets the deployment, not each instance")
}

func TestEditRefusesWhileADeployIsRunning(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{deployment: deployment, locked: true}

	_, err := newTestService(instanceService, &recordingPublisher{}).EditDeployment(context.Background(), "token", deployment.ID, instance.Edit{}, 1)

	require.Error(t, err)
	assert.True(t, errdef.IsConflict(err))
}

func TestEditReleasesTheLockWhenTheEditIsRejected(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{deployment: deployment, editErr: errdef.NewBadRequest("nope")}

	_, err := newTestService(instanceService, &recordingPublisher{}).EditDeployment(context.Background(), "token", deployment.ID, instance.Edit{}, 1)

	require.Error(t, err)
	assert.False(t, instanceService.isLocked())
}

func ptrTo[T any](v T) *T { return &v }
