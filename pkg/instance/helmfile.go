package instance

import (
	"fmt"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"log"
	"os"
	"os/exec"
	"path"
)

type HelmfileService interface {
	Sync(instance *model.Instance, group *models.Group) (*exec.Cmd, error)
	Destroy(instance *model.Instance, group *models.Group) (*exec.Cmd, error)
}

func ProvideHelmfileService(stackService stack.Service, config config.Config) HelmfileService {
	return helmfileService{
		stackService,
		config,
	}
}

type helmfileService struct {
	stackService stack.Service
	config       config.Config
}

func (h helmfileService) Sync(instance *model.Instance, group *models.Group) (*exec.Cmd, error) {
	return h.executeHelmfileCommand(instance, group, "sync")
}

func (h helmfileService) Destroy(instance *model.Instance, group *models.Group) (*exec.Cmd, error) {
	return h.executeHelmfileCommand(instance, group, "destroy")
}

// **Security considerations**
// * No shell is invoked - So && and ; doesn't work even if an attacker managed to inject them
// * STACKS_FOLDER is configured on the host so if that can be tampered the attacker already has access
// * stack.Name is populated by reading the name of a folder and even if that folder name could contain something malicious it won't be running in a shell anyway
// * stackPath is concatenated using path.Join which also cleans the path and furthermore it's existence is validated
// * Binaries are executed using their full path and not from $PATH which would be very difficult to exploit anyway
func (h helmfileService) executeHelmfileCommand(instance *model.Instance, group *models.Group, operation string) (*exec.Cmd, error) {
	stack, err := h.stackService.FindById(instance.StackID)
	if err != nil {
		return nil, err
	}

	// TODO
	//stacksFolder := h.config.StacksFolder
	stacksFolder := "./stacks"

	stackPath := path.Join(stacksFolder, "/", stack.Name, "/helmfile.yaml")
	if _, err = os.Stat(stackPath); err != nil {
		log.Printf("Stack doesn't exists: %s\n", stackPath)
		return nil, err
	}

	cmd := exec.Command("/usr/bin/helmfile", "--helm-binary", "/usr/bin/helm", "-f", stackPath, operation)
	log.Printf("Command: %s\n", cmd.String())

	h.configureInstanceEnvironment(instance, group, cmd)
	return cmd, nil
}

func (h helmfileService) configureInstanceEnvironment(instance *model.Instance, group *models.Group, cmd *exec.Cmd) {
	instanceNameEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAME", instance.Name)
	instanceNamespaceEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAMESPACE", group.Name)
	instanceIdEnv := fmt.Sprintf("%s=%d", "INSTANCE_ID", instance.ID)
	instanceHostnameEnv := fmt.Sprintf("%s=%s", "INSTANCE_HOSTNAME", group.Hostname)
	homeEnv := fmt.Sprintf("%s=%s", "HOME", "/tmp")
	cmd.Env = append(cmd.Env, instanceNameEnv, instanceNamespaceEnv, instanceIdEnv, instanceHostnameEnv, homeEnv)

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
}

func (h helmfileService) injectEnv(env string, envs *[]string) {
	if value, exists := os.LookupEnv(env); exists {
		cmdEnv := fmt.Sprintf("%s=%s", env, value)
		*envs = append(*envs, cmdEnv)
	}
}
