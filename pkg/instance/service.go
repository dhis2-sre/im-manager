package instance

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/dominikbraun/graph"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(
	config config.Config,
	instanceRepository Repository,
	groupService groupService,
	stackService stack.Service,
	helmfileService helmfile,
) *service {
	return &service{config, instanceRepository, groupService, stackService, helmfileService}
}

type Repository interface {
	SaveDeployment(chain *model.Deployment) error
	FindDeploymentById(id uint) (*model.Deployment, error)
	SaveInstance(*model.DeploymentInstance) error
	Link(firstInstance, secondInstance *model.Instance) error
	Unlink(instance *model.Instance) error
	Save(instance *model.Instance) error
	FindById(id uint) (*model.Instance, error)
	FindByIdDecrypted(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	FindByGroups(groups []model.Group, presets bool) ([]GroupsWithInstances, error)
	FindPublicInstances() ([]GroupsWithInstances, error)
	SaveDeployLog(instance *model.Instance, log string) error
	Delete(id uint) error
}

type groupService interface {
	Find(name string) (*model.Group, error)
	FindAll(user *model.User, deployable bool) ([]model.Group, error)
}

type helmfile interface {
	sync(token string, instance *model.Instance, group *model.Group) (*exec.Cmd, error)
	destroy(instance *model.Instance, group *model.Group) (*exec.Cmd, error)
}

type service struct {
	config             config.Config
	instanceRepository Repository
	groupService       groupService
	stackService       stack.Service
	helmfileService    helmfile
}

func (s service) SaveDeployment(chain *model.Deployment) error {
	return s.instanceRepository.SaveDeployment(chain)
}

func (s service) FindDeploymentById(id uint) (*model.Deployment, error) {
	return s.instanceRepository.FindDeploymentById(id)
}

func (s service) SaveInstance(instance *model.DeploymentInstance) error {
	deployment, err := s.instanceRepository.FindDeploymentById(instance.DeploymentID)
	if err != nil {
		return err
	}

	deployment.Instances = append(deployment.Instances, instance)
	err = s.validateDeployment(deployment)
	if err != nil {
		return err
	}

	return s.instanceRepository.SaveInstance(instance)
}

func (s service) validateDeployment(deployment *model.Deployment) error {
	err := s.validateNoCycles(deployment)
	if err != nil {
		return err
	}

	return nil
}

func (s service) validateNoCycles(deployment *model.Deployment) error {
	g := graph.New(func(instance *model.DeploymentInstance) string {
		return instance.StackName
	}, graph.Directed(), graph.PreventCycles())

	instances := deployment.Instances
	for _, instance := range instances {
		err := g.AddVertex(instance)
		if err != nil {
			return fmt.Errorf("failed adding vertex for instance with stack %q: %v", instance.StackName, err)
		}
	}

	for _, instance := range instances {
		src, err := s.stackService.Find(instance.StackName)
		if err != nil {
			return err
		}

		for _, dest := range src.Requires {
			err := g.AddEdge(src.Name, dest.Name)
			if err != nil {
				if errors.Is(err, graph.ErrEdgeAlreadyExists) {
					return fmt.Errorf("instance %q requires %q more than once", src.Name, dest.Name)
				} else if errors.Is(err, graph.ErrEdgeCreatesCycle) {
					return fmt.Errorf("edge from instance %q to instance %q creates a cycle", src.Name, dest.Name)
				}
				return fmt.Errorf("failed adding edge from instance %q to instance %q: %v", src.Name, dest.Name, err)
			}
		}
	}

	return nil
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

	// Consumed parameters
	for name, parameter := range destinationStack.Parameters {
		if (parameter.Consumed || source.Preset) && name != destinationStack.HostnameVariable {
			value, err := s.findParameterValue(name, source, sourceStack)
			if err != nil {
				return err
			}
			parameterRequest := model.InstanceParameter{
				Name:  name,
				Value: value,
			}
			destination.Parameters = append(destination.Parameters, parameterRequest)
		}
	}

	// Hostname parameter
	if !source.Preset && destinationStack.HostnameVariable != "" {
		hostnameParameter := model.InstanceParameter{
			Name:  destinationStack.HostnameVariable,
			Value: fmt.Sprintf(sourceStack.HostnamePattern, source.Name, source.GroupName),
		}
		destination.Parameters = append(destination.Parameters, hostnameParameter)
	}

	return nil
}

func (s service) findParameterValue(parameter string, sourceInstance *model.Instance, sourceStack *model.Stack) (string, error) {
	instanceParameter, err := sourceInstance.FindParameter(parameter)
	if err == nil {
		return instanceParameter.Value, nil
	}

	stackParameter, ok := sourceStack.Parameters[parameter]
	if !ok {
		return "", fmt.Errorf("unable to find value for parameter: %s", parameter)
	}

	return *stackParameter.DefaultValue, nil
}

func (s service) Pause(instance *model.Instance) error {
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

func (s service) Resume(instance *model.Instance) error {
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

func (s service) Restart(instance *model.Instance, typeSelector string) error {
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
		ID: id,
	}
	return s.instanceRepository.Unlink(instance)
}

func (s service) Save(instance *model.Instance) error {
	instanceStack, err := s.stackService.Find(instance.StackName)
	if err != nil {
		return err
	}

	err = validateParameters(instanceStack, instance)
	if err != nil {
		return err
	}

	return s.instanceRepository.Save(instance)
}

func validateParameters(stack *model.Stack, instance *model.Instance) error {
	var errs []error
	for _, parameter := range instance.Parameters {
		stackParameter, ok := stack.Parameters[parameter.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("parameter %q: is not a stack parameter", parameter.Name))
			continue
		}

		if stackParameter.Validator == nil {
			continue
		}
		err := stackParameter.Validator(parameter.Value)
		if err != nil {
			errs = append(errs, fmt.Errorf("parameter %q: %v", parameter.Name, err))
		}
	}

	if errs != nil {
		return errdef.NewBadRequest("invalid parameter(s): %v", errors.Join(errs...))
	}

	return nil
}

func (s service) Deploy(token string, instance *model.Instance) error {
	group, err := s.groupService.Find(instance.GroupName)
	if err != nil {
		return err
	}

	syncCmd, err := s.helmfileService.sync(token, instance, group)
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

func (s service) Delete(id uint) error {
	err := s.unlink(id)
	if err != nil {
		return err
	}

	instance, err := s.FindByIdDecrypted(id)
	if err != nil {
		return err
	}

	err = s.destroy(instance)
	if err != nil {
		return err
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

func (s service) FindInstances(user *model.User, presets bool) ([]GroupsWithInstances, error) {
	groups := append(user.Groups, user.AdminGroups...) //nolint:gocritic

	instances, err := s.instanceRepository.FindByGroups(groups, presets)
	if err != nil {
		return nil, err
	}

	return instances, err
}

func (s service) FindPublicInstances() ([]GroupsWithInstances, error) {
	return s.instanceRepository.FindPublicInstances()
}

func (s service) Reset(token string, instance *model.Instance) error {
	err := s.destroy(instance)
	if err != nil {
		return err
	}

	return s.Deploy(token, instance)
}

func (s service) destroy(instance *model.Instance) error {
	if instance.DeployLog != "" {
		group, err := s.groupService.Find(instance.GroupName)
		if err != nil {
			return err
		}

		destroyCmd, err := s.helmfileService.destroy(instance, group)
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

		err = ks.deletePersistentVolumeClaim(instance)
		if err != nil {
			return err
		}
	}
	return nil
}
