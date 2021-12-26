package instance

import (
	"context"
	"fmt"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	userClient "github.com/dhis2-sre/im-user/pkg/client"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"io"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
)

type Service interface {
	Create(instance *model.Instance, group *models.Group) error
	FindById(id uint) (*model.Instance, error)
	Delete(id uint, group *models.Group) error
	Logs(instance *model.Instance, group *models.Group) (io.ReadCloser, error)
	FindWithParametersById(id uint) (*model.Instance, error)
	FindByNameAndGroup(instanceName string, groupId uint) (*model.Instance, error)
	FindInstances(groups []*models.Group) ([]*model.Instance, error)
}

func ProvideService(
	config config.Config,
	instanceRepository Repository,
	userClient userClient.Client,
	kubernetesService KubernetesService,
	helmfileService HelmfileService,
) Service {
	return &service{
		config,
		instanceRepository,
		userClient,
		helmfileService,
		kubernetesService,
	}
}

type service struct {
	config             config.Config
	instanceRepository Repository
	userClient         userClient.Client
	helmfileService    HelmfileService
	kubernetesService  KubernetesService
}

func (s service) Create(instance *model.Instance, group *models.Group) error {
	err := s.instanceRepository.Create(instance)
	if err != nil {
		return err
	}

	instanceWithParameters, err := s.instanceRepository.FindWithParametersById(instance.ID)
	if err != nil {
		return err
	}

	syncCmd, err := s.helmfileService.Sync(instanceWithParameters, group)
	if err != nil {
		return err
	}

	deployLog, deployErrorLog, err := s.kubernetesService.CommandExecutor(syncCmd, group.ClusterConfiguration)
	log.Printf("Deploy log: %s\n", deployLog)
	log.Printf("Error log: %s", deployErrorLog)
	if err != nil {
		return err
	}

	err = s.instanceRepository.SaveDeployLog(instance, string(deployLog))
	instance.DeployLog = string(deployLog)
	if err != nil {
		// TODO
		log.Printf("Store error log: %s", deployErrorLog)
		return err
	}

	return nil
}

func (s service) FindById(id uint) (*model.Instance, error) {
	return s.instanceRepository.FindById(id)
}

func (s service) Delete(id uint, group *models.Group) error {
	instanceWithParameters, err := s.instanceRepository.FindWithParametersById(id)
	if err != nil {
		return err
	}

	destroyCmd, err := s.helmfileService.Destroy(instanceWithParameters, group)
	if err != nil {
		return err
	}

	destroyLog, destroyErrorLog, err := s.kubernetesService.CommandExecutor(destroyCmd, group.ClusterConfiguration)
	log.Printf("Destroy log: %s\n", destroyLog)
	log.Printf("Destroy error log: %s\n", destroyErrorLog)
	if err != nil {
		return err
	}
	/*
		err = s.instanceRepository.SaveDeployLog(instanceWithParameters, string(deployLog))
		instanceWithParameters.DeployLog = string(deployLog)
		if err != nil {
			// TODO
			log.Printf("Store error log: %s", deployErrorLog)
			return err
		}
	*/

	return s.instanceRepository.Delete(id)
}

func (s service) Logs(instance *model.Instance, group *models.Group) (io.ReadCloser, error) {
	var read io.ReadCloser

	err := s.kubernetesService.Executor(group.ClusterConfiguration, func(client *kubernetes.Clientset) error {
		pod := s.getPod(client, instance)

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

func (s service) getPod(client *kubernetes.Clientset, instance *model.Instance) v1.Pod {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("dhis2-id=%d", instance.ID),
	}

	podList, err := client.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		log.Fatalln(err)
	}

	if len(podList.Items) > 1 {
		log.Fatalln("More than one pod found... TODO")
	}

	return podList.Items[0]
}

func (s service) FindWithParametersById(id uint) (*model.Instance, error) {
	return s.instanceRepository.FindWithParametersById(id)
}

func (s service) FindByNameAndGroup(instanceName string, groupId uint) (*model.Instance, error) {
	return s.instanceRepository.FindByNameAndGroup(instanceName, groupId)
}

func (s service) FindInstances(groups []*models.Group) ([]*model.Instance, error) {
	groupIds := make([]uint, len(groups))
	for i, group := range groups {
		groupIds[i] = uint(group.ID)
	}

	instances, err := s.instanceRepository.FindByGroupIds(groupIds)
	if err != nil {
		return nil, err
	}
	return instances, nil
}
