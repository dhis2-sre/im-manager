package instance

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveParameters(t *testing.T) {
	t.Run("ResolveParameter", func(t *testing.T) {
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
				"PROVIDER-PARAMETER": model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
					return fmt.Sprintf("%s-%s", instance.Name, instance.GroupName), nil
				}),
			},
		}
		stackB := model.Stack{
			Name: "stack-b",
			Parameters: map[string]model.StackParameter{
				"PROVIDER-PARAMETER": {
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
					Name:       "name",
					GroupName:  "group",
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
		assert.Equal(t, "name-group", deployment.Instances[1].Parameters["PROVIDER-PARAMETER"].Value)
	})
}

func TestValidateDeploymentParameters(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		defaultValue := "value"
		stackA := model.Stack{
			Name: "stack",
			Parameters: map[string]model.StackParameter{
				"parameter-a": {
					DefaultValue: &defaultValue,
				},
				"parameter-b": {},
			},
			ParameterProviders: model.ParameterProviders{
				"PROVIDER-PARAMETER": model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
					return fmt.Sprintf("%s-%s", instance.Name, instance.GroupName), nil
				}),
			},
		}
		stackB := model.Stack{
			Name: "stack-b",
			Parameters: map[string]model.StackParameter{
				"PROVIDER-PARAMETER": {
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
					StackName: "stack-a",
					Parameters: map[string]model.DeploymentInstanceParameter{
						"parameter-a": {
							ParameterName: "parameter-a",
						},
						"parameter-b": {
							ParameterName: "parameter-b",
							Value:         "1",
						},
					},
				},
				{
					StackName:  "stack-b",
					Parameters: map[string]model.DeploymentInstanceParameter{},
				},
			},
		}

		err := service.validateDeploymentParameters(deployment)

		require.NoError(t, err)

	})

	t.Run("MissingProvider", func(t *testing.T) {
		stackA := model.Stack{
			Name:       "stack-a",
			Parameters: map[string]model.StackParameter{},
		}
		stackB := model.Stack{
			Name: "stack-b",
			Parameters: map[string]model.StackParameter{
				"PROVIDER-PARAMETER": {
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
					StackName:  "stack-a",
					Parameters: map[string]model.DeploymentInstanceParameter{},
				},
				{
					StackName:  "stack-b",
					Parameters: map[string]model.DeploymentInstanceParameter{},
				},
			},
		}

		err := service.validateDeploymentParameters(deployment)

		require.ErrorContains(t, err, `missing value for parameter: PROVIDER-PARAMETER`)
	})
}

func TestValidateParameters(t *testing.T) {
	defaultPort := "8000"
	stack := &model.Stack{
		Name: "server",
		Parameters: model.StackParameters{
			"HOST": model.StackParameter{
				Validator: func(value string) error {
					if strings.TrimSpace(value) == "" {
						return errors.New("empty hostname")
					}

					return nil
				},
			},
			"PORT": model.StackParameter{
				DefaultValue: &defaultPort,
				Validator: func(value string) error {
					_, err := strconv.Atoi(value)
					if err != nil {
						return errors.New("not a number")
					}

					return nil
				},
			},
		},
	}

	t.Run("Success", func(t *testing.T) {
		instance := &model.Instance{
			StackName: stack.Name,
			Parameters: []model.InstanceParameter{
				{
					Name:  "HOST",
					Value: "myhost",
				},
			},
		}

		err := validateParameters(stack, instance)

		require.NoError(t, err)
	})

	t.Run("FailsIfGivenParameterIsNotInStack", func(t *testing.T) {
		instance := &model.Instance{
			StackName: stack.Name,
			Parameters: []model.InstanceParameter{
				{
					Name:  "HOST",
					Value: "myhost",
				},
				{
					Name:  "ADDITIONAL",
					Value: "some",
				},
			},
		}

		err := validateParameters(stack, instance)

		assert.True(t, errdef.IsBadRequest(err), "should be a bad request error")
		assert.ErrorContains(t, err, `parameter "ADDITIONAL": is not a stack parameter`)
	})

	t.Run("FailsIfGivenRequiredParameterHasNoValue", func(t *testing.T) {
		// this is to show that right now the parameter Validator needs to decide whether an empty
		// string is a valid value for a required parameter because Value is of type string not
		// *string
		instance := &model.Instance{
			StackName: stack.Name,
			Parameters: []model.InstanceParameter{
				{
					Name: "HOST",
				},
			},
		}

		err := validateParameters(stack, instance)

		assert.True(t, errdef.IsBadRequest(err), "should be a bad request error")
		assert.ErrorContains(t, err, "invalid parameter(s)")
		assert.ErrorContains(t, err, `parameter "HOST": empty hostname`)
	})

	t.Run("FailsIfGivenParameterIsNotValid", func(t *testing.T) {
		instance := &model.Instance{
			StackName: stack.Name,
			Parameters: []model.InstanceParameter{
				{
					Name:  "HOST",
					Value: "   ",
				},
				{
					Name:  "PORT",
					Value: "not an integer",
				},
			},
		}

		err := validateParameters(stack, instance)

		assert.True(t, errdef.IsBadRequest(err), "should be a bad request error")
		assert.ErrorContains(t, err, "invalid parameter(s)")
		assert.ErrorContains(t, err, `parameter "HOST": empty hostname`)
		assert.ErrorContains(t, err, `parameter "PORT": not a number`)
	})
}
