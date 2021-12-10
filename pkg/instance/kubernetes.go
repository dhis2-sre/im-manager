package instance

import (
	"bytes"
	"github.com/dhis2-sre/im-users/swagger/sdk/models"
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"go.mozilla.org/sops/v3/decrypt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os/exec"
)

type KubernetesService interface {
	Executor(configuration *models.ClusterConfiguration, fn func(client *kubernetes.Clientset) error) error
	CommandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) ([]byte, []byte, error)
}

func ProvideKubernetesService() KubernetesService {
	return &kubernetesService{}
}

type kubernetesService struct{}

func (k kubernetesService) CommandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) ([]byte, []byte, error) {
	/*
		if len(configuration.KubernetesConfiguration) > 0 {
			// Decrypt
			kubernetesConfigurationInCleartext, err := configuration.GetKubernetesConfigurationInCleartext()
			if err != nil {
				log.Printf("Error decrypting: %s\n", err)
				return nil, nil, err
			}

			// Create tmp file
			file, err := ioutil.TempFile("", "kubectl")
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}

			defer func(name string) {
				err := os.Remove(name)
				if err != nil {
					log.Println(err)
				}
			}(file.Name())

			// Write configuration to file
			_, err = file.Write(kubernetesConfigurationInCleartext)
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}

			err = file.Close()
			if err != nil {
				log.Println(err)
				return nil, nil, err
			}

			cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", file.Name()))
		}
	*/
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}

func (k kubernetesService) getClient(configuration *models.ClusterConfiguration) *kubernetes.Clientset {
	var restClientConfig *rest.Config
	/*
		if len(configuration.KubernetesConfiguration) > 0 {
			configurationInCleartext, err := k.decrypt(configuration.KubernetesConfiguration, "yaml")
			if err != nil {
				log.Println(err)
			}

			config, err := clientcmd.NewClientConfigFromBytes(configurationInCleartext)
			if err != nil {
				log.Println(err)
			}

			restClientConfig, err = config.ClientConfig()
			if err != nil {
				log.Println(err)
			}
		} else {
	*/
	var err error
	restClientConfig, err = clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Println(err)
	}
	/*
		}
	*/
	client, err := kubernetes.NewForConfig(restClientConfig)
	if err != nil {
		log.Println(err)
	}

	return client
}

func (k kubernetesService) Executor(configuration *models.ClusterConfiguration, fn func(client *kubernetes.Clientset) error) error {
	client := k.getClient(configuration)
	return fn(client)
}

func (k kubernetesService) decrypt(data []byte, format string) ([]byte, error) {
	kubernetesConfigurationCleartext, err := decrypt.DataWithFormat(data, formats.FormatFromString(format))
	if err != nil {
		log.Printf("Error decrypting: %s\n", err)
		return nil, err
	}
	return kubernetesConfigurationCleartext, nil
}
