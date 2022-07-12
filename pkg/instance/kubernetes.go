package instance

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesService interface {
	Executor(configuration *models.ClusterConfiguration, fn func(client *kubernetes.Clientset) error) error
	CommandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) ([]byte, []byte, error)
}

type kubernetesService struct{}

func NewKubernetesService() *kubernetesService {
	return &kubernetesService{}
}

func (k kubernetesService) CommandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) ([]byte, []byte, error) {
	if len(configuration.KubernetesConfiguration) > 0 {
		kubernetesConfigurationInCleartext, err := decryptYaml(configuration.KubernetesConfiguration)
		if err != nil {
			return nil, nil, err
		}

		file, err := ioutil.TempFile("", "kubectl")
		if err != nil {
			return nil, nil, err
		}

		defer func(name string) {
			err := os.Remove(name)
			if err != nil {
				// TODO ... What to do?
			}
		}(file.Name())

		_, err = file.Write(kubernetesConfigurationInCleartext)
		if err != nil {
			return nil, nil, err
		}

		err = file.Close()
		if err != nil {
			return nil, nil, err
		}

		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", file.Name()))
	}
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}

func newClient(configuration *models.ClusterConfiguration) (*kubernetes.Clientset, error) {
	var restClientConfig *rest.Config
	if len(configuration.KubernetesConfiguration) > 0 {
		configurationInCleartext, err := decryptYaml(configuration.KubernetesConfiguration)
		if err != nil {
			return nil, err
		}

		config, err := clientcmd.NewClientConfigFromBytes(configurationInCleartext)
		if err != nil {
			return nil, err
		}

		restClientConfig, err = config.ClientConfig()
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		restClientConfig, err = clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			return nil, err
		}
	}

	client, err := kubernetes.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (k kubernetesService) Executor(configuration *models.ClusterConfiguration, fn func(client *kubernetes.Clientset) error) error {
	// TODO: This isn't good code. The error returned could be from either getClient or from fn
	client, err := newClient(configuration)
	if err != nil {
		return err
	}
	return fn(client)
}
