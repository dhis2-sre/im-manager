package instance

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
