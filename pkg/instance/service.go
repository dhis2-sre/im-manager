package instance

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"sort"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"gorm.io/gorm"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
)

func NewService(
	config config.Config,
	instanceRepository Repository,
	userClient userClientService,
	stackService stack.Service,
	helmfileService helmfile,
) *service {
	return &service{
		config,
		instanceRepository,
		userClient,
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
	FindByGroupNames(names []string, presets bool) ([]*model.Instance, error)
	SaveDeployLog(instance *model.Instance, log string) error
	Delete(id uint) error
}

type userClientService interface {
	FindGroupByName(token string, name string) (*models.Group, error)
}

type helmfile interface {
	sync(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error)
	destroy(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error)
}

type service struct {
	config             config.Config
	instanceRepository Repository
	userClient         userClientService
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
	group, err := s.userClient.FindGroupByName(token, instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.pause(instance)
}

func (s service) Restart(token string, instance *model.Instance, typeSelector string) error {
	group, err := s.userClient.FindGroupByName(token, instance.GroupName)
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

func (s service) Save(instance *model.Instance) (*model.Instance, error) {
	err := s.instanceRepository.Save(instance)
	return instance, err
}

func (s service) Deploy(accessToken string, instance *model.Instance) error {
	group, err := s.userClient.FindGroupByName(accessToken, instance.GroupName)
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
		group, err := s.userClient.FindGroupByName(token, instanceWithParameters.GroupName)
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
	}

	return s.instanceRepository.Delete(id)
}

func (s service) Logs(instance *model.Instance, group *models.Group, typeSelector string) (io.ReadCloser, error) {
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

type GroupWithInstances struct {
	Name      string
	Hostname  string
	Instances []*model.Instance
}

func (s service) FindInstances(user *models.User, presets bool) ([]GroupWithInstances, error) {
	groups := s.uniqueUserGroups(user)

	groupNames := make([]string, len(groups))
	for i, group := range groups {
		groupNames[i] = group.Name
	}

	instances, err := s.instanceRepository.FindByGroupNames(groupNames, presets)
	if err != nil {
		return nil, err
	}

	return s.groupsWithInstances(instances), err
}

func (s service) uniqueUserGroups(user *models.User) []*models.Group {
	groups := append(user.Groups, user.AdminGroups...)
	return s.removeDuplicates(groups)
}

func (s service) groupsWithInstances(instances []*model.Instance) []GroupWithInstances {
	groups := s.uniqueInstanceGroups(instances)
	groupsWithInstances := make([]GroupWithInstances, len(groups))
	for i, group := range groups {
		groupsWithInstances[i].Name = group.Name
		groupsWithInstances[i].Hostname = group.Hostname
		groupsWithInstances[i].Instances = s.filterByGroupId(instances, func(instance *model.Instance) bool {
			return instance.GroupName == group.Name
		})
	}
	return groupsWithInstances
}

func (s service) uniqueInstanceGroups(instances []*model.Instance) []*models.Group {
	groups := make([]*models.Group, len(instances))
	for i, instance := range instances {
		groups[i] = &models.Group{Name: instance.GroupName}
	}
	return s.removeDuplicates(groups)
}

func (s service) filterByGroupId(instances []*model.Instance, test func(instance *model.Instance) bool) (ret []*model.Instance) {
	for _, instance := range instances {
		if test(instance) {
			ret = append(ret, instance)
		}
	}
	return
}

func (s service) removeDuplicates(groups []*models.Group) []*models.Group {
	if len(groups) <= 1 {
		return groups
	}

	sort.Sort(ByName(groups))

	prev := 1
	for curr := 1; curr < len(groups); curr++ {
		if groups[curr-1].Name != groups[curr].Name {
			groups[prev] = groups[curr]
			prev++
		}
	}

	return groups[:prev]
}

type ByName []*models.Group

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }
