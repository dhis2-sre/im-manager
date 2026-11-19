package instance

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentComponentsReportsAnUnreachableCluster(t *testing.T) {
	t.Setenv("IN_CLUSTER", "false")

	service := Service{groupService: stubGroupService{group: &model.Group{Name: "group-name"}}}
	deployment := &model.Deployment{
		Instances: []*model.DeploymentInstance{
			{ID: 1, Name: "instance-name", GroupName: "group-name", StackName: "whoami-go"},
		},
	}

	components, err := service.DeploymentComponents(t.Context(), deployment)
	require.NoError(t, err)

	assert.Equal(t, []InstanceComponents{
		{
			InstanceID:               1,
			InstanceName:             "instance-name",
			StackName:                "whoami-go",
			Components:               []ComponentStatus{},
			ClusterUnreachableReason: "no cluster is assigned, and instance manager is not set up to deploy to the cluster it is running in",
		},
	}, components)
}
