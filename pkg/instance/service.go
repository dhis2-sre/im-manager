package instance

import (
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/dhis2-sre/im-manager/internal/apperror"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"gorm.io/gorm"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

func NewService(
	config config.Config,
	instanceRepository Repository,
	groupService groupService,
	stackService stack.Service,
	helmfileService helmfile,
) *service {
	return &service{
		config,
		instanceRepository,
		groupService,
		stackService,
		helmfileService,
	}
}

type Repository interface {
	Link(firstInstance, secondInstance *model.Instance) error
	Unlink(instance *model.Instance) error
	Save(instance *model.Instance) error
	FindById(id uint) (*model.Instance, error)
	FindByIdDecrypted(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	FindByGroups(groups []model.Group, presets bool) ([]GroupWithInstances, error)
	SaveDeployLog(instance *model.Instance, log string) error
	Delete(id uint) error
}

type groupService interface {
	Find(name string) (*model.Group, error)
}

type helmfile interface {
	sync(token string, instance *model.Instance, group *model.Group) (*exec.Cmd, error)
	destroy(token string, instance *model.Instance, group *model.Group) (*exec.Cmd, error)
}

type service struct {
	config             config.Config
	instanceRepository Repository
	groupService       groupService
	stackService       stack.Service
	helmfileService    helmfile
}

func (s service) ConsumeParameters(source, destination *model.Instance) error {
	sourceStack, err := s.stackService.Find(source.StackName)
	if err != nil {
		return fmt.Errorf("error finding stack %q of source instance: %w", source.StackName, err)
	}

	destinationStack, err := s.stackService.Find(destination.StackName)
	if err != nil {
		return fmt.Errorf("error finding stack %q of destination instance: %w", destination.StackName, err)
	}

	// Consumed required parameters
	for _, parameter := range destinationStack.RequiredParameters {
		if (parameter.Consumed || source.Preset) && parameter.Name != destinationStack.HostnameVariable {
			value, err := s.findParameterValue(parameter.Name, source, sourceStack)
			if err != nil {
				return err
			}
			parameterRequest := model.InstanceRequiredParameter{
				StackRequiredParameterID: parameter.Name,
				Value:                    value,
			}
			destination.RequiredParameters = append(destination.RequiredParameters, parameterRequest)
		}
	}

	// Consumed optional parameters
	for _, parameter := range destinationStack.OptionalParameters {
		if (parameter.Consumed || source.Preset) && parameter.Name != destinationStack.HostnameVariable {
			value, err := s.findParameterValue(parameter.Name, source, sourceStack)
			if err != nil {
				return err
			}
			parameterRequest := model.InstanceOptionalParameter{
				StackOptionalParameterID: parameter.Name,
				Value:                    value,
			}
			destination.OptionalParameters = append(destination.OptionalParameters, parameterRequest)
		}
	}

	// Hostname parameter
	if !source.Preset && destinationStack.HostnameVariable != "" {
		hostnameParameter := model.InstanceRequiredParameter{
			StackRequiredParameterID: destinationStack.HostnameVariable,
			Value:                    fmt.Sprintf(sourceStack.HostnamePattern, source.Name, source.GroupName),
		}
		destination.RequiredParameters = append(destination.RequiredParameters, hostnameParameter)
	}

	return nil
}

func (s service) findParameterValue(parameter string, sourceInstance *model.Instance, sourceStack *model.Stack) (string, error) {
	requiredParameter, err := sourceInstance.FindRequiredParameter(parameter)
	if err == nil {
		return requiredParameter.Value, nil
	}

	optionalParameter, err := sourceInstance.FindOptionalParameter(parameter)
	if err == nil {
		return optionalParameter.Value, nil
	}

	stackOptionalParameter, err := sourceStack.FindOptionalParameter(parameter)
	if err == nil {
		return stackOptionalParameter.DefaultValue, nil
	}

	return "", fmt.Errorf("unable to find value for parameter: %s", parameter)
}

func (s service) Pause(token string, instance *model.Instance) error {
	group, err := s.groupService.Find(instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.pause(instance)
}

func (s service) Resume(token string, instance *model.Instance) error {
	group, err := s.groupService.Find(instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.resume(instance)
}

func (s service) Restart(token string, instance *model.Instance, typeSelector string) error {
	group, err := s.groupService.Find(instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.restart(instance, typeSelector)
}

func (s service) Link(source, destination *model.Instance) error {
	return s.instanceRepository.Link(source, destination)
}

func (s service) unlink(id uint) error {
	instance := &model.Instance{
		Model: gorm.Model{ID: id},
	}
	return s.instanceRepository.Unlink(instance)
}

func matchRequiredParameters(stackParams []model.StackRequiredParameter, instanceParams []model.InstanceRequiredParameter) []string {
	unmatchedParams := make([]string, 0)
	paramNames := make(map[string]struct{})

	for _, param := range stackParams {
		paramNames[param.Name] = struct{}{}
	}

	for _, param := range instanceParams {
		if _, ok := paramNames[param.StackRequiredParameterID]; !ok {
			unmatchedParams = append(unmatchedParams, param.StackRequiredParameterID)
		}
	}

	return unmatchedParams
}

func matchOptionalParameters(stackParams []model.StackOptionalParameter, instanceParams []model.InstanceOptionalParameter) []string {
	unmatchedParams := make([]string, 0)
	paramNames := make(map[string]struct{})

	for _, param := range stackParams {
		paramNames[param.Name] = struct{}{}
	}

	for _, param := range instanceParams {
		if _, ok := paramNames[param.StackOptionalParameterID]; !ok {
			unmatchedParams = append(unmatchedParams, param.StackOptionalParameterID)
		}
	}

	return unmatchedParams
}

func validateParameters(stack *model.Stack, instance *model.Instance) error {
	unmatchedReqParams := matchRequiredParameters(stack.RequiredParameters, instance.RequiredParameters)
	unmatchedOptParams := matchOptionalParameters(stack.OptionalParameters, instance.OptionalParameters)

	unmatchedParams := make([]string, 0)
	unmatchedParams = append(unmatchedParams, unmatchedReqParams...)
	unmatchedParams = append(unmatchedParams, unmatchedOptParams...)

	if len(unmatchedParams) > 0 {
		return apperror.NewBadRequest(
			fmt.Sprintf("parameters %q are not valid parameters for stack %q", unmatchedParams, instance.StackName),
		)
	}

	return nil
}

func (s service) Save(instance *model.Instance) (*model.Instance, error) {
	instanceStack, err := s.stackService.Find(instance.StackName)
	if err != nil {
		return nil, err
	}

	err = validateParameters(instanceStack, instance)
	if err != nil {
		return nil, err
	}

	err = s.instanceRepository.Save(instance)
	return instance, err
}

func (s service) Deploy(accessToken string, instance *model.Instance) error {
	group, err := s.groupService.Find(instance.GroupName)
	if err != nil {
		return err
	}

	syncCmd, err := s.helmfileService.sync(accessToken, instance, group)
	if err != nil {
		return err
	}

	deployLog, deployErrorLog, err := commandExecutor(syncCmd, group.ClusterConfiguration)
	log.Printf("Deploy log: %s\n", deployLog)
	log.Printf("Deploy error log: %s\n", deployErrorLog)
	/* TODO: return error log if relevant
	if len(deployErrorLog) > 0 {
		return errors.New(string(deployErrorLog))
	}
	*/
	if err != nil {
		return err
	}

	// TODO: Encrypt before saving? Yes...
	err = s.instanceRepository.SaveDeployLog(instance, string(deployLog))
	instance.DeployLog = string(deployLog)
	if err != nil {
		// TODO
		log.Printf("Store error log: %s", deployErrorLog)
		return err
	}

	return nil
}

func (s service) Delete(token string, id uint) error {
	err := s.unlink(id)
	if err != nil {
		return err
	}

	instanceWithParameters, err := s.FindByIdDecrypted(id)
	if err != nil {
		return err
	}

	if instanceWithParameters.DeployLog != "" {
		group, err := s.groupService.Find(instanceWithParameters.GroupName)
		if err != nil {
			return err
		}

		destroyCmd, err := s.helmfileService.destroy(token, instanceWithParameters, group)
		if err != nil {
			return err
		}

		destroyLog, destroyErrorLog, err := commandExecutor(destroyCmd, group.ClusterConfiguration)
		log.Printf("Destroy log: %s\n", destroyLog)
		log.Printf("Destroy error log: %s\n", destroyErrorLog)
		if err != nil {
			return err
		}

		ks, err := NewKubernetesService(group.ClusterConfiguration)
		if err != nil {
			return err
		}

		err = ks.deletePersistentVolumeClaim(instanceWithParameters)
		if err != nil {
			return err
		}
	}

	return s.instanceRepository.Delete(id)
}

func (s service) Logs(instance *model.Instance, group *model.Group, typeSelector string) (io.ReadCloser, error) {
	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return nil, err
	}

	return ks.getLogs(instance, typeSelector)
}

func (s service) FindById(id uint) (*model.Instance, error) {
	return s.instanceRepository.FindById(id)
}

func (s service) FindByIdDecrypted(id uint) (*model.Instance, error) {
	return s.instanceRepository.FindByIdDecrypted(id)
}

func (s service) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	return s.instanceRepository.FindByNameAndGroup(instance, group)
}

func (s service) FindInstances(user *model.User, presets bool) ([]GroupWithInstances, error) {
	allGroups := append(user.Groups, user.AdminGroups...) //nolint:gocritic

	instances, err := s.instanceRepository.FindByGroups(allGroups, presets)
	if err != nil {
		return nil, err
	}

	return instances, err
}
