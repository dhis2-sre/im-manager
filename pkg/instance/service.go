package instance

import (
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	userClient "github.com/dhis2-sre/im-users/pkg/client"
	"log"
)

type Service interface {
	Create(instance *model.Instance) error
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

func (s service) Create(instance *model.Instance) error {
	err := s.instanceRepository.Create(instance)
	if err != nil {
		return err
	}

	instanceWithParameters, err := s.instanceRepository.FindWithParametersById(instance.ID)
	if err != nil {
		return err
	}

	group, err := s.userClient.FindGroupById(instance.GroupID)
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
