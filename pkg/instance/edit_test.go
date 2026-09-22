package instance

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func editTestService() *Service {
	host := stack.Stack{
		Name: "host",
		Parameters: map[string]stack.StackParameter{
			"IMAGE_TAG": {DefaultValue: ptr("2.42.0")},
			// ENABLE_SIDECAR only decides whether the companion belongs to the deployment, like
			// ENABLE_PGADMIN; DEPLOY_EXTRA also reaches the host's own template, like DEPLOY_CHAP.
			"ENABLE_SIDECAR":  {DefaultValue: ptr("false"), NotRendered: true},
			"DEPLOY_EXTRA":    {DefaultValue: ptr("false")},
			"DATABASE_ID":     {ImmutableReason: "it is seeded once"},
			"SHARED_PASSWORD": {DefaultValue: ptr("secret")},
		},
	}
	sidecar := stack.Stack{
		Name: "sidecar",
		Parameters: map[string]stack.StackParameter{
			"SHARED_PASSWORD": {Consumed: true},
			"SIDECAR_TAG":     {DefaultValue: ptr("1.0.0")},
		},
	}
	extra := stack.Stack{
		Name: "extra",
		Parameters: map[string]stack.StackParameter{
			"SHARED_PASSWORD": {Consumed: true},
		},
	}
	host.Companions = []stack.Companion{
		{Stack: sidecar, When: &kube.Condition{Parameter: "ENABLE_SIDECAR", Equals: "true"}},
		{Stack: extra, When: &kube.Condition{Parameter: "DEPLOY_EXTRA", Equals: "true"}},
	}

	stackService := stack.NewService(stack.Stacks{"host": host, "sidecar": sidecar, "extra": extra})
	return NewService(slog.Default(), nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
}

func ptr[T any](v T) *T { return &v }

func editTestDeployment(withSidecar bool) *model.Deployment {
	deployed := time.Now()
	sidecarEnabled := "false"
	if withSidecar {
		sidecarEnabled = "true"
	}

	deployment := &model.Deployment{
		ID: 1, Name: "whoami", GroupName: "group", TTL: 86400,
		Instances: []*model.DeploymentInstance{
			{
				ID: 10, Name: "whoami", GroupName: "group", StackName: "host", DeploymentID: 1, DeployedAt: &deployed,
				Parameters: model.DeploymentInstanceParameters{
					"IMAGE_TAG":       {ParameterName: "IMAGE_TAG", Value: "2.42.0"},
					"ENABLE_SIDECAR":  {ParameterName: "ENABLE_SIDECAR", Value: sidecarEnabled},
					"DATABASE_ID":     {ParameterName: "DATABASE_ID", Value: "7"},
					"SHARED_PASSWORD": {ParameterName: "SHARED_PASSWORD", Value: "secret"},
				},
			},
		},
	}

	if withSidecar {
		deployment.Instances = append(deployment.Instances, &model.DeploymentInstance{
			ID: 11, Name: "whoami", GroupName: "group", StackName: "sidecar", DeploymentID: 1, DeployedAt: &deployed,
			Parameters: model.DeploymentInstanceParameters{
				"SHARED_PASSWORD": {ParameterName: "SHARED_PASSWORD", Value: "secret"},
				"SIDECAR_TAG":     {ParameterName: "SIDECAR_TAG", Value: "1.0.0"},
			},
		})
	}

	return deployment
}

func stackNames(instances []*model.DeploymentInstance) []string {
	names := make([]string, len(instances))
	for i, instance := range instances {
		names[i] = instance.StackName
	}
	return names
}

func TestPlanEdit(t *testing.T) {
	t.Run("AnEmptyEditPlansNothing", func(t *testing.T) {
		changes, err := editTestService().planEdit(editTestDeployment(true), Edit{})

		require.NoError(t, err)
		assert.Empty(t, changes.Redeploy)
		assert.Empty(t, changes.Destroy)
	})

	t.Run("ResubmittingTheStoredValuesPlansNothing", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"IMAGE_TAG": {Value: "2.42.0"}, "DATABASE_ID": {Value: "7"}}},
		}}

		changes, err := editTestService().planEdit(editTestDeployment(true), edit)

		require.NoError(t, err)
		assert.Empty(t, changes.Redeploy)
	})

	t.Run("AParameterChangeRedeploysOnlyThatInstance", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"IMAGE_TAG": {Value: "2.43.0"}}},
		}}

		changes, err := editTestService().planEdit(editTestDeployment(true), edit)

		require.NoError(t, err)
		assert.Equal(t, []string{"host"}, stackNames(changes.Redeploy))
		assert.Empty(t, changes.Destroy)
	})

	t.Run("AChangeToAConsumedValueRedeploysTheConsumerToo", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"SHARED_PASSWORD": {Value: "rotated"}}},
		}}

		changes, err := editTestService().planEdit(editTestDeployment(true), edit)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"host", "sidecar"}, stackNames(changes.Redeploy))
	})

	t.Run("ATTLChangeRedeploysNothing", func(t *testing.T) {
		deployment := editTestDeployment(true)

		changes, err := editTestService().planEdit(deployment, Edit{TTL: ptr(uint(172800)), Description: ptr("later")})

		require.NoError(t, err)
		assert.Empty(t, changes.Redeploy)
		assert.Empty(t, changes.Destroy)
		assert.Equal(t, uint(172800), deployment.TTL)
		assert.Equal(t, "later", deployment.Description)
	})

	t.Run("APublicChangeRedeploysNothing", func(t *testing.T) {
		deployment := editTestDeployment(true)
		edit := Edit{Instances: map[string]InstanceEdit{"host": {Public: ptr(true)}}}

		changes, err := editTestService().planEdit(deployment, edit)

		require.NoError(t, err)
		assert.Empty(t, changes.Redeploy)
		assert.True(t, deployment.Instances[0].Public)
	})

	t.Run("AGatingParameterTheHostDoesNotRenderAddsTheCompanionWithoutRollingTheHost", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host":    {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "true"}}},
			"sidecar": {Parameters: Parameters{"SIDECAR_TAG": {Value: "2.0.0"}}},
		}}

		changes, err := editTestService().planEdit(editTestDeployment(false), edit)

		require.NoError(t, err)
		assert.Equal(t, []string{"sidecar"}, stackNames(changes.Redeploy))
		assert.Empty(t, changes.Destroy)

		added := changes.Redeploy[0]
		assert.Equal(t, "2.0.0", added.Parameters["SIDECAR_TAG"].Value)
		assert.Equal(t, "secret", added.Parameters["SHARED_PASSWORD"].Value, "a consumed parameter is resolved from its provider")
	})

	t.Run("AGatingParameterTheHostRendersRollsTheHostToo", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"DEPLOY_EXTRA": {Value: "true"}}},
		}}

		changes, err := editTestService().planEdit(editTestDeployment(false), edit)

		require.NoError(t, err)
		assert.Equal(t, []string{"host", "extra"}, stackNames(changes.Redeploy))
	})

	t.Run("AnAddedCompanionFallsBackToTheStackDefaults", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "true"}}},
		}}

		changes, err := editTestService().planEdit(editTestDeployment(false), edit)

		require.NoError(t, err)
		assert.Equal(t, "1.0.0", changes.Redeploy[0].Parameters["SIDECAR_TAG"].Value)
	})

	t.Run("TheGatingParameterGoingFalseDestroysTheCompanion", func(t *testing.T) {
		deployment := editTestDeployment(true)
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"ENABLE_SIDECAR": {Value: "false"}}},
		}}

		changes, err := editTestService().planEdit(deployment, edit)

		require.NoError(t, err)
		assert.Equal(t, []string{"sidecar"}, stackNames(changes.Destroy))
		assert.Empty(t, changes.Redeploy, "the host does not render the switch, so nothing is redeployed")
		assert.Equal(t, []string{"host"}, stackNames(deployment.Instances), "the answer leaves out the instance on its way out")
	})

	t.Run("AnImmutableParameterIsRejected", func(t *testing.T) {
		deployment := editTestDeployment(true)
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"DATABASE_ID": {Value: "8"}, "IMAGE_TAG": {Value: "2.43.0"}}},
		}}

		_, err := editTestService().planEdit(deployment, edit)

		require.ErrorContains(t, err, "DATABASE_ID can't be changed once the instance has been deployed")
		assert.True(t, errdef.IsBadRequest(err))
		assert.Equal(t, "2.42.0", deployment.Instances[0].Parameters["IMAGE_TAG"].Value, "a rejected edit changes nothing")
	})

	t.Run("AStackTheDeploymentNeitherHasNorOffersIsRejected", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{"whoami-go": {Parameters: Parameters{"IMAGE_TAG": {Value: "1.0.0"}}}}}

		_, err := editTestService().planEdit(editTestDeployment(true), edit)

		require.ErrorContains(t, err, `has no instance of stack "whoami-go"`)
		assert.True(t, errdef.IsBadRequest(err))
	})

	t.Run("AConsumedParameterCannotBeSupplied", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"sidecar": {Parameters: Parameters{"SHARED_PASSWORD": {Value: "mine"}}},
		}}

		_, err := editTestService().planEdit(editTestDeployment(true), edit)

		require.ErrorContains(t, err, "consumed parameters can't be supplied by the user: SHARED_PASSWORD")
		assert.True(t, errdef.IsBadRequest(err))
	})

	t.Run("AnUnknownParameterIsRejected", func(t *testing.T) {
		edit := Edit{Instances: map[string]InstanceEdit{
			"host": {Parameters: Parameters{"NOT_A_PARAMETER": {Value: "x"}}},
		}}

		_, err := editTestService().planEdit(editTestDeployment(true), edit)

		require.ErrorContains(t, err, "parameter not found on stack: NOT_A_PARAMETER")
		assert.True(t, errdef.IsBadRequest(err))
	})
}
