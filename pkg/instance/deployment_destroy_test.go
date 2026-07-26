package instance

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/require"
)

type failingDestroyHelmfile struct {
	failStack string
}

func (failingDestroyHelmfile) sync(context.Context, string, *model.DeploymentInstance, *model.Group, uint, map[string]string) (*exec.Cmd, error) {
	return exec.Command("true"), nil
}

func (f failingDestroyHelmfile) destroy(_ context.Context, instance *model.DeploymentInstance, _ *model.Group) (*exec.Cmd, error) {
	if instance.StackName == f.failStack {
		return nil, errors.New("simulated helmfile destroy failure")
	}
	return exec.Command("true"), nil
}

type stubGroupService struct {
	group *model.Group
}

func (s stubGroupService) Find(context.Context, string) (*model.Group, error) {
	return s.group, nil
}

func (s stubGroupService) FindByGroupNames(context.Context, []string) ([]model.Group, error) {
	return []model.Group{*s.group}, nil
}

// A failed helmfile destroy must not delete the instance's DB record, otherwise the
// leaked release/PVC becomes invisible to us: IM's own bookkeeping says it's gone.
func TestDeleteDeploymentPreservesInstanceRecordWhenDestroyFails(t *testing.T) {
	db := inttest.SetupDB(t)

	group := model.Group{Name: "group-name", Namespace: "group-name", Hostname: "some-host"}
	user := &model.User{Email: "user@dhis2.org", Groups: []model.Group{group}}
	require.NoError(t, db.Create(user).Error)
	group = user.Groups[0]

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	instanceRepo, err := NewRepository(db, "01234567890123456789012345678901")
	require.NoError(t, err)

	deployment := &model.Deployment{
		Name:      "dep",
		GroupName: group.Name,
		UserID:    user.ID,
		Instances: []*model.DeploymentInstance{
			{Name: "instance-a", GroupName: group.Name, StackName: "whoami-go"},
		},
	}
	require.NoError(t, instanceRepo.SaveDeployment(context.Background(), deployment))

	stackService := stack.NewService(stack.Stacks{"whoami-go": stack.WhoamiGo})
	service := NewService(logger, instanceRepo, stubGroupService{group: &group}, stackService, failingDestroyHelmfile{failStack: "whoami-go"}, nil, "")

	err = service.DeleteDeployment(context.Background(), deployment)
	require.Error(t, err)

	_, err = instanceRepo.FindDeploymentInstanceById(context.Background(), deployment.Instances[0].ID)
	require.NoError(t, err, "instance whose destroy failed should keep its DB record")
}

// An instance that has locked a database must still be deletable: deleting it releases the
// lock. Otherwise the locks.instance_id FK blocks the delete and the deployment is stuck.
func TestDeleteDeploymentInstanceReleasesDatabaseLock(t *testing.T) {
	db := inttest.SetupDB(t)

	group := model.Group{Name: "group-name", Namespace: "group-name", Hostname: "some-host"}
	user := &model.User{Email: "user@dhis2.org", Groups: []model.Group{group}}
	require.NoError(t, db.Create(user).Error)
	group = user.Groups[0]

	instanceRepo, err := NewRepository(db, "01234567890123456789012345678901")
	require.NoError(t, err)

	deployment := &model.Deployment{
		Name:      "dep",
		GroupName: group.Name,
		UserID:    user.ID,
		Instances: []*model.DeploymentInstance{
			{Name: "instance-a", GroupName: group.Name, StackName: "whoami-go"},
		},
	}
	require.NoError(t, instanceRepo.SaveDeployment(context.Background(), deployment))
	instance := deployment.Instances[0]

	database := &model.Database{Name: "db", GroupName: group.Name, Slug: "db", UserID: user.ID}
	require.NoError(t, db.Create(database).Error)
	require.NoError(t, db.Create(&model.Lock{DatabaseID: database.ID, InstanceID: instance.ID, UserID: user.ID}).Error)

	err = instanceRepo.DeleteDeploymentInstance(context.Background(), instance)
	require.NoError(t, err)

	var locks int64
	require.NoError(t, db.Model(&model.Lock{}).Where("instance_id = ?", instance.ID).Count(&locks).Error)
	require.Zero(t, locks, "lock held by the deleted instance should be released")

	var remaining int64
	require.NoError(t, db.Model(&model.Database{}).Where("id = ?", database.ID).Count(&remaining).Error)
	require.EqualValues(t, 1, remaining, "releasing the lock must not delete the database itself")
}
