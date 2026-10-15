package deployment

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

type fakeBatchInstanceService struct {
	*fakeInstanceService
	deployments map[uint]*model.Deployment
	locks       map[uint]bool
}

func newFakeBatchInstanceService(count uint) *fakeBatchInstanceService {
	f := &fakeBatchInstanceService{fakeInstanceService: &fakeInstanceService{}, deployments: map[uint]*model.Deployment{}, locks: map[uint]bool{}}
	for id := uint(1); id <= count; id++ {
		name := fmt.Sprintf("bulk-%d", id)
		f.deployments[id] = &model.Deployment{ID: id, Name: name, GroupName: "packages", Instances: []*model.DeploymentInstance{{ID: 100 + id, Name: name, StackName: name, DeploymentID: id, DeployStatus: model.DeployStatusPending}}}
	}
	return f
}

func (f *fakeBatchInstanceService) FindDecryptedDeploymentById(_ context.Context, id uint) (*model.Deployment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	deployment, ok := f.deployments[id]
	if !ok {
		return nil, errdef.NewNotFound("deployment not found by id: %d", id)
	}
	found := *deployment
	found.Instances = nil
	for _, deploymentInstance := range deployment.Instances {
		instanceCopy := *deploymentInstance
		found.Instances = append(found.Instances, &instanceCopy)
	}
	return &found, nil
}

func (f *fakeBatchInstanceService) markDeployed(id uint) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, deploymentInstance := range f.deployments[id].Instances {
		deploymentInstance.DeployStatus = model.DeployStatusDeployed
	}
}

func (f *fakeBatchInstanceService) unlock(id uint) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locks[id] = false
}

func (f *fakeBatchInstanceService) AcquireDeployLock(_ context.Context, id uint) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.locks[id] {
		return false, nil
	}
	f.locks[id] = true
	return true, nil
}

func (f *fakeBatchInstanceService) ReleaseDeployLock(_ context.Context, id uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locks[id] = false
	return nil
}

func (f *fakeBatchInstanceService) isLockedById(id uint) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.locks[id]
}

func countTerminalEvents(publisher *recordingPublisher) int {
	count := 0
	for _, event := range publisher.recorded() {
		if !event.transient {
			count++
		}
	}
	return count
}

func TestStartDeploymentsDeploysABatchAFewAtATime(t *testing.T) {
	instanceService := newFakeBatchInstanceService(7)
	var running, most atomic.Int32
	instanceService.deployHook = func() {
		now := running.Add(1)
		for {
			seen := most.Load()
			if now <= seen || most.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
	}
	publisher := &recordingPublisher{}
	service := Service{logger: newTestService(nil, nil).logger, instanceService: instanceService, tokenService: fakeTokenService{}, publisher: publisher}

	require.NoError(t, service.StartDeployments(context.Background(), "token", []uint{1, 2, 3, 4, 5, 6, 7}, 7))

	require.Eventually(t, func() bool { return countTerminalEvents(publisher) == 7 }, 2*time.Second, 5*time.Millisecond)
	assert.Len(t, instanceService.deployedStacks(), 7)
	assert.Equal(t, int32(BulkDeployConcurrency), most.Load())
}

func TestStartDeploymentsWaitsForADeploymentSomeoneElseIsDeploying(t *testing.T) {
	queuedLockRetry = 10 * time.Millisecond
	instanceService := newFakeBatchInstanceService(3)
	instanceService.locks[2] = true
	publisher := &recordingPublisher{}
	service := Service{logger: newTestService(nil, nil).logger, instanceService: instanceService, tokenService: fakeTokenService{}, publisher: publisher}

	require.NoError(t, service.StartDeployments(context.Background(), "token", []uint{1, 2, 3}, 7))
	require.Eventually(t, func() bool { return countTerminalEvents(publisher) == 2 }, 2*time.Second, 5*time.Millisecond)
	assert.ElementsMatch(t, []string{"bulk-1", "bulk-3"}, instanceService.deployedStacks())

	instanceService.unlock(2)
	require.Eventually(t, func() bool { return countTerminalEvents(publisher) == 3 }, 2*time.Second, 5*time.Millisecond)
	assert.ElementsMatch(t, []string{"bulk-1", "bulk-2", "bulk-3"}, instanceService.deployedStacks())
}

func TestStartDeploymentsSkipsADeploymentDeployedWhileItWaited(t *testing.T) {
	queuedLockRetry = 10 * time.Millisecond
	instanceService := newFakeBatchInstanceService(2)
	instanceService.locks[2] = true
	publisher := &recordingPublisher{}
	service := Service{logger: newTestService(nil, nil).logger, instanceService: instanceService, tokenService: fakeTokenService{}, publisher: publisher}

	require.NoError(t, service.StartDeployments(context.Background(), "token", []uint{1, 2}, 7))
	require.Eventually(t, func() bool { return countTerminalEvents(publisher) == 1 }, 2*time.Second, 5*time.Millisecond)

	instanceService.markDeployed(2)
	instanceService.unlock(2)
	require.Eventually(t, func() bool { return !instanceService.isLockedById(2) }, 2*time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, []string{"bulk-1"}, instanceService.deployedStacks())
	assert.Equal(t, 1, countTerminalEvents(publisher))
}
