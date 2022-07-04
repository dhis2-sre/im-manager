package instance

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/dhis2-sre/im-manager/internal/apperror"
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
	LinkDeploy(token string, oldInstance, newInstance *model.Instance) error
	Link(firstID, secondID uint, stackName string) error
	Unlink(id uint) error
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

type service struct {
	config             config.Config
	instanceRepository Repository
	userClient         userClientService
	stackService       stack.Service
	helmfileService    HelmfileService
	kubernetesService  KubernetesService
}

func NewService(
	config config.Config,
	instanceRepository Repository,
	userClient userClientService,
	stackService stack.Service,
	kubernetesService KubernetesService,
	helmfileService HelmfileService,
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

func (s service) LinkDeploy(token string, oldInstance, newInstance *model.Instance) error {
	err := s.Link(oldInstance.ID, newInstance.ID, newInstance.StackName)
	if err != nil {
		return err
	}

	oldStack, err := s.stackService.Find(oldInstance.StackName)
	if err != nil {
		return err
	}

	newStack, err := s.stackService.Find(newInstance.StackName)
	if err != nil {
		return err
	}

	// Consumed required parameters
	for _, v := range newStack.RequiredParameters {
		if v.Consumed && v.Name != newStack.HostnameVariable {
			parameter, err := oldInstance.FindRequiredParameter(v.Name)
			if err != nil {
				return err
			}
			parameterRequest := model.InstanceRequiredParameter{
				StackRequiredParameterID: v.Name,
				Value:                    parameter.Value,
			}
			newInstance.RequiredParameters = append(newInstance.RequiredParameters, parameterRequest)
		}
	}

	// Consumed optional parameters
	for _, v := range newStack.OptionalParameters {
		if v.Consumed && v.Name != newStack.HostnameVariable {
			parameter, err := oldInstance.FindOptionalParameter(v.Name)
			if err != nil {
				// TODO: Better error handling
				stackParameter, serr := oldStack.FindOptionalParameter(v.Name)
				if serr != nil {
					// TODO: don't use apperror here
					notFound := apperror.NewNotFound("optional stack parameter", v.Name)
					return notFound
				}
				parameter.Value = stackParameter.DefaultValue
			}
			parameterRequest := model.InstanceOptionalParameter{
				StackOptionalParameterID: v.Name,
				Value:                    parameter.Value,
			}
			newInstance.OptionalParameters = append(newInstance.OptionalParameters, parameterRequest)
		}
	}

	// Hostname parameter
	// TODO: Is hostname always a required parameter?
	if newStack.HostnameVariable != "" {
		hostnameParameter := model.InstanceRequiredParameter{
			StackRequiredParameterID: newStack.HostnameVariable,
			Value:                    fmt.Sprintf(oldStack.HostnamePattern, oldInstance.Name, oldInstance.GroupName),
		}
		newInstance.RequiredParameters = append(newInstance.RequiredParameters, hostnameParameter)
	}

	err = s.Deploy(token, newInstance)
	if err != nil {
		return err
	}

	return nil
}

func (s service) Link(firstID, secondID uint, stackName string) error {
	instance := &model.Instance{
		Model: gorm.Model{ID: firstID},
	}
	secondInstance := &model.Instance{
		Model:     gorm.Model{ID: secondID},
		StackName: stackName,
	}
	return s.instanceRepository.Link(instance, secondInstance)
}

func (s service) Unlink(id uint) error {
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

	syncCmd, err := s.helmfileService.Sync(accessToken, instanceWithParameters, group)
	if err != nil {
		return err
	}

	deployLog, deployErrorLog, err := s.kubernetesService.CommandExecutor(syncCmd, group.ClusterConfiguration)
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
	instanceWithParameters, err := s.FindWithDecryptedParametersById(id)
	if err != nil {
		return err
	}

	if instanceWithParameters.DeployLog != "" {
		group, err := s.userClient.FindGroupByName(token, instanceWithParameters.GroupName)
		if err != nil {
			return err
		}

		destroyCmd, err := s.helmfileService.Destroy(token, instanceWithParameters, group)
		if err != nil {
			return err
		}

		destroyLog, destroyErrorLog, err := s.kubernetesService.CommandExecutor(destroyCmd, group.ClusterConfiguration)
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

	err := s.kubernetesService.Executor(group.ClusterConfiguration, func(client *kubernetes.Clientset) error {
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
