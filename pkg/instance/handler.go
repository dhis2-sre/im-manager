package instance

import (
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	userClient "github.com/dhis2-sre/im-users/pkg/client"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func ProvideHandler(
	//	instanceAuthorizer service.InstanceAuthorizer,
	userClient userClient.Client,
	instanceService Service,
) Handler {
	return Handler{
		//		instanceAuthorizer,
		userClient,
		instanceService,
	}
}

type Handler struct {
	userClient      userClient.Client
	instanceService Service
}

type ParameterRequest struct {
	StackParameterID uint   `json:"stackParameterId" binding:"required"`
	Value            string `json:"value" binding:"required"`
}

type CreateInstanceRequest struct {
	Name               string             `json:"name" binding:"required"`
	GroupID            uint               `json:"groupId" binding:"required"`
	StackID            uint               `json:"stackId" binding:"required"`
	RequiredParameters []ParameterRequest `json:"requiredParameters"`
	OptionalParameters []ParameterRequest `json:"optionalParameters"`
}

// Create godoc
// @Summary Create instance
// @Description Create instance
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{} //model.Instance
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Router /instances [post]
// @Param createInstanceRequest body CreateInstanceRequest true "Create instance request"
// @Security OAuth2Password
func (i Handler) Create(c *gin.Context) {
	var request CreateInstanceRequest

	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	requiredParameters := convertRequiredParameters(&request.RequiredParameters)
	optionalParameters := convertOptionalParameters(&request.OptionalParameters)

	instance := &model.Instance{
		Name:               request.Name,
		UserID:             uint(user.ID),
		GroupID:            request.GroupID,
		StackID:            request.StackID,
		RequiredParameters: *requiredParameters,
		OptionalParameters: *optionalParameters,
	}

	userWithGroups, err := i.userClient.FindUserById(uint(user.ID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	log.Printf("%+v", userWithGroups)

	/* TODO
	canWrite := i.instanceAuthorizer.CanWrite(userWithGroups, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("Write access denied")
		_ = c.Error(unauthorized)
		return
	}
	*/
	if err := i.instanceService.Create(instance); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, instance)
}

func convertRequiredParameters(requestParameters *[]ParameterRequest) *[]model.InstanceRequiredParameter {
	if len(*requestParameters) > 0 {
		var parameters = make([]model.InstanceRequiredParameter, len(*requestParameters))
		for i, parameter := range *requestParameters {
			parameters[i] = model.InstanceRequiredParameter{
				StackRequiredParameterID: parameter.StackParameterID,
				Value:                    parameter.Value,
			}
		}
		return &parameters
	}
	return &[]model.InstanceRequiredParameter{}
}

func convertOptionalParameters(requestParameters *[]ParameterRequest) *[]model.InstanceOptionalParameter {
	if len(*requestParameters) > 0 {
		var parameters = make([]model.InstanceOptionalParameter, len(*requestParameters))
		for i, parameter := range *requestParameters {
			parameters[i] = model.InstanceOptionalParameter{
				StackOptionalParameterID: parameter.StackParameterID,
				Value:                    parameter.Value,
			}
		}
		return &parameters
	}
	return &[]model.InstanceOptionalParameter{}
}
