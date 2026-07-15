package instance

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/storage"
	"golang.org/x/sync/errgroup"

	v1 "k8s.io/api/core/v1"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dominikbraun/graph"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

func NewService(logger *slog.Logger, instanceRepository *repository, groupService groupService, stackService stack.Service, helmfileService helmfile, s3Client *storage.S3Client, s3Bucket string) *Service {
	return &Service{
		logger:             logger,
		instanceRepository: instanceRepository,
		groupService:       groupService,
		stackService:       stackService,
		helmfileService:    helmfileService,
		s3Client:           s3Client,
		s3Bucket:           s3Bucket,
	}
}

type groupService interface {
	Find(ctx context.Context, name string) (*model.Group, error)
	FindByGroupNames(ctx context.Context, groupNames []string) ([]model.Group, error)
}

type helmfile interface {
	sync(ctx context.Context, token string, instance *model.DeploymentInstance, group *model.Group, ttl uint, extraEnv map[string]string) (*exec.Cmd, error)
	destroy(ctx context.Context, instance *model.DeploymentInstance, group *model.Group) (*exec.Cmd, error)
}

type Service struct {
	logger             *slog.Logger
	instanceRepository *repository
	groupService       groupService
	stackService       stack.Service
	helmfileService    helmfile
	s3Client           *storage.S3Client
	s3Bucket           string
}

// restoreFilestoreToS3 restores the given filestore backup into the instance's external
// S3 bucket, since external S3 has no pod to seed the way minio/filesystem do.
func (s Service) restoreFilestoreToS3(ctx context.Context, core *model.DeploymentInstance, filestore *model.Database) error {
	key := strings.TrimPrefix(filestore.Url, fmt.Sprintf("s3://%s/", s.s3Bucket))

	client, err := newExternalS3Client(core)
	if err != nil {
		return err
	}

	bucket := core.Parameters["S3_BUCKET"].Value
	if err := ensureBucket(ctx, client, bucket, core.Parameters["S3_REGION"].Value); err != nil {
		return err
	}

	restored, err := filestoreRestored(ctx, client, bucket)
	if err != nil {
		return err
	}
	if restored {
		s.logger.InfoContext(ctx, "Filestore already restored to external S3, skipping", "bucket", bucket)
		return nil
	}

	pr, pw := io.Pipe()
	// Use a dedicated context for the streaming errgroup; it is cancelled once
	// g.Wait returns, so the marker write below must use the outer detached ctx.
	g, streamCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := s.s3Client.Download(streamCtx, s.s3Bucket, key, pw, func(int64) {})
		pw.CloseWithError(err)
		return err
	})
	g.Go(func() error {
		err := restoreTarGzToBucket(streamCtx, client, bucket, pr)
		pr.CloseWithError(err)
		return err
	})
	if err := g.Wait(); err != nil {
		return fmt.Errorf("filestore restore failed: %v", err)
	}

	if err := markFilestoreRestored(ctx, client, bucket); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "Filestore restored to external S3", "bucket", bucket, "key", key)
	return nil
}

func (s Service) SaveDeployment(ctx context.Context, deployment *model.Deployment) error {
	return s.instanceRepository.SaveDeployment(ctx, deployment)
}

func (s Service) FindDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	return s.instanceRepository.FindDeploymentById(ctx, id)
}

func (s Service) FindDecryptedDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	deployment, err := s.instanceRepository.FindDeploymentById(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.decryptDeployment(deployment)
}

func (s Service) decryptDeployment(deployment *model.Deployment) (*model.Deployment, error) {
	var stacksByName = map[string]*model.Stack{}
	for _, instance := range deployment.Instances {
		stack, err := s.stackService.Find(instance.StackName)
		if err != nil {
			return nil, err
		}
		stacksByName[instance.StackName] = stack
	}

	return s.instanceRepository.DecryptDeployment(deployment, stacksByName)
}

func (s Service) FindDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	return s.instanceRepository.FindDeploymentInstanceById(ctx, id)
}

func (s Service) FindDecryptedDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	deploymentInstance, err := s.instanceRepository.FindDeploymentInstanceById(ctx, id)
	if err != nil {
		return nil, err
	}
	stack, err := s.stackService.Find(deploymentInstance.StackName)
	if err != nil {
		return nil, err
	}
	return s.instanceRepository.DecryptDeploymentInstance(deploymentInstance, stack)
}

func (s Service) SaveInstance(ctx context.Context, instance *model.DeploymentInstance) error {
	err := s.rejectConsumedParameters(instance.StackName, maps.Keys(instance.Parameters))
	if err != nil {
		return err
	}

	deployment, err := s.FindDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		return err
	}

	decryptedDeployment, err := s.decryptDeployment(deployment)
	if err != nil {
		return err
	}

	decryptedDeployment.Instances = append(decryptedDeployment.Instances, instance)

	_, err = s.validateNoCycles(decryptedDeployment.Instances)
	if err != nil {
		return errdef.NewBadRequest("failed to validate instance: %v", err)
	}

	err = s.resolveParameters(decryptedDeployment)
	if err != nil {
		return errdef.NewBadRequest("failed to resolve parameters: %v", err)
	}

	stack, err := s.stackService.Find(instance.StackName)
	if err != nil {
		return err
	}

	return s.instanceRepository.SaveInstance(ctx, instance, stack)
}

func (s Service) rejectConsumedParameters(stackName string, paramNames iter.Seq[string]) error {
	stack, err := s.stackService.Find(stackName)
	if err != nil {
		return err
	}

	var errs []error
	for name := range paramNames {
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

	err = s.DestroyInstance(ctx, instance)
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

		for _, instanceParameter := range src.Parameters {
			stackParameter := stack.Parameters[instanceParameter.ParameterName]
			if stackParameter.RequireCompanion != nil {
				companion, err := stackParameter.RequireCompanion.Require(instanceParameter)
				if err != nil {
					return nil, fmt.Errorf("failed to check companion for parameter %q: %v", instanceParameter.ParameterName, err)
				}
				if companion != nil {
					companionStackName := companion.Name
					err := g.AddEdge(src.StackName, companionStackName)
					// TODO: Fix error messages so they're unique and not the same as for required stacks
					if err != nil {
						if errors.Is(err, graph.ErrEdgeAlreadyExists) {
							return nil, fmt.Errorf("instance %q requires %q more than once", src.Name, companionStackName)
						} else if errors.Is(err, graph.ErrEdgeCreatesCycle) {
							return nil, fmt.Errorf("link from instance %q to stack %q creates a cycle", src.Name, companionStackName)
						} else if errors.Is(err, graph.ErrVertexNotFound) {
							return nil, fmt.Errorf("%q is required by %q", companionStackName, src.StackName)
						}
						return nil, fmt.Errorf("failed linking instance %q with instance %q: %v", src.Name, companionStackName, err)
					}
				}
			}
		}

		for _, requiredStack := range stack.Requires {
			requiredStackName := requiredStack.Name
			err := g.AddEdge(src.StackName, requiredStackName)
			if err != nil {
				if errors.Is(err, graph.ErrEdgeAlreadyExists) {
					return nil, fmt.Errorf("instance %q requires %q more than once", src.Name, requiredStackName)
				} else if errors.Is(err, graph.ErrEdgeCreatesCycle) {
					return nil, fmt.Errorf("link from instance %q to stack %q creates a cycle", src.Name, requiredStackName)
				} else if errors.Is(err, graph.ErrVertexNotFound) {
					return nil, fmt.Errorf("%q is required by %q", requiredStackName, src.StackName)
				}
				return nil, fmt.Errorf("failed linking instance %q with instance %q: %v", src.Name, requiredStackName, err)
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
				sourceInstance.Group = instance.Group
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

// DeploymentOrder validates the deployment's instance graph and returns its instances in
// topological deploy order.
func (s Service) DeploymentOrder(deployment *model.Deployment) ([]*model.DeploymentInstance, error) {
	deploymentGraph, err := s.validateNoCycles(deployment.Instances)
	if err != nil {
		return nil, err
	}

	return deploymentOrder(deployment, deploymentGraph)
}

func (s Service) DeployInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint, extraEnv map[string]string, filestoreBackup *model.Database) error {
	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	if storageType(instance) == "s3" && filestoreBackup != nil {
		// A filestore restore can push the deploy past the client/load-balancer
		// timeout. Detach from the request context so the restore and the
		// subsequent ingress-class/cert-issuer discovery still run instead of
		// failing on a cancelled context, which would drop TLS from the ingress.
		// Only the s3 path needs this; other storage types deploy well within the
		// timeout and keep the request context so client cancellation still works.
		ctx = context.WithoutCancel(ctx)
		if err := s.restoreFilestoreToS3(ctx, instance, filestoreBackup); err != nil {
			return err
		}
	}

	syncCmd, err := s.helmfileService.sync(ctx, token, instance, group, ttl, extraEnv)
	if err != nil {
		return err
	}

	deployLog, deployErrorLog, err := commandExecutor(syncCmd, group.Cluster)
	// In recent versions of helmfile most of the command output is sent to stderr https://github.com/roboll/helmfile/pull/583
	s.logger.InfoContext(ctx, "Deploy log", "log", string(deployLog), "errorLog", string(deployErrorLog))
	/* TODO: return error log if relevant
	if len(deployErrorLog) > 0 {
		return errors.New(string(deployErrorLog))
	}
	*/
	if err != nil {
		// TODO: This is a hack to detect if the helmfile operation is already in progress.
		if strings.Contains(string(deployErrorLog), "another operation (install/upgrade/rollback) is in progress") {
			s.logger.WarnContext(ctx, "Helm operation already in progress, skipping", "instance", instance.Name, "stack", instance.StackName, "deployment", instance.DeploymentID, "errorLog", deployErrorLog)
			return nil
		}
		if strings.Contains(string(deployErrorLog), fmt.Sprintf("namespaces %q not found", group.Namespace)) {
			return errdef.NewBadRequest("namespace %q does not exist", group.Namespace)
		}
		return fmt.Errorf("%w: %s", err, deployErrorLog)
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
	instances, err := s.DeploymentOrder(deployment)
	if err != nil {
		return err
	}
	slices.Reverse(instances)

	var errs error
	for _, instance := range instances {
		err := s.DestroyInstance(ctx, instance)
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

func (s Service) DestroyInstance(ctx context.Context, instance *model.DeploymentInstance) error {
	if _, err := s.FindDeploymentInstanceById(ctx, instance.ID); err != nil {
		return err
	}

	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	destroyCmd, err := s.helmfileService.destroy(ctx, instance, group)
	if err != nil {
		return err
	}

	destroyLog, destroyErrorLog, err := commandExecutor(destroyCmd, group.Cluster)
	// In recent versions of helmfile most of the command output is sent to stderr https://github.com/roboll/helmfile/pull/583
	s.logger.InfoContext(ctx, "Destroy log", "log", destroyLog, "errorLog", destroyErrorLog)
	if err != nil {
		return err
	}

	ks, err := NewKubernetesService(group.Cluster)
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

	ks, err := NewKubernetesService(group.Cluster)
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

	ks, err := NewKubernetesService(group.Cluster)
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

	ks, err := NewKubernetesService(group.Cluster)
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
	ks, err := NewKubernetesService(group.Cluster)
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
	groupNames := slices.Collect(maps.Keys(groupsByName))

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
	for i, name := range slices.Collect(maps.Keys(groupsByName)) {
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
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Hostname    string    `json:"hostname"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
			if instance.GroupName == name && instance.StackName == "dhis2-core" {
				publicInstance := PublicInstance{
					Name:        instance.Name,
					Description: instance.Deployment.Description,
					Hostname:    fmt.Sprintf("https://%s/%s", instance.Group.Hostname, instance.Name),
					UpdatedAt:   instance.UpdatedAt,
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

		if len(stableCategory.Instances) > 0 {
			groupWithPublicInstances.Categories = append(groupWithPublicInstances.Categories, stableCategory)
		}

		if len(devCategory.Instances) > 0 {
			groupWithPublicInstances.Categories = append(groupWithPublicInstances.Categories, devCategory)
		}

		if len(nightlyCategory.Instances) > 0 {
			groupWithPublicInstances.Categories = append(groupWithPublicInstances.Categories, nightlyCategory)
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
	ks, err := NewKubernetesService(instance.Group.Cluster)
	if err != nil {
		return "", err
	}

	pod, err := ks.getPod(instance.ID, "")
	if err != nil {
		if errdef.IsNotFound(err) {
			s.logger.Info("Pod not found, assuming not deployed", "instance", instance.ID, "group", instance.GroupName, "error", err)
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

func (s Service) FilestoreBackup(ctx context.Context, instance *model.DeploymentInstance, name string, database *model.Database) error {
	// Detach from the request context so the backup isn't cancelled if the client disconnects.
	ctx = context.WithoutCancel(ctx)

	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return err
	}

	// Re-fetch decrypted so STORAGE_TYPE and any external S3 credentials are populated.
	core, err := s.FindDecryptedDeploymentInstanceById(ctx, instance.ID)
	if err != nil {
		return err
	}

	baseName := name
	baseName = strings.TrimSuffix(baseName, ".sql.gz")
	baseName = strings.TrimSuffix(baseName, ".pgc")
	baseName = strings.TrimSuffix(baseName, ".tar.gz")

	streamer, err := s.filestoreStreamerFor(core, group.Cluster)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s/%s-%s.tar.gz", instance.GroupName, baseName, "fs")
	backupService := NewBackupService(s.logger, s.s3Client)
	if err := backupService.PerformBackup(ctx, streamer, s.s3Bucket, key); err != nil {
		return err
	}

	s3Uri := fmt.Sprintf("s3://%s/%s", s.s3Bucket, key)
	filestore, err := s.recordBackup(ctx, instance.GroupName, s3Uri, baseName+"-fs.tar.gz", database.UserID)
	if err != nil {
		return err
	}

	database.FilestoreID = filestore.ID

	return s.instanceRepository.SaveDatabase(ctx, database)
}

func (s Service) recordBackup(ctx context.Context, groupName, s3uri, name string, userID uint) (*model.Database, error) {
	database := &model.Database{
		Name:      name,
		GroupName: groupName,
		Url:       s3uri,
		Type:      "fs",
		UserID:    userID,
	}
	err := s.instanceRepository.RecordBackup(ctx, database)
	if err != nil {
		return nil, err
	}

	return database, nil
}

func (s Service) FindAllDeployments(ctx context.Context) ([]model.Deployment, error) {
	return s.instanceRepository.FindAllDeployments(ctx)
}

func (s Service) UpdateInstanceParameters(ctx context.Context, deploymentId, instanceId uint, parameters Parameters, public *bool) (*model.DeploymentInstance, error) {
	instance, err := s.FindDecryptedDeploymentInstanceById(ctx, instanceId)
	if err != nil {
		return nil, err
	}

	if instance.DeploymentID != deploymentId {
		return nil, errdef.NewBadRequest("instance %d does not belong to deployment %d", instanceId, deploymentId)
	}

	if public != nil {
		instance.Public = *public
	}

	if err := s.rejectConsumedParameters(instance.StackName, maps.Keys(parameters)); err != nil {
		return nil, err
	}

	for name, parameter := range parameters {
		instance.Parameters[name] = model.DeploymentInstanceParameter{
			ParameterName: name,
			Value:         parameter.Value,
		}
	}

	deployment, err := s.FindDeploymentById(ctx, deploymentId)
	if err != nil {
		return nil, err
	}

	decryptedDeployment, err := s.decryptDeployment(deployment)
	if err != nil {
		return nil, err
	}

	for i, inst := range decryptedDeployment.Instances {
		if inst.ID == instanceId {
			decryptedDeployment.Instances[i] = instance
			break
		}
	}

	_, err = s.validateNoCycles(decryptedDeployment.Instances)
	if err != nil {
		return nil, errdef.NewBadRequest("failed to validate instance: %v", err)
	}

	err = s.resolveParameters(decryptedDeployment)
	if err != nil {
		return nil, errdef.NewBadRequest("failed to resolve parameters: %v", err)
	}

	stack, err := s.stackService.Find(instance.StackName)
	if err != nil {
		return nil, err
	}

	err = s.instanceRepository.SaveInstance(ctx, instance, stack)
	if err != nil {
		return nil, err
	}

	return instance, nil
}
