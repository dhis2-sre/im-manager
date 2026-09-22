package instance

import (
	"context"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDeployment(t *testing.T) (*gorm.DB, *repository, *model.Deployment) {
	t.Helper()

	db := inttest.SetupDB(t)
	group := model.Group{Name: "group-name", Namespace: "group-name", Hostname: "some-host"}
	user := &model.User{Email: "deploy-lock@dhis2.org", Groups: []model.Group{group}}
	require.NoError(t, db.Create(user).Error)

	instanceRepo, err := NewRepository(db, "01234567890123456789012345678901")
	require.NoError(t, err)

	deployment := &model.Deployment{
		Name:      "dep",
		GroupName: user.Groups[0].Name,
		UserID:    user.ID,
		Instances: []*model.DeploymentInstance{
			{Name: "instance-a", GroupName: user.Groups[0].Name, StackName: "whoami-go"},
		},
	}
	require.NoError(t, instanceRepo.SaveDeployment(context.Background(), deployment))

	return db, instanceRepo, deployment
}

func TestDeployLockAdmitsOneHolder(t *testing.T) {
	ctx := context.Background()
	_, instanceRepo, deployment := setupDeployment(t)

	acquired, err := instanceRepo.AcquireDeployLock(ctx, deployment.ID, time.Hour)
	require.NoError(t, err)
	assert.True(t, acquired)

	acquired, err = instanceRepo.AcquireDeployLock(ctx, deployment.ID, time.Hour)
	require.NoError(t, err)
	assert.False(t, acquired, "a second deploy must be refused while the first holds the lock")

	require.NoError(t, instanceRepo.ReleaseDeployLock(ctx, deployment.ID))

	acquired, err = instanceRepo.AcquireDeployLock(ctx, deployment.ID, time.Hour)
	require.NoError(t, err)
	assert.True(t, acquired, "the lock must be available again once released")
}

// A lock nobody can still be holding is taken over, otherwise a deployment whose deploy died
// without releasing it could never be deployed again.
func TestDeployLockIsTakenOverOnceStale(t *testing.T) {
	ctx := context.Background()
	_, instanceRepo, deployment := setupDeployment(t)

	acquired, err := instanceRepo.AcquireDeployLock(ctx, deployment.ID, time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, err = instanceRepo.AcquireDeployLock(ctx, deployment.ID, 0)
	require.NoError(t, err)
	assert.True(t, acquired)
}

func TestAbandonDeploysInProgressFailsThemAndClearsLocks(t *testing.T) {
	ctx := context.Background()
	db, instanceRepo, deployment := setupDeployment(t)

	acquired, err := instanceRepo.AcquireDeployLock(ctx, deployment.ID, time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	deploymentInstance := deployment.Instances[0]
	require.NoError(t, instanceRepo.SaveDeployState(ctx, deploymentInstance, model.DeployStatusDeploying, ""))

	abandoned, err := instanceRepo.AbandonDeploysInProgress(ctx, "interrupted")
	require.NoError(t, err)
	assert.EqualValues(t, 1, abandoned)

	var reloaded model.DeploymentInstance
	require.NoError(t, db.First(&reloaded, deploymentInstance.ID).Error)
	assert.Equal(t, model.DeployStatusFailed, reloaded.DeployStatus)
	assert.Equal(t, "interrupted", reloaded.DeployError)

	acquired, err = instanceRepo.AcquireDeployLock(ctx, deployment.ID, time.Hour)
	require.NoError(t, err)
	assert.True(t, acquired, "a restart must leave no lock behind")
}

// A deploy that succeeds is the only one that gets a timestamp, so "when was this last deployed"
// cannot be answered with the moment it last failed.
func TestSaveDeployStateTimestampsOnlyASuccessfulDeploy(t *testing.T) {
	ctx := context.Background()
	db, instanceRepo, deployment := setupDeployment(t)
	deploymentInstance := deployment.Instances[0]

	require.NoError(t, instanceRepo.SaveDeployState(ctx, deploymentInstance, model.DeployStatusFailed, "helmfile exploded"))

	var reloaded model.DeploymentInstance
	require.NoError(t, db.First(&reloaded, deploymentInstance.ID).Error)
	assert.Equal(t, model.DeployStatusFailed, reloaded.DeployStatus)
	assert.Nil(t, reloaded.DeployedAt)

	require.NoError(t, instanceRepo.SaveDeployState(ctx, deploymentInstance, model.DeployStatusDeployed, ""))

	require.NoError(t, db.First(&reloaded, deploymentInstance.ID).Error)
	assert.Equal(t, model.DeployStatusDeployed, reloaded.DeployStatus)
	assert.Empty(t, reloaded.DeployError)
	require.NotNil(t, reloaded.DeployedAt)
}
