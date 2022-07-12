package instance

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"go.mozilla.org/sops/v3/decrypt"
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

func (k kubernetesService) CommandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) (stdout []byte, stderr []byte, err error) {
	if len(configuration.KubernetesConfiguration) == 0 {
		return runCommand(cmd)
	}

	kubeCfg, err := decryptYaml(configuration.KubernetesConfiguration)
	if err != nil {
		return nil, nil, err
	}

	file, err := ioutil.TempFile("", "kubectl")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		// TODO this comment can be remove later on, its just to explain an interesting detail about
		// os.Remove and not using defer on file.Close()
		// os.Remove will successfully "remove" the file even if its still open
		// previously, if file.Write() failed for example when there is no space on tmp
		// the actual file would still be referenced (at least for a little longer)
		// https://stackoverflow.com/questions/19441823/a-file-opened-for-read-and-write-can-be-unlinked
		err = file.Close()
		// TODO here in case err is != nil due to a failed close we would discard it; ok with me :)
		err = os.Remove(file.Name())
	}()

	_, err = file.Write(kubeCfg)
	if err != nil {
		return nil, nil, err
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", file.Name()))
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
		kubeCfg, err := decryptYaml(configuration.KubernetesConfiguration)
		if err != nil {
			return nil, err
		}

		config, err := clientcmd.NewClientConfigFromBytes(kubeCfg)
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
	// TODO: This isn't good code. The error returned could be from either newClient or from fn
	client, err := newClient(configuration)
	if err != nil {
		return err
	}
	return fn(client)
}

func decryptYaml(data []byte) ([]byte, error) {
	return decrypt.DataWithFormat(data, formats.FormatFromString("yaml"))
}
