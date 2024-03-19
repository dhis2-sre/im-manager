package instance

import (
	"fmt"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveParameters(t *testing.T) {
	t.Run("PreventUserFromOverwritingConsumedParameters", func(t *testing.T) {
		s := model.Stack{
			Name: "stack",
			Parameters: map[string]model.StackParameter{
				"parameter": {
					Consumed: true,
				},
			},
		}
		stacks := stack.Stacks{
			"stack": s,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, stackService, nil)
		instance := &model.DeploymentInstance{
			StackName: "stack",
			Parameters: map[string]model.DeploymentInstanceParameter{
				"parameter": {
					ParameterName: "parameter",
					Value:         "user overwrite",
				},
			},
		}

		err := service.SaveInstance(instance)

		require.ErrorContains(t, err, "consumed parameters can't be supplied by the user: parameter")
	})

	t.Run("RejectNonExistingParameter", func(t *testing.T) {
		s := model.Stack{
			Name:       "name-a",
			Parameters: map[string]model.StackParameter{},
		}
		stacks := stack.Stacks{
			"name-a": s,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, stackService, nil)
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
		stackA := model.Stack{
			Name: "stack-a",
			Parameters: map[string]model.StackParameter{
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
		service := NewService(nil, nil, stackService, nil)
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

	t.Run("ResolveParameterUsingProvider", func(t *testing.T) {
		stackA := model.Stack{
			Name: "stack-a",
			ParameterProviders: model.ParameterProviders{
				"provider-parameter": model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
					return fmt.Sprintf("%s-%s", instance.Name, instance.GroupName), nil
				}),
			},
		}
		stackB := model.Stack{
			Name: "stack-b",
			Parameters: map[string]model.StackParameter{
				"provider-parameter": {
					Consumed: true,
				},
			},
			Requires: []model.Stack{stackA},
		}
		stacks := stack.Stacks{
			"stack-a": stackA,
			"stack-b": stackB,
		}
		stackService := stack.NewService(stacks)
		service := NewService(nil, nil, stackService, nil)
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
