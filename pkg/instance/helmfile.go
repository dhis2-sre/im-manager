package instance

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
)

type HelmfileService interface {
	Sync(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error)
	Destroy(token string, instance *model.Instance, group *models.Group) (*exec.Cmd, error)
}

type helmfileService struct {
	stackService stack.Service
	config       config.Config
}

func NewHelmfileService(stackService stack.Service, config config.Config) helmfileService {
	return helmfileService{
		stackService,
		config,
	}
}

func (h helmfileService) Sync(accessToken string, instance *model.Instance, group *models.Group) (*exec.Cmd, error) {
	return h.executeHelmfileCommand(accessToken, instance, group, "sync")
}

func (h helmfileService) Destroy(accessToken string, instance *model.Instance, group *models.Group) (*exec.Cmd, error) {
	return h.executeHelmfileCommand(accessToken, instance, group, "destroy")
}

// **Security considerations**
// * No shell is invoked - So && and ; doesn't work even if an attacker managed to inject them
// * STACKS_FOLDER is configured on the host so if that can be tampered the attacker already has access
// * stack.Name is populated by reading the name of a folder and even if that folder name could contain something malicious it won't be running in a shell anyway
// * stackPath is concatenated using path.Join which also cleans the path and furthermore it's existence is validated
// * Binaries are executed using their full path and not from $PATH which would be very difficult to exploit anyway
func (h helmfileService) executeHelmfileCommand(accessToken string, instance *model.Instance, group *models.Group, operation string) (*exec.Cmd, error) {
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		return nil, err
	}

	// TODO
	// stacksFolder := h.config.StacksFolder
	stacksFolder := "./stacks"

	stackPath := path.Join(stacksFolder, "/", stack.Name, "/helmfile.yaml")
	if _, err = os.Stat(stackPath); err != nil {
		log.Printf("Stack doesn't exists: %s\n", stackPath)
		return nil, err
	}

	stackParameters, err := h.loadStackParameters(stacksFolder, stack.Name)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("/usr/bin/helmfile", "--helm-binary", "/usr/bin/helm", "-f", stackPath, operation)
	log.Printf("Command: %s\n", cmd.String())
	h.configureInstanceEnvironment(accessToken, instance, group, stackParameters, cmd)

	return cmd, nil
}

type StackParameters map[string]string

func (h helmfileService) loadStackParameters(folder string, name string) (*StackParameters, error) {
	environment := h.config.Environment
	path := fmt.Sprintf("%s/%s/parameters/%s/parameters.yaml", folder, name, environment)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	b, err := decryptYaml(data)
	if err != nil {
		return nil, err
	}

	params := &StackParameters{}
	err = yaml.Unmarshal(b, params)
	if err != nil {
		return nil, err
	}

	return params, nil
}

func (h helmfileService) configureInstanceEnvironment(accessToken string, instance *model.Instance, group *models.Group, stackParameters *StackParameters, cmd *exec.Cmd) {
	// TODO: We should only inject what the stack require, currently we just blindly inject IM_ACCESS_TOKEN and others which may not be required by the stack
	// We could probably list the required system parameters in the stacks helmfile and parse those as well as other parameters
	instanceNameEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAME", instance.Name)
	instanceNamespaceEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAMESPACE", group.Name)
	instanceIdEnv := fmt.Sprintf("%s=%d", "INSTANCE_ID", instance.ID)
	instanceHostnameEnv := fmt.Sprintf("%s=%s", "INSTANCE_HOSTNAME", group.Hostname)
	imTokenEnv := fmt.Sprintf("%s=%s", "IM_ACCESS_TOKEN", accessToken)
	homeEnv := fmt.Sprintf("%s=%s", "HOME", "/tmp")
	cmd.Env = append(cmd.Env, instanceNameEnv, instanceNamespaceEnv, instanceIdEnv, instanceHostnameEnv, homeEnv, imTokenEnv)

	h.injectEnv("AWS_ACCESS_KEY_ID", &cmd.Env)
	h.injectEnv("AWS_SECRET_ACCESS_KEY", &cmd.Env)
	h.injectEnv("AWS_DEFAULT_REGION", &cmd.Env)
	h.injectEnv("AWS_REGION", &cmd.Env)
	h.injectEnv("AWS_ROLE_ARN", &cmd.Env)
	h.injectEnv("AWS_WEB_IDENTITY_TOKEN_FILE", &cmd.Env)

	h.injectEnv("KUBERNETES_SERVICE_PORT", &cmd.Env)
	h.injectEnv("KUBERNETES_PORT", &cmd.Env)
	h.injectEnv("KUBERNETES_PORT_443_TCP_ADDR", &cmd.Env)
	h.injectEnv("KUBERNETES_PORT_443_TCP_PORT", &cmd.Env)
	h.injectEnv("KUBERNETES_PORT_443_TCP_PROTO", &cmd.Env)
	h.injectEnv("KUBERNETES_PORT_443_TCP", &cmd.Env)
	h.injectEnv("KUBERNETES_SERVICE_PORT_HTTPS", &cmd.Env)
	h.injectEnv("KUBERNETES_SERVICE_HOST", &cmd.Env)

	for _, parameter := range instance.RequiredParameters {
		instanceEnv := fmt.Sprintf("%s=%s", parameter.StackRequiredParameter.Name, parameter.Value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}

	for _, parameter := range instance.OptionalParameters {
		instanceEnv := fmt.Sprintf("%s=%s", parameter.StackOptionalParameter.Name, parameter.Value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}

	for parameter, value := range *stackParameters {
		instanceEnv := fmt.Sprintf("%s=%s", parameter, value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}
}

func (h helmfileService) injectEnv(env string, envs *[]string) {
	if value, exists := os.LookupEnv(env); exists {
		cmdEnv := fmt.Sprintf("%s=%s", env, value)
		*envs = append(*envs, cmdEnv)
	} else {
		log.Println("WARNING!!! Env not found:", env)
	}
}
