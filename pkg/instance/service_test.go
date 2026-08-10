package instance

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveParameters(t *testing.T) {
	t.Run("PreventUserFromOverwritingConsumedParameters", func(t *testing.T) {
		s := stack.Stack{
			Name: "stack",
			Parameters: map[string]stack.StackParameter{
				"parameter": {
					Consumed: true,
				},
			},
		}
		stacks := stack.Stacks{
			"stack": s,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		instance := &model.DeploymentInstance{
			StackName: "stack",
			Parameters: map[string]model.DeploymentInstanceParameter{
				"parameter": {
					ParameterName: "parameter",
					Value:         "user overwrite",
				},
			},
		}

		err := service.SaveInstance(context.Background(), instance)

		require.ErrorContains(t, err, "consumed parameters can't be supplied by the user: parameter")
	})

	t.Run("RejectNonExistingParameter", func(t *testing.T) {
		s := stack.Stack{
			Name:       "name-a",
			Parameters: map[string]stack.StackParameter{},
		}
		stacks := stack.Stacks{
			"name-a": s,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{
					StackName: "name-a",
					Parameters: map[string]model.DeploymentInstanceParameter{
						"parameter": {
							ParameterName: "parameter",
						},
					},
				},
			},
		}

		err := service.resolveParameters(deployment)

		require.ErrorContains(t, err, "parameter not found on stack: parameter")
	})

	t.Run("ResolveParameters", func(t *testing.T) {
		defaultValue1 := "default value used"
		defaultValue2 := "default value not user"
		stackA := stack.Stack{
			Name: "stack-a",
			Parameters: map[string]stack.StackParameter{
				"parameter-a": {
					DefaultValue: &defaultValue1,
				},
				"parameter-b": {
					DefaultValue: &defaultValue2,
				},
				"parameter-c": {},
			},
		}
		stacks := stack.Stacks{
			"stack-a": stackA,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{
					StackName: "stack-a",
					Parameters: map[string]model.DeploymentInstanceParameter{
						"parameter-b": {
							ParameterName: "parameter-b",
							Value:         "default value overwritten by user",
						},
						"parameter-c": {
							ParameterName: "parameter-c",
							Value:         "some value",
						},
					},
				},
			},
		}

		err := service.resolveParameters(deployment)

		require.NoError(t, err)
		want := []*model.DeploymentInstance{
			{
				StackName: "stack-a",
				Parameters: map[string]model.DeploymentInstanceParameter{
					"parameter-a": {
						ParameterName: "parameter-a",
						Value:         "default value used",
					},
					"parameter-b": {
						ParameterName: "parameter-b",
						Value:         "default value overwritten by user",
					},
					"parameter-c": {
						ParameterName: "parameter-c",
						Value:         "some value",
					},
				},
			},
		}
		assert.ElementsMatch(t, want, deployment.Instances)
	})

	t.Run("ParameterProviderNotInDeployment", func(t *testing.T) {
		stackA := stack.Stack{
			Name: "stack-a",
		}
		stackB := stack.Stack{
			Name: "stack-b",
			Parameters: map[string]stack.StackParameter{
				"parameter": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{stackA},
		}
		stacks := stack.Stacks{
			"stack-a": stackA,
			"stack-b": stackB,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{
					Name:       "name-b",
					StackName:  "stack-b",
					Parameters: map[string]model.DeploymentInstanceParameter{},
				},
			},
		}

		err := service.resolveParameters(deployment)

		require.ErrorContains(t, err, `no instance provides parameter "parameter" consumed by "stack-b"`)
	})

	t.Run("ResolveParameterUsingProvider", func(t *testing.T) {
		stackA := stack.Stack{
			Name: "stack-a",
			ParameterProviders: stack.ParameterProviders{
				"provider-parameter": stack.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
					return fmt.Sprintf("%s-%s", instance.Name, instance.GroupName), nil
				}),
			},
		}
		stackB := stack.Stack{
			Name: "stack-b",
			Parameters: map[string]stack.StackParameter{
				"provider-parameter": {
					Consumed: true,
				},
			},
			Requires: []stack.Stack{stackA},
		}
		stacks := stack.Stacks{
			"stack-a": stackA,
			"stack-b": stackB,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{
					Name:       "name-a",
					GroupName:  "group-a",
					StackName:  "stack-a",
					Parameters: map[string]model.DeploymentInstanceParameter{},
				},
				{
					StackName:  "stack-b",
					Parameters: map[string]model.DeploymentInstanceParameter{},
				},
			},
		}

		err := service.resolveParameters(deployment)

		require.NoError(t, err)
		want := []*model.DeploymentInstance{
			{
				Name:       "name-a",
				GroupName:  "group-a",
				StackName:  "stack-a",
				Parameters: map[string]model.DeploymentInstanceParameter{},
			},
			{
				StackName: "stack-b",
				Parameters: map[string]model.DeploymentInstanceParameter{
					"provider-parameter": {
						ParameterName: "provider-parameter",
						Value:         "name-a-group-a",
					},
				},
			},
		}
		assert.ElementsMatch(t, want, deployment.Instances)
	})
}

func TestProviderBasedRequirements(t *testing.T) {
	consumer := stack.Stack{
		Name: "consumer",
		Parameters: map[string]stack.StackParameter{
			"HOST": {Consumed: true},
		},
	}
	provider := stack.Stack{
		Name: "provider",
		Parameters: map[string]stack.StackParameter{
			"HOST": {},
		},
	}
	otherProvider := stack.Stack{
		Name: "other-provider",
		Parameters: map[string]stack.StackParameter{
			"HOST": {},
		},
	}

	t.Run("ResolvedFromAnyProvidingStack", func(t *testing.T) {
		stackService := stack.NewService(stack.Stacks{"consumer": consumer, "provider": provider})
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{StackName: "provider", Parameters: map[string]model.DeploymentInstanceParameter{"HOST": {ParameterName: "HOST", Value: "db.svc"}}},
				{StackName: "consumer", Parameters: map[string]model.DeploymentInstanceParameter{}},
			},
		}

		_, err := service.validateNoCycles(deployment.Instances)
		require.NoError(t, err)
		err = service.resolveParameters(deployment)
		require.NoError(t, err)
		assert.Equal(t, "db.svc", deployment.Instances[1].Parameters["HOST"].Value)
	})

	t.Run("MissingProvider", func(t *testing.T) {
		stackService := stack.NewService(stack.Stacks{"consumer": consumer})
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		instances := []*model.DeploymentInstance{
			{StackName: "consumer", Parameters: map[string]model.DeploymentInstanceParameter{}},
		}

		_, err := service.validateNoCycles(instances)

		require.ErrorContains(t, err, `no instance provides parameter "HOST" consumed by "consumer"`)
	})

	t.Run("AmbiguousProviders", func(t *testing.T) {
		stackService := stack.NewService(stack.Stacks{"consumer": consumer, "provider": provider, "other-provider": otherProvider})
		service := NewService(nil, nil, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
		instances := []*model.DeploymentInstance{
			{StackName: "provider", Parameters: map[string]model.DeploymentInstanceParameter{}},
			{StackName: "other-provider", Parameters: map[string]model.DeploymentInstanceParameter{}},
			{StackName: "consumer", Parameters: map[string]model.DeploymentInstanceParameter{}},
		}

		_, err := service.validateNoCycles(instances)

		require.ErrorContains(t, err, `provided by both`)
	})

	t.Run("PgAdminComposesWithDhis2V2", func(t *testing.T) {
		stacks, err := stack.New(stack.All...)
		require.NoError(t, err)
		service := NewService(nil, nil, nil, stack.NewService(stacks), nil, nil, "", kube.NewClients(slog.Default()))
		group := &model.Group{Name: "group", Namespace: "namespace"}
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{Name: "core", StackName: "dhis2-v2", GroupName: "group", Group: group, Parameters: map[string]model.DeploymentInstanceParameter{}},
				{Name: "admin", StackName: "pgadmin", GroupName: "group", Group: group, Parameters: map[string]model.DeploymentInstanceParameter{}},
			},
		}

		_, err = service.validateNoCycles(deployment.Instances)
		require.NoError(t, err)
		err = service.resolveParameters(deployment)
		require.NoError(t, err)

		pgadmin := deployment.Instances[1]
		assert.Contains(t, pgadmin.Parameters["DATABASE_HOSTNAME"].Value, "-dhis2-postgresql-rw.namespace.svc")
		assert.NotEmpty(t, pgadmin.Parameters["DATABASE_NAME"].Value)
		assert.NotEmpty(t, pgadmin.Parameters["DATABASE_USERNAME"].Value)
	})

	t.Run("PgAdminComposesWithDhis2DB", func(t *testing.T) {
		stacks, err := stack.New(stack.All...)
		require.NoError(t, err)
		service := NewService(nil, nil, nil, stack.NewService(stacks), nil, nil, "", kube.NewClients(slog.Default()))
		group := &model.Group{Name: "group", Namespace: "namespace"}
		deployment := &model.Deployment{
			Instances: []*model.DeploymentInstance{
				{Name: "db", StackName: "dhis2-db", GroupName: "group", Group: group, Parameters: map[string]model.DeploymentInstanceParameter{}},
				{Name: "admin", StackName: "pgadmin", GroupName: "group", Group: group, Parameters: map[string]model.DeploymentInstanceParameter{}},
			},
		}

		_, err = service.validateNoCycles(deployment.Instances)
		require.NoError(t, err)
		err = service.resolveParameters(deployment)
		require.NoError(t, err)

		pgadmin := deployment.Instances[1]
		assert.NotEmpty(t, pgadmin.Parameters["DATABASE_HOSTNAME"].Value)
	})
}
