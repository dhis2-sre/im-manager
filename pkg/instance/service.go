package instance

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/exp/maps"

	v1 "k8s.io/api/core/v1"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dominikbraun/graph"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

func NewService(
	logger *slog.Logger,
	instanceRepository *repository,
	groupService groupService,
	stackService stack.Service,
	helmfileService helmfile,
) *Service {
	return &Service{
		logger:             logger,
		instanceRepository: instanceRepository,
		groupService:       groupService,
		stackService:       stackService,
		helmfileService:    helmfileService,
	}
}

type groupService interface {
	Find(ctx context.Context, name string) (*model.Group, error)
	FindByGroupNames(ctx context.Context, groupNames []string) ([]model.Group, error)
}

type helmfile interface {
	sync(ctx context.Context, token string, instance *model.DeploymentInstance, group *model.Group, ttl uint) (*exec.Cmd, error)
	destroy(ctx context.Context, instance *model.DeploymentInstance, group *model.Group) (*exec.Cmd, error)
}

type Service struct {
	logger             *slog.Logger
	instanceRepository *repository
	groupService       groupService
	stackService       stack.Service
	helmfileService    helmfile
}

func (s Service) SaveDeployment(ctx context.Context, deployment *model.Deployment) error {
	return s.instanceRepository.SaveDeployment(ctx, deployment)
}

func (s Service) FindDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	return s.instanceRepository.FindDeploymentById(ctx, id)
}

func (s Service) FindDecryptedDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	return s.instanceRepository.FindDecryptedDeploymentById(ctx, id)
}

func (s Service) FindDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	return s.instanceRepository.FindDeploymentInstanceById(ctx, id)
}

func (s Service) FindDecryptedDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	return s.instanceRepository.FindDecryptedDeploymentInstanceById(ctx, id)
}

func (s Service) SaveInstance(ctx context.Context, instance *model.DeploymentInstance) error {
	err := s.rejectConsumedParameters(instance)
	if err != nil {
		return err
	}

	deployment, err := s.instanceRepository.FindDecryptedDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		return err
	}

	deployment.Instances = append(deployment.Instances, instance)

	_, err = s.validateNoCycles(deployment.Instances)
	if err != nil {
		return errdef.NewBadRequest("failed to validate instance: %v", err)
	}

	err = s.resolveParameters(deployment)
	if err != nil {
		return errdef.NewBadRequest("failed to resolve parameters: %v", err)
	}

	return s.instanceRepository.SaveInstance(ctx, instance)
}

func (s Service) rejectConsumedParameters(instance *model.DeploymentInstance) error {
	stack, err := s.stackService.Find(instance.StackName)
	if err != nil {
		return err
	}

	var errs []error
	for name := range instance.Parameters {
		if stack.Parameters[name].Consumed {
			errs = append(errs, fmt.Errorf("consumed parameters can't be supplied by the user: %s", name))
		}
	}
	return errors.Join(errs...)
}

func (s Service) DeleteInstance(ctx context.Context, deploymentId, instanceId uint) error {
	deployment, err := s.FindDeploymentById(ctx, deploymentId)
	if err != nil {
		return err
	}

	index := slices.IndexFunc(deployment.Instances, func(instance *model.DeploymentInstance) bool {
		return instanceId == instance.ID
	})
	if index == -1 {
		return errdef.NewNotFound("instance %d not found in deployment %d", instanceId, deployment.ID)
	}
	instance := deployment.Instances[index]

	deployment.Instances = slices.DeleteFunc(deployment.Instances, func(instance *model.DeploymentInstance) bool {
		return instanceId == instance.ID
	})

	_, err = s.validateNoCycles(deployment.Instances)
	if err != nil {
		return errdef.NewBadRequest("failed to delete instance: %v", err)
	}

	err = s.destroyDeploymentInstance(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to destroy instance %d in deployment %d: %v", instanceId, deployment.ID, err)
	}

	return s.instanceRepository.DeleteDeploymentInstance(ctx, instance)
}

func (s Service) validateNoCycles(instances []*model.DeploymentInstance) (graph.Graph[string, *model.DeploymentInstance], error) {
	g := graph.New(func(instance *model.DeploymentInstance) string {
		return instance.StackName
	}, graph.Directed(), graph.PreventCycles())

	for _, instance := range instances {
		err := g.AddVertex(instance)
		if err != nil {
			if errors.Is(err, graph.ErrVertexAlreadyExists) {
				return nil, fmt.Errorf("failed adding instance for stack %q as one already exists", instance.StackName)
			}
			return nil, fmt.Errorf("failed adding instance %q: %v", instance.Name, err)
		}
	}

	for _, src := range instances {
		stack, err := s.stackService.Find(src.StackName)
		if err != nil {
			return nil, err
		}
		for _, requiredStack := range stack.Requires {
			err := g.AddEdge(src.StackName, requiredStack.Name)
			if err != nil {
				if errors.Is(err, graph.ErrEdgeAlreadyExists) {
					return nil, fmt.Errorf("instance %q requires %q more than once", src.Name, requiredStack.Name)
				} else if errors.Is(err, graph.ErrEdgeCreatesCycle) {
					return nil, fmt.Errorf("link from instance %q to stack %q creates a cycle", src.Name, requiredStack.Name)
				} else if errors.Is(err, graph.ErrVertexNotFound) {
					return nil, fmt.Errorf("%q is required by %q", requiredStack.Name, src.StackName)
				}
				return nil, fmt.Errorf("failed linking instance %q with instance %q: %v", src.Name, requiredStack.Name, err)
			}
		}
	}

	return g, nil
}

func (s Service) resolveParameters(deployment *model.Deployment) error {
	for _, instance := range deployment.Instances {
		stack, err := s.stackService.Find(instance.StackName)
		if err != nil {
			return err
		}

		instanceParameters := instance.Parameters
		err = rejectNonExistingParameters(instanceParameters, stack)
		if err != nil {
			return err
		}

		addDefaultParameterValues(instanceParameters, stack)

		err = validateParameters(instanceParameters, stack)
		if err != nil {
			return err
		}

		err = resolveConsumedParameters(deployment, instance, stack)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateParameters(instanceParameters model.DeploymentInstanceParameters, stack *model.Stack) error {
	var errs []error
	for name, parameter := range instanceParameters {
		stackParameter := stack.Parameters[name]
		if stackParameter.Validator != nil {
			err := stackParameter.Validator(parameter.Value)
			if err != nil {
				errs = append(errs, fmt.Errorf("validation failed for parameter %s: %v", name, err))
			}
		}
	}
	return errors.Join(errs...)
}

func resolveConsumedParameters(deployment *model.Deployment, instance *model.DeploymentInstance, stack *model.Stack) error {
	for name, parameter := range instance.Parameters {
		stackParameter := stack.Parameters[name]
		if !stackParameter.Consumed {
			continue
		}

		for _, requiredStack := range stack.Requires {
			// consume from instance parameters
			sourceInstance := findInstanceByStackName(requiredStack.Name, deployment)
			if sourceInstance == nil {
				return errdef.NewNotFound("failed to find required instance %q of instance %q", sourceInstance.Name, instance.Name)
			}

			if sourceInstanceParameter, ok := sourceInstance.Parameters[name]; ok {
				parameter.Value = sourceInstanceParameter.Value
			}

			// consume from provider
			if provider, ok := requiredStack.ParameterProviders[name]; ok {
				value, err := provider.Provide(*sourceInstance)
				if err != nil {
					return fmt.Errorf("failed to provide value for instance %q parameter %q: %v", instance.Name, name, err)
				}
				parameter.Value = value
			}

			instance.Parameters[name] = parameter
		}
	}
	return nil
}

func findInstanceByStackName(name string, deployment *model.Deployment) *model.DeploymentInstance {
	for _, instance := range deployment.Instances {
		if instance.StackName == name {
			return instance
		}
	}
	return nil
}

func rejectNonExistingParameters(instanceParameters model.DeploymentInstanceParameters, stack *model.Stack) error {
	var errs []error
	for name := range instanceParameters {
		if _, ok := stack.Parameters[name]; !ok {
			errs = append(errs, fmt.Errorf("parameter not found on stack: %s", name))
		}
	}
	return errors.Join(errs...)
}

func addDefaultParameterValues(instanceParameters model.DeploymentInstanceParameters, stack *model.Stack) {
	for name, stackParameter := range stack.Parameters {
		if _, ok := instanceParameters[name]; !ok {
			instanceParameter := model.DeploymentInstanceParameter{
				ParameterName: name,
			}

			if stackParameter.DefaultValue != nil {
				instanceParameter.Value = *stackParameter.DefaultValue
			}

			instanceParameters[name] = instanceParameter
		}
	}
}

func (s Service) DeployDeployment(ctx context.Context, token string, deployment *model.Deployment) error {
	deploymentGraph, err := s.validateNoCycles(deployment.Instances)
	if err != nil {
		return err
	}

	instances, err := deploymentOrder(deployment, deploymentGraph)
	if err != nil {
		return err
	}

	deployment.Instances = instances

	for _, instance := range instances {
		err := s.deployDeploymentInstance(ctx, token, instance, deployment.TTL)
		if err != nil {
			return fmt.Errorf("failed to deploy instance(%s) %q: %v", instance.StackName, instance.Name, err)
		}
	}

	return nil
}

func (s Service) deployDeploymentInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint) error {
	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	syncCmd, err := s.helmfileService.sync(ctx, token, instance, group, ttl)
	if err != nil {
		return err
	}

	deployLog, deployErrorLog, err := commandExecutor(syncCmd, group.ClusterConfiguration)
	s.logger.InfoContext(ctx, "Deploy log", "log", string(deployLog), "errorLog", string(deployErrorLog))
	/* TODO: return error log if relevant
	if len(deployErrorLog) > 0 {
		return errors.New(string(deployErrorLog))
	}
	*/
	if err != nil {
		return err
	}

	// TODO: Encrypt before saving? Yes...
	err = s.instanceRepository.SaveDeployLog(ctx, instance, string(deployLog))
	instance.DeployLog = string(deployLog)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed saving deploy log", "error", err)
		return err
	}
	return nil
}

func (s Service) Delete(ctx context.Context, deploymentInstanceId uint) error {
	deploymentInstance, err := s.FindDeploymentInstanceById(ctx, deploymentInstanceId)
	if err != nil {
		return err
	}

	err = s.DeleteInstance(ctx, deploymentInstance.DeploymentID, deploymentInstance.ID)
	if err != nil {
		return err
	}

	deployment, err := s.FindDeploymentById(ctx, deploymentInstance.DeploymentID)
	if err != nil {
		return err
	}

	if len(deployment.Instances) == 0 {
		return s.DeleteDeployment(ctx, deployment)
	}

	return nil
}

func (s Service) DeleteDeployment(ctx context.Context, deployment *model.Deployment) error {
	deploymentGraph, err := s.validateNoCycles(deployment.Instances)
	if err != nil {
		return err
	}

	instances, err := deploymentOrder(deployment, deploymentGraph)
	if err != nil {
		return err
	}
	slices.Reverse(instances)

	var errs error
	for _, instance := range instances {
		err := s.destroyDeploymentInstance(ctx, instance)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to destroy instance(%s) %q: %v", instance.StackName, instance.Name, err))
		}

		err = s.instanceRepository.DeleteDeploymentInstance(ctx, instance)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete instance(%s) %q: %v", instance.StackName, instance.Name, err))
		}
	}
	if errs != nil {
		return errs
	}

	return s.instanceRepository.DeleteDeployment(ctx, deployment)
}

func (s Service) destroyDeploymentInstance(ctx context.Context, instance *model.DeploymentInstance) error {
	if instance.DeployLog == "" {
		return nil
	}

	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	destroyCmd, err := s.helmfileService.destroy(ctx, instance, group)
	if err != nil {
		return err
	}

	destroyLog, destroyErrorLog, err := commandExecutor(destroyCmd, group.ClusterConfiguration)
	s.logger.InfoContext(ctx, "Destroy log", "log", destroyLog, "errorLog", destroyErrorLog)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.deletePersistentVolumeClaim(instance)
}

func deploymentOrder(deployment *model.Deployment, g graph.Graph[string, *model.DeploymentInstance]) ([]*model.DeploymentInstance, error) {
	instances, err := graph.TopologicalSort(g)
	if err != nil {
		return nil, fmt.Errorf("failed to order the deployment: %v", err)
	}

	slices.Reverse(instances)

	orderedInstances := make([]*model.DeploymentInstance, len(instances))
	for i, name := range instances {
		orderedInstances[i] = findInstanceByStackName(name, deployment)
	}

	return orderedInstances, nil
}

func (s Service) Pause(ctx context.Context, instance *model.DeploymentInstance) error {
	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.pause(instance)
}

func (s Service) Resume(ctx context.Context, instance *model.DeploymentInstance) error {
	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	return ks.resume(instance)
}

func (s Service) Restart(ctx context.Context, instance *model.DeploymentInstance, typeSelector string) error {
	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return err
	}

	stack, err := s.stackService.Find(instance.StackName)
	if err != nil {
		return err
	}

	return ks.restart(instance, typeSelector, stack)
}

func (s Service) Logs(instance *model.DeploymentInstance, group *model.Group, typeSelector string) (io.ReadCloser, error) {
	ks, err := NewKubernetesService(group.ClusterConfiguration)
	if err != nil {
		return nil, err
	}

	return ks.getLogs(instance, typeSelector)
}

type GroupWithDeployments struct {
	Name        string              `json:"name"`
	Hostname    string              `json:"hostname"`
	Deployments []*model.Deployment `json:"deployments"`
}

func (s Service) FindDeployments(ctx context.Context, user *model.User) ([]GroupWithDeployments, error) {
	groups := append(user.Groups, user.AdminGroups...) //nolint:gocritic

	groupsByName := make(map[string]model.Group)
	for _, group := range groups {
		groupsByName[group.Name] = group
	}
	groupNames := maps.Keys(groupsByName)

	deployments, err := s.instanceRepository.FindDeployments(ctx, groupNames)
	if err != nil {
		return nil, err
	}

	if len(deployments) < 1 {
		return []GroupWithDeployments{}, nil
	}

	return s.groupDeployments(deployments)
}

func (s Service) groupDeployments(deployments []*model.Deployment) ([]GroupWithDeployments, error) {
	groupsByName := map[string]*model.Group{}
	for _, deployment := range deployments {
		for _, instance := range deployment.Instances {
			groupsByName[instance.GroupName] = deployment.Group
		}
	}

	groupsWithDeployments := make([]GroupWithDeployments, len(groupsByName))
	for i, name := range maps.Keys(groupsByName) {
		groupWithDeployments := groupsWithDeployments[i]
		groupWithDeployments.Name = name
		groupWithDeployments.Hostname = groupsByName[name].Hostname
		for _, deployment := range deployments {
			if name == deployment.GroupName {
				groupWithDeployments.Deployments = append(groupWithDeployments.Deployments, deployment)
			}
		}

		slices.SortFunc(groupWithDeployments.Deployments, func(a, b *model.Deployment) int {
			return cmp.Compare(a.Name, b.Name)
		})

		groupsWithDeployments[i] = groupWithDeployments
	}

	slices.SortFunc(groupsWithDeployments, func(a, b GroupWithDeployments) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return groupsWithDeployments, nil
}

type PublicInstance struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Hostname    string `json:"hostname"`
}

type Category struct {
	Label     string           `json:"label"`
	Instances []PublicInstance `json:"instances"`
}

type GroupWithPublicInstances struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Categories  []Category `json:"categories"`
}

func (s Service) FindPublicInstances(ctx context.Context) ([]GroupWithPublicInstances, error) {
	instances, err := s.instanceRepository.FindPublicInstances(ctx)
	if err != nil {
		return nil, err
	}

	if len(instances) < 1 {
		return []GroupWithPublicInstances{}, nil
	}

	return s.groupPublicInstances(instances)
}

func (s Service) groupPublicInstances(instances []*model.DeploymentInstance) ([]GroupWithPublicInstances, error) {
	groupsByName := map[string]*model.Group{}
	for _, instance := range instances {
		groupsByName[instance.GroupName] = instance.Group
	}

	var groupsWithPublicInstances []GroupWithPublicInstances
	for name, group := range groupsByName {
		groupWithPublicInstances := GroupWithPublicInstances{
			Name:        name,
			Description: group.Description,
			Categories:  nil,
		}
		stableCategory := Category{Label: "Stable"}
		devCategory := Category{Label: "Under Development"}
		nightlyCategory := Category{Label: "Canary"}
		for _, instance := range instances {
			if instance.GroupName == name {
				publicInstance := PublicInstance{
					Name:        instance.Name,
					Description: instance.Deployment.Description,
					Hostname:    fmt.Sprintf("https://%s/%s", instance.Group.Hostname, instance.Name),
				}
				if strings.HasPrefix(instance.Name, "dev") {
					devCategory.Instances = append(devCategory.Instances, publicInstance)
				}
				if strings.HasPrefix(instance.Name, "nightly") {
					nightlyCategory.Instances = append(nightlyCategory.Instances, publicInstance)
				}
				if strings.HasPrefix(instance.Name, "stable") {
					stableCategory.Instances = append(stableCategory.Instances, publicInstance)
				}
			}
		}

		if len(devCategory.Instances) > 0 {
			groupWithPublicInstances.Categories = append(groupWithPublicInstances.Categories, devCategory)
		}

		if len(nightlyCategory.Instances) > 0 {
			groupWithPublicInstances.Categories = append(groupWithPublicInstances.Categories, nightlyCategory)
		}

		if len(stableCategory.Instances) > 0 {
			groupWithPublicInstances.Categories = append(groupWithPublicInstances.Categories, stableCategory)
		}

		if len(groupWithPublicInstances.Categories) > 0 {
			groupsWithPublicInstances = append(groupsWithPublicInstances, groupWithPublicInstances)
		}
	}

	return groupsWithPublicInstances, nil
}

type InstanceStatus string

const (
	NotDeployed        InstanceStatus = "NotDeployed"
	Pending            InstanceStatus = "Pending"
	Booting            InstanceStatus = "Booting"
	BootingWithRestart InstanceStatus = "Booting (%d)"
	Running            InstanceStatus = "Running"
	Error              InstanceStatus = "Error"
)

func (s Service) GetStatus(instance *model.DeploymentInstance) (InstanceStatus, error) {
	ks, err := NewKubernetesService(instance.Group.ClusterConfiguration)
	if err != nil {
		return "", err
	}

	pod, err := ks.getPod(instance.ID, "")
	if err != nil {
		if errdef.IsNotFound(err) {
			return NotDeployed, nil
		}
		return "", err
	}

	switch pod.Status.Phase {
	case v1.PodPending:
		initContainerErrorIndex := slices.IndexFunc(pod.Status.InitContainerStatuses, func(status v1.ContainerStatus) bool {
			return status.State.Waiting != nil && status.State.Waiting.Reason == "ImagePullBackOff"
		})
		if initContainerErrorIndex != -1 {
			status := pod.Status.InitContainerStatuses[initContainerErrorIndex]
			return InstanceStatus(string(Error) + ": " + status.State.Waiting.Message), nil
		}

		containerErrorIndex := slices.IndexFunc(pod.Status.ContainerStatuses, func(status v1.ContainerStatus) bool {
			return status.State.Waiting != nil && status.State.Waiting.Reason == "ImagePullBackOff"
		})
		if containerErrorIndex != -1 {
			status := pod.Status.ContainerStatuses[containerErrorIndex]
			return InstanceStatus(string(Error) + ": " + status.State.Waiting.Message), nil
		}
		return Pending, nil
	case v1.PodFailed:
		return Error, nil
	case v1.PodRunning:
		booting := slices.ContainsFunc(pod.Status.Conditions, func(condition v1.PodCondition) bool {
			return condition.Status == v1.ConditionFalse
		})
		if booting {
			initContainerStatuses := pod.Status.InitContainerStatuses
			if initContainerStatuses != nil {
				initContainerError := slices.ContainsFunc(initContainerStatuses, func(status v1.ContainerStatus) bool {
					return status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.Reason == "Error"
				})
				if initContainerError {
					status := fmt.Sprintf(string(BootingWithRestart), initContainerStatuses[0].RestartCount)
					return InstanceStatus(status), nil
				}
			}

			containerError := slices.ContainsFunc(pod.Status.ContainerStatuses, func(status v1.ContainerStatus) bool {
				return status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.Reason == "Error"
			})
			if containerError {
				status := fmt.Sprintf(string(BootingWithRestart), pod.Status.ContainerStatuses[0].RestartCount)
				return InstanceStatus(status), nil
			}

			return Booting, nil
		}
		return Running, nil
	}
	return "", fmt.Errorf("failed to get instance status")
}

func (s Service) Reset(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint) error {
	err := s.destroyDeploymentInstance(ctx, instance)
	if err != nil {
		return err
	}

	return s.deployDeploymentInstance(ctx, token, instance, ttl)
}
