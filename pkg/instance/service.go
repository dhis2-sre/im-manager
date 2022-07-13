package instance

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"gorm.io/gorm"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Service interface {
	LinkDeploy(token string, sourceInstance, destinationInstance *model.Instance) error
	Restart(token string, id uint) error
	Create(instance *model.Instance) (*model.Instance, error)
	Deploy(token string, instance *model.Instance) error
	FindById(id uint) (*model.Instance, error)
	Delete(token string, id uint) error
	Logs(instance *model.Instance, group *models.Group, selector string) (io.ReadCloser, error)
	FindWithParametersById(id uint) (*model.Instance, error)
	FindWithDecryptedParametersById(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	FindInstances(groups []*models.Group) ([]*model.Instance, error)
}

type helmfile interface {
	sync(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error)
	destroy(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error)
}

type kubernetesExecutor interface {
	executor(configuration *models.ClusterConfiguration, fn func(client *kubernetes.Clientset) error) error
	commandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) ([]byte, []byte, error)
}

type service struct {
	config             config.Config
	instanceRepository Repository
	userClient         userClientService
	stackService       stack.Service
	helmfileService    helmfile
	kubernetesService  kubernetesExecutor
}

func NewService(
	config config.Config,
	instanceRepository Repository,
	userClient userClientService,
	stackService stack.Service,
	kubernetesService kubernetesExecutor,
	helmfileService helmfile,
) *service {
	return &service{
		config,
		instanceRepository,
		userClient,
		stackService,
		helmfileService,
		kubernetesService,
	}
}

type userClientService interface {
	FindGroupByName(token string, name string) (*models.Group, error)
}

func (s service) LinkDeploy(token string, sourceInstance, destinationInstance *model.Instance) error {
	err := s.link(sourceInstance, destinationInstance)
	if err != nil {
		return err
	}

	sourceStack, err := s.stackService.Find(sourceInstance.StackName)
	if err != nil {
		return err
	}

	destinationStack, err := s.stackService.Find(destinationInstance.StackName)
	if err != nil {
		return err
	}

	// Consumed required parameters
	for _, parameter := range destinationStack.RequiredParameters {
		if parameter.Consumed && parameter.Name != destinationStack.HostnameVariable {
			value, err := s.findParameterValue(parameter.Name, sourceInstance, sourceStack)
			if err != nil {
				return err
			}
			parameterRequest := model.InstanceRequiredParameter{
				StackRequiredParameterID: parameter.Name,
				Value:                    value,
			}
			destinationInstance.RequiredParameters = append(destinationInstance.RequiredParameters, parameterRequest)
		}
	}

	// Consumed optional parameters
	for _, parameter := range destinationStack.OptionalParameters {
		if parameter.Consumed && parameter.Name != destinationStack.HostnameVariable {
			value, err := s.findParameterValue(parameter.Name, sourceInstance, sourceStack)
			if err != nil {
				return err
			}
			parameterRequest := model.InstanceOptionalParameter{
				StackOptionalParameterID: parameter.Name,
				Value:                    value,
			}
			destinationInstance.OptionalParameters = append(destinationInstance.OptionalParameters, parameterRequest)
		}
	}

	// Hostname parameter
	if destinationStack.HostnameVariable != "" {
		hostnameParameter := model.InstanceRequiredParameter{
			StackRequiredParameterID: destinationStack.HostnameVariable,
			Value:                    fmt.Sprintf(sourceStack.HostnamePattern, sourceInstance.Name, sourceInstance.GroupName),
		}
		destinationInstance.RequiredParameters = append(destinationInstance.RequiredParameters, hostnameParameter)
	}

	err = s.Deploy(token, destinationInstance)
	if err != nil {
		return err
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

func (s service) Restart(token string, id uint) error {
	instance, err := s.FindById(id)
	if err != nil {
		return err
	}

	group, err := s.userClient.FindGroupByName(token, instance.GroupName)
	if err != nil {
		return err
	}

	err = s.kubernetesService.executor(group.ClusterConfiguration, func(client *kubernetes.Clientset) error {
		deployments := client.AppsV1().Deployments(instance.GroupName)

		labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", instance.Name)
		listOptions := metav1.ListOptions{LabelSelector: labelSelector}
		deploymentList, err := deployments.List(context.TODO(), listOptions)
		if err != nil {
			return err
		}

		items := deploymentList.Items
		if len(items) > 1 {
			return fmt.Errorf("multiple deployments found using the selector: %q", labelSelector)
		}
		if len(items) < 1 {
			return fmt.Errorf("no deployment found using the selector: %q", labelSelector)
		}

		name := items[0].Name

		// Scale down
		deployment, err := deployments.GetScale(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		replicas := deployment.Spec.Replicas
		deployment.Spec.Replicas = 0

		_, err = deployments.UpdateScale(context.TODO(), name, deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		// Scale up
		updatedDeployment, err := deployments.GetScale(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updatedDeployment.Spec.Replicas = replicas
		_, err = deployments.UpdateScale(context.TODO(), name, updatedDeployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (s service) link(source, destination *model.Instance) error {
	return s.instanceRepository.Link(source, destination)
}

func (s service) unlink(id uint) error {
	instance := &model.Instance{
		Model: gorm.Model{ID: id},
	}
	return s.instanceRepository.Unlink(instance)
}

func (s service) Create(instance *model.Instance) (*model.Instance, error) {
	err := s.instanceRepository.Create(instance)
	if err != nil {
		return nil, err
	}

	instanceWithParameters, err := s.instanceRepository.FindWithParametersById(instance.ID)
	if err != nil {
		return nil, err
	}

	return instanceWithParameters, nil
}

func (s service) Deploy(accessToken string, instance *model.Instance) error {
	enrichParameters(instance)

	encryptInstance, err := s.encryptParameters(instance)
	if err != nil {
		return err
	}

	err = s.instanceRepository.Save(encryptInstance)
	if err != nil {
		return err
	}

	instanceWithParameters, err := s.FindWithDecryptedParametersById(encryptInstance.ID)
	if err != nil {
		return err
	}

	group, err := s.userClient.FindGroupByName(accessToken, instance.GroupName)
	if err != nil {
		return err
	}

	syncCmd, err := s.helmfileService.sync(accessToken, instanceWithParameters, group)
	if err != nil {
		return err
	}

	deployLog, deployErrorLog, err := s.kubernetesService.commandExecutor(syncCmd, group.ClusterConfiguration)
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

	err = s.instanceRepository.SaveDeployLog(instanceWithParameters, string(deployLog))
	instance.DeployLog = string(deployLog)
	if err != nil {
		// TODO
		log.Printf("Store error log: %s", deployErrorLog)
		return err
	}

	return nil
}

func enrichParameters(instance *model.Instance) {
	requiredParameters := instance.RequiredParameters
	if len(requiredParameters) > 0 {
		for i := range requiredParameters {
			requiredParameters[i].InstanceID = instance.ID
			requiredParameters[i].StackName = instance.StackName
		}
	}

	optionalParameters := instance.OptionalParameters
	if len(optionalParameters) > 0 {
		for i := range optionalParameters {
			optionalParameters[i].InstanceID = instance.ID
			optionalParameters[i].StackName = instance.StackName
		}
	}
}

func (s service) FindById(id uint) (*model.Instance, error) {
	return s.instanceRepository.FindById(id)
}

func (s service) Delete(token string, id uint) error {
	err := s.unlink(id)
	if err != nil {
		return err
	}

	instanceWithParameters, err := s.FindWithDecryptedParametersById(id)
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

		destroyLog, destroyErrorLog, err := s.kubernetesService.commandExecutor(destroyCmd, group.ClusterConfiguration)
		log.Printf("Destroy log: %s\n", destroyLog)
		log.Printf("Destroy error log: %s\n", destroyErrorLog)
		if err != nil {
			return err
		}
	}

	return s.instanceRepository.Delete(id)
}

func (s service) Logs(instance *model.Instance, group *models.Group, selector string) (io.ReadCloser, error) {
	var read io.ReadCloser

	err := s.kubernetesService.executor(group.ClusterConfiguration, func(client *kubernetes.Clientset) error {
		pod, err := s.getPod(client, instance, selector)
		if err != nil {
			return err
		}

		podLogOptions := v1.PodLogOptions{
			Follow: true,
		}

		readCloser, err := client.
			CoreV1().
			Pods(pod.Namespace).
			GetLogs(pod.Name, &podLogOptions).
			Stream(context.TODO())
		read = readCloser
		return err
	})

	return read, err
}

func (s service) getPod(client *kubernetes.Clientset, instance *model.Instance, selector string) (v1.Pod, error) {
	var labelSelector string
	if selector == "" {
		labelSelector = fmt.Sprintf("im-id=%d", instance.ID)
	} else {
		labelSelector = fmt.Sprintf("im-%s-id=%d", selector, instance.ID)
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	podList, err := client.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		return v1.Pod{}, err
	}

	if len(podList.Items) > 1 {
		return v1.Pod{}, fmt.Errorf("multiple pods found using the selector: %s", labelSelector)
	}

	return podList.Items[0], nil
}

func (s service) FindWithParametersById(id uint) (*model.Instance, error) {
	return s.instanceRepository.FindWithParametersById(id)
}

func (s service) FindWithDecryptedParametersById(id uint) (*model.Instance, error) {
	instance, err := s.instanceRepository.FindWithParametersById(id)
	if err != nil {
		return nil, err
	}

	err = s.decryptParameters(instance)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (s service) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	return s.instanceRepository.FindByNameAndGroup(instance, group)
}

func (s service) FindInstances(groups []*models.Group) ([]*model.Instance, error) {
	groupNames := make([]string, len(groups))
	for i, group := range groups {
		groupNames[i] = group.Name
	}

	instances, err := s.instanceRepository.FindByGroupNames(groupNames)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

func (s service) encryptParameters(instance *model.Instance) (*model.Instance, error) {
	for i, parameter := range instance.RequiredParameters {
		value, err := encryptText(parameter.Value, s.config.InstanceParameterEncryptionKey)
		if err != nil {
			return nil, err
		}
		instance.RequiredParameters[i].Value = value
	}

	for i, parameter := range instance.OptionalParameters {
		value, err := encryptText(parameter.Value, s.config.InstanceParameterEncryptionKey)
		if err != nil {
			return nil, err
		}
		instance.OptionalParameters[i].Value = value
	}

	return instance, nil
}

func (s service) decryptParameters(instance *model.Instance) error {
	for i, parameter := range instance.RequiredParameters {
		value, err := decryptText(parameter.Value, s.config.InstanceParameterEncryptionKey)
		if err != nil {
			return err
		}
		instance.RequiredParameters[i].Value = value
	}

	for i, parameter := range instance.OptionalParameters {
		value, err := decryptText(parameter.Value, s.config.InstanceParameterEncryptionKey)
		if err != nil {
			return err
		}
		instance.OptionalParameters[i].Value = value
	}

	return nil
}
