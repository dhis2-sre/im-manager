package instance

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func presetTestDeployment() *model.Deployment {
	return &model.Deployment{Name: "copy", GroupName: "other", Group: &model.Group{Name: "other"}}
}

func TestCopyPresetInstances(t *testing.T) {
	t.Run("CopiesTheInstancesUnderTheNewNameAndGroup", func(t *testing.T) {
		deployment := presetTestDeployment()

		err := editTestService().copyPresetInstances(editTestDeployment(true), deployment, nil)

		require.NoError(t, err)
		assert.Equal(t, []string{"host", "sidecar"}, stackNames(deployment.Instances))
		for _, deploymentInstance := range deployment.Instances {
			assert.Equal(t, "copy", deploymentInstance.Name)
			assert.Equal(t, "other", deploymentInstance.GroupName)
			assert.Zero(t, deploymentInstance.ID)
			assert.Nil(t, deploymentInstance.DeployedAt)
		}
		host := findInstanceByStackName("host", deployment)
		assert.Equal(t, "7", host.Parameters["DATABASE_ID"].Value)
		assert.Equal(t, "secret", host.Parameters["SHARED_PASSWORD"].Value)
	})

	t.Run("LeavesOutConsumedParameters", func(t *testing.T) {
		deployment := presetTestDeployment()

		err := editTestService().copyPresetInstances(editTestDeployment(true), deployment, nil)

		require.NoError(t, err)
		sidecar := findInstanceByStackName("sidecar", deployment)
		assert.NotContains(t, sidecar.Parameters, "SHARED_PASSWORD")
		assert.Equal(t, "1.0.0", sidecar.Parameters["SIDECAR_TAG"].Value)
	})

	t.Run("LeavesOutParametersTheStackNoLongerDeclares", func(t *testing.T) {
		preset := editTestDeployment(false)
		preset.Instances[0].Parameters["RETIRED"] = model.DeploymentInstanceParameter{ParameterName: "RETIRED", Value: "x"}
		deployment := presetTestDeployment()

		err := editTestService().copyPresetInstances(preset, deployment, nil)

		require.NoError(t, err)
		assert.NotContains(t, deployment.Instances[0].Parameters, "RETIRED")
	})

	t.Run("AppliesTheEditsOnTopOfThePreset", func(t *testing.T) {
		deployment := presetTestDeployment()
		edits := map[string]InstanceEdit{
			"host": {Parameters: Parameters{"IMAGE_TAG": {Value: "2.43.0"}}, Public: ptr(true)},
		}

		err := editTestService().copyPresetInstances(editTestDeployment(false), deployment, edits)

		require.NoError(t, err)
		host := findInstanceByStackName("host", deployment)
		assert.Equal(t, "2.43.0", host.Parameters["IMAGE_TAG"].Value)
		assert.Equal(t, "7", host.Parameters["DATABASE_ID"].Value)
		assert.True(t, host.Public)
	})

	t.Run("SwitchingACompanionOnAddsIt", func(t *testing.T) {
		deployment := presetTestDeployment()
		edits := map[string]InstanceEdit{
			"host":    {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "true"}}},
			"sidecar": {Parameters: Parameters{"SIDECAR_TAG": {Value: "2.0.0"}}},
		}

		err := editTestService().copyPresetInstances(editTestDeployment(false), deployment, edits)

		require.NoError(t, err)
		assert.Equal(t, []string{"host", "sidecar"}, stackNames(deployment.Instances))
		sidecar := findInstanceByStackName("sidecar", deployment)
		assert.Equal(t, "copy", sidecar.Name)
		assert.Equal(t, "2.0.0", sidecar.Parameters["SIDECAR_TAG"].Value)
	})

	t.Run("SwitchingACompanionOffLeavesItOut", func(t *testing.T) {
		deployment := presetTestDeployment()
		edits := map[string]InstanceEdit{
			"host": {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "false"}}},
		}

		err := editTestService().copyPresetInstances(editTestDeployment(true), deployment, edits)

		require.NoError(t, err)
		assert.Equal(t, []string{"host"}, stackNames(deployment.Instances))
	})

	t.Run("RefusesAStackThePresetDoesNotOffer", func(t *testing.T) {
		edits := map[string]InstanceEdit{
			"unknown": {Parameters: Parameters{"IMAGE_TAG": {Value: "1"}}},
		}

		err := editTestService().copyPresetInstances(editTestDeployment(false), presetTestDeployment(), edits)

		assert.ErrorContains(t, err, `has no instance of stack "unknown"`)
	})

	t.Run("TheCopyValidatesAsANewDeployment", func(t *testing.T) {
		service := editTestService()
		deployment := presetTestDeployment()
		require.NoError(t, service.copyPresetInstances(editTestDeployment(true), deployment, nil))

		err := service.validateNewDeployment(deployment, map[string]*stack.Stack{})

		require.NoError(t, err)
		sidecar := findInstanceByStackName("sidecar", deployment)
		assert.Equal(t, "secret", sidecar.Parameters["SHARED_PASSWORD"].Value)
	})
}
