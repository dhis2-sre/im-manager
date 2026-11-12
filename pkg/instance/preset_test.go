package instance

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func presetTestRequest(edits map[string]InstanceEdit) DeploymentFromPreset {
	return DeploymentFromPreset{UserID: 2, Name: "copy", Group: &model.Group{Name: "other"}, Instances: edits}
}

func TestNewDeploymentFromPreset(t *testing.T) {
	t.Run("CopiesTheInstancesUnderTheNewNameAndGroup", func(t *testing.T) {
		deployment, err := editTestService().newDeploymentFromPreset(editTestDeployment(true), presetTestRequest(nil))

		require.NoError(t, err)
		assert.Equal(t, "copy", deployment.Name)
		assert.Equal(t, "other", deployment.GroupName)
		assert.Equal(t, uint(2), deployment.UserID)
		assert.False(t, deployment.Preset)
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

	t.Run("TakesTheDescriptionAndTTLOfThePresetUnlessGiven", func(t *testing.T) {
		preset := editTestDeployment(false)
		preset.Description = "from the preset"

		kept, err := editTestService().newDeploymentFromPreset(preset, presetTestRequest(nil))
		require.NoError(t, err)
		assert.Equal(t, "from the preset", kept.Description)
		assert.Equal(t, uint(86400), kept.TTL)

		request := presetTestRequest(nil)
		request.Description = ptr("given")
		request.TTL = ptr(uint(3600))
		given, err := editTestService().newDeploymentFromPreset(preset, request)
		require.NoError(t, err)
		assert.Equal(t, "given", given.Description)
		assert.Equal(t, uint(3600), given.TTL)
	})

	t.Run("LeavesOutConsumedParameters", func(t *testing.T) {
		deployment, err := editTestService().newDeploymentFromPreset(editTestDeployment(true), presetTestRequest(nil))

		require.NoError(t, err)
		sidecar := findInstanceByStackName("sidecar", deployment)
		assert.NotContains(t, sidecar.Parameters, "SHARED_PASSWORD")
		assert.Equal(t, "1.0.0", sidecar.Parameters["SIDECAR_TAG"].Value)
	})

	t.Run("LeavesOutParametersTheStackNoLongerDeclares", func(t *testing.T) {
		preset := editTestDeployment(false)
		preset.Instances[0].Parameters["RETIRED"] = model.DeploymentInstanceParameter{ParameterName: "RETIRED", Value: "x"}

		deployment, err := editTestService().newDeploymentFromPreset(preset, presetTestRequest(nil))

		require.NoError(t, err)
		assert.NotContains(t, deployment.Instances[0].Parameters, "RETIRED")
	})

	t.Run("AppliesTheEditsOnTopOfThePreset", func(t *testing.T) {
		edits := map[string]InstanceEdit{
			"host": {Parameters: Parameters{"IMAGE_TAG": {Value: "2.43.0"}}, Public: ptr(true)},
		}

		deployment, err := editTestService().newDeploymentFromPreset(editTestDeployment(false), presetTestRequest(edits))

		require.NoError(t, err)
		host := findInstanceByStackName("host", deployment)
		assert.Equal(t, "2.43.0", host.Parameters["IMAGE_TAG"].Value)
		assert.Equal(t, "7", host.Parameters["DATABASE_ID"].Value)
		assert.True(t, host.Public)
	})

	t.Run("SwitchingACompanionOnAddsIt", func(t *testing.T) {
		edits := map[string]InstanceEdit{
			"host":    {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "true"}}},
			"sidecar": {Parameters: Parameters{"SIDECAR_TAG": {Value: "2.0.0"}}},
		}

		deployment, err := editTestService().newDeploymentFromPreset(editTestDeployment(false), presetTestRequest(edits))

		require.NoError(t, err)
		assert.Equal(t, []string{"host", "sidecar"}, stackNames(deployment.Instances))
		sidecar := findInstanceByStackName("sidecar", deployment)
		assert.Equal(t, "copy", sidecar.Name)
		assert.Equal(t, "2.0.0", sidecar.Parameters["SIDECAR_TAG"].Value)
	})

	t.Run("SwitchingACompanionOffLeavesItOut", func(t *testing.T) {
		edits := map[string]InstanceEdit{
			"host": {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "false"}}},
		}

		deployment, err := editTestService().newDeploymentFromPreset(editTestDeployment(true), presetTestRequest(edits))

		require.NoError(t, err)
		assert.Equal(t, []string{"host"}, stackNames(deployment.Instances))
	})

	t.Run("RefusesAStackThePresetDoesNotOffer", func(t *testing.T) {
		edits := map[string]InstanceEdit{
			"unknown": {Parameters: Parameters{"IMAGE_TAG": {Value: "1"}}},
		}

		_, err := editTestService().newDeploymentFromPreset(editTestDeployment(false), presetTestRequest(edits))

		assert.ErrorContains(t, err, `has no instance of stack "unknown"`)
	})

	t.Run("TheResultValidatesAsANewDeployment", func(t *testing.T) {
		service := editTestService()
		deployment, err := service.newDeploymentFromPreset(editTestDeployment(true), presetTestRequest(nil))
		require.NoError(t, err)

		err = service.validateNewDeployment(deployment, map[string]*stack.Stack{})

		require.NoError(t, err)
		sidecar := findInstanceByStackName("sidecar", deployment)
		assert.Equal(t, "secret", sidecar.Parameters["SHARED_PASSWORD"].Value)
	})
}
