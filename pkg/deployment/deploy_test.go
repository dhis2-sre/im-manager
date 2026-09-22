package deployment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

type fakeTokenService struct{}

func (fakeTokenService) RefreshAccessToken(string) (string, error) { return "refreshed", nil }

type recordedEvent struct {
	kind      string
	transient bool
	payload   deploymentEvent
}

type recordingPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (p *recordingPublisher) Publish(_ context.Context, _ uint, _, kind string, payload any) {
	p.record(kind, false, payload)
}

func (p *recordingPublisher) PublishTransient(_ context.Context, _, kind string, payload any) {
	p.record(kind, true, payload)
}

func (p *recordingPublisher) record(kind string, transient bool, payload any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	event, _ := payload.(deploymentEvent)
	p.events = append(p.events, recordedEvent{kind: kind, transient: transient, payload: event})
}

func (p *recordingPublisher) recorded() []recordedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]recordedEvent(nil), p.events...)
}

// fakeInstanceService is a deployment whose instances deploy without a cluster, with a deploy lock
// that behaves like the real one: one holder at a time.
type fakeInstanceService struct {
	mu         sync.Mutex
	deployment *model.Deployment
	locked     bool
	deployed   []string
	statuses   map[uint]model.DeployStatus
	deployErr  error
	deployHook func()
}

func (f *fakeInstanceService) DeploymentOrder(deployment *model.Deployment) ([]*model.DeploymentInstance, error) {
	return deployment.Instances, nil
}

func (f *fakeInstanceService) DeployInstance(_ context.Context, _ string, deploymentInstance *model.DeploymentInstance, _ uint, _ map[string]string, _ *model.Database) error {
	if f.deployHook != nil {
		f.deployHook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deployErr != nil {
		return f.deployErr
	}
	f.deployed = append(f.deployed, deploymentInstance.StackName)
	return nil
}

func (f *fakeInstanceService) DestroyInstance(context.Context, *model.DeploymentInstance) error {
	return nil
}

func (f *fakeInstanceService) FindDecryptedDeploymentById(context.Context, uint) (*model.Deployment, error) {
	return f.deployment, nil
}

func (f *fakeInstanceService) SaveDeployment(context.Context, *model.Deployment) error { return nil }

func (f *fakeInstanceService) UpdateInstanceParameters(context.Context, uint, uint, instance.Parameters, *bool) (*model.DeploymentInstance, error) {
	panic("not used")
}

func (f *fakeInstanceService) FilestoreBackup(context.Context, *model.DeploymentInstance, string, *model.Database) error {
	panic("not used")
}

func (f *fakeInstanceService) AcquireDeployLock(context.Context, uint) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.locked {
		return false, nil
	}
	f.locked = true
	return true, nil
}

func (f *fakeInstanceService) ReleaseDeployLock(context.Context, uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locked = false
	return nil
}

func (f *fakeInstanceService) SetDeployStatus(_ context.Context, deploymentInstance *model.DeploymentInstance, status model.DeployStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statuses == nil {
		f.statuses = map[uint]model.DeployStatus{}
	}
	f.statuses[deploymentInstance.ID] = status
	return nil
}

func (f *fakeInstanceService) isLocked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.locked
}

func (f *fakeInstanceService) deployedStacks() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deployed...)
}

func newTestDeployment() *model.Deployment {
	deployment := &model.Deployment{ID: 1, Name: "whoami", GroupName: "packages"}
	deployment.Instances = []*model.DeploymentInstance{
		{ID: 10, Name: "whoami", StackName: "dhis2-v2", DeploymentID: 1},
		{ID: 11, Name: "whoami", StackName: "pgadmin", DeploymentID: 1},
	}
	return deployment
}

func newTestService(instanceService *fakeInstanceService, publisher Publisher) Service {
	return Service{
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		instanceService: instanceService,
		tokenService:    fakeTokenService{},
		publisher:       publisher,
	}
}

func awaitUnlocked(t *testing.T, instanceService *fakeInstanceService) {
	t.Helper()
	require.Eventually(t, func() bool { return !instanceService.isLocked() }, 2*time.Second, 5*time.Millisecond)
}

func TestStartDeploymentDeploysEveryInstanceAndReleasesTheLock(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{deployment: deployment}
	publisher := &recordingPublisher{}
	service := newTestService(instanceService, publisher)

	require.NoError(t, service.StartDeployment(context.Background(), "token", deployment.ID, 7))
	awaitUnlocked(t, instanceService)

	assert.Equal(t, []string{"dhis2-v2", "pgadmin"}, instanceService.deployedStacks())
	assert.Equal(t, model.DeployStatusPending, instanceService.statuses[10])
	assert.Equal(t, model.DeployStatusPending, instanceService.statuses[11])
}

// Progress is per instance and streamed only; the bell gets the outcome of the deploy and nothing
// else, so a deployment of any size leaves exactly one notification behind.
func TestStartDeploymentPersistsOnlyTheTerminalEvent(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{deployment: deployment}
	publisher := &recordingPublisher{}
	service := newTestService(instanceService, publisher)

	require.NoError(t, service.StartDeployment(context.Background(), "token", deployment.ID, 7))
	awaitUnlocked(t, instanceService)

	var persisted []recordedEvent
	for _, event := range publisher.recorded() {
		assert.Equal(t, kindDeployment, event.kind)
		if !event.transient {
			persisted = append(persisted, event)
		}
	}
	require.Len(t, persisted, 1)
	assert.Equal(t, "success", persisted[0].payload.Status)
	assert.Zero(t, persisted[0].payload.InstanceID)
}

func TestStartDeploymentReportsAFailedInstance(t *testing.T) {
	deployment := newTestDeployment()
	instanceService := &fakeInstanceService{deployment: deployment, deployErr: errors.New("helmfile exploded")}
	publisher := &recordingPublisher{}
	service := newTestService(instanceService, publisher)

	require.NoError(t, service.StartDeployment(context.Background(), "token", deployment.ID, 7))
	awaitUnlocked(t, instanceService)

	events := publisher.recorded()
	require.NotEmpty(t, events)
	last := events[len(events)-1]
	assert.False(t, last.transient)
	assert.Equal(t, "error", last.payload.Status)
	assert.Contains(t, last.payload.Error, "helmfile exploded")
}

// A second deploy is refused rather than left to race helm for the same releases.
func TestStartDeploymentRefusesASecondDeploy(t *testing.T) {
	deployment := newTestDeployment()
	release := make(chan struct{})
	instanceService := &fakeInstanceService{deployment: deployment, deployHook: func() { <-release }}
	service := newTestService(instanceService, &recordingPublisher{})

	require.NoError(t, service.StartDeployment(context.Background(), "token", deployment.ID, 7))

	err := service.StartDeployment(context.Background(), "token", deployment.ID, 7)
	require.Error(t, err)
	assert.True(t, errdef.IsConflict(err), "expected a conflict, got %v", err)

	close(release)
	awaitUnlocked(t, instanceService)
}

func TestResetRefusesWhileADeployIsRunning(t *testing.T) {
	deployment := newTestDeployment()
	release := make(chan struct{})
	instanceService := &fakeInstanceService{deployment: deployment, deployHook: func() { <-release }}
	service := newTestService(instanceService, &recordingPublisher{})

	require.NoError(t, service.StartDeployment(context.Background(), "token", deployment.ID, 7))

	err := service.Reset(context.Background(), "token", deployment.ID, deployment.Instances[0].ID, 60, 7)
	require.Error(t, err)
	assert.True(t, errdef.IsConflict(err), "expected a conflict, got %v", err)

	close(release)
	awaitUnlocked(t, instanceService)
}
