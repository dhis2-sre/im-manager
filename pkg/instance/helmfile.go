package instance

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewHelmfileService(stackFolder string, stackService stackService, classification string) helmfileService {
	return helmfileService{stackFolder, stackService, classification}
}

type helmfileService struct {
	stackFolder    string
	stackService   stackService
	classification string
}

type stackService interface {
	Find(name string) (*model.Stack, error)
}

func (h helmfileService) sync(token string, instance *model.DeploymentInstance, group *model.Group, ttl uint) (*exec.Cmd, error) {
	return h.executeHelmfileCommand(token, instance, group, ttl, "sync")
}

func (h helmfileService) destroy(instance *model.DeploymentInstance, group *model.Group) (*exec.Cmd, error) {
	return h.executeHelmfileCommand("token", instance, group, 0, "destroy")
}

// **Security considerations**
// * No shell is invoked - So && and ; doesn't work even if an attacker managed to inject them
// * STACKS_FOLDER is configured on the host so if that can be tampered the attacker already has access
// * stack.Name is populated by reading the name of a folder and even if that folder name could contain something malicious it won't be running in a shell anyway
// * stackPath is concatenated using path.Join which also cleans the path and furthermore it's existence is validated
// * Binaries are executed using their full path and not from $PATH which would be very difficult to exploit anyway
func (h helmfileService) executeHelmfileCommand(token string, instance *model.DeploymentInstance, group *model.Group, ttl uint, operation string) (*exec.Cmd, error) {
	//goland:noinspection GoImportUsedAsName
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		return nil, err
	}

	stackPath := path.Join(h.stackFolder, "/", stack.Name, "/helmfile.yaml")
	if _, err = os.Stat(stackPath); err != nil {
		log.Printf("Stack doesn't exists: %s\n", stackPath)
		return nil, err
	}

	stackParameters, err := h.loadStackParameters(stack.Name)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("/usr/bin/helmfile", "--helm-binary", "/usr/bin/helm", "-f", stackPath, operation) // #nosec
	log.Printf("Command: %s\n", cmd.String())
	configureInstanceEnvironment(token, instance, group, ttl, stackParameters, cmd)

	return cmd, nil
}

type stackParameters map[string]string

func (h helmfileService) loadStackParameters(stackName string) (stackParameters, error) {
	//goland:noinspection GoImportUsedAsName
	path := fmt.Sprintf("%s/%s/parameters/%s.yaml", h.stackFolder, stackName, h.classification)
	data, err := os.ReadFile(path) // #nosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	b, err := decryptYaml(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt stack parameters: %v", err)
	}

	var params stackParameters
	err = yaml.Unmarshal(b, &params)
	if err != nil {
		return nil, err
	}

	return params, nil
}

func configureInstanceEnvironment(accessToken string, instance *model.DeploymentInstance, group *model.Group, ttl uint, stackParameters stackParameters, cmd *exec.Cmd) {
	// TODO: We should only inject what the stack require, currently we just blindly inject IM_ACCESS_TOKEN and others which may not be required by the stack
	// We could probably list the required system parameters in the stacks helmfile and parse those as well as other parameters
	instanceNameEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAME", instance.Name)
	instanceNamespaceEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAMESPACE", group.Name)
	instanceIdEnv := fmt.Sprintf("%s=%d", "INSTANCE_ID", instance.ID)
	deploymentIdEnv := fmt.Sprintf("%s=%d", "DEPLOYMENT_ID", instance.DeploymentID)
	instanceTTLEnv := fmt.Sprintf("%s=%d", "INSTANCE_TTL", ttl)
	instanceHostnameEnv := fmt.Sprintf("%s=%s", "INSTANCE_HOSTNAME", group.Hostname)
	imTokenEnv := fmt.Sprintf("%s=%s", "IM_ACCESS_TOKEN", accessToken)
	homeEnv := fmt.Sprintf("%s=%s", "HOME", "/tmp")
	imCreationTimestamp := fmt.Sprintf("%s=%d", "INSTANCE_CREATION_TIMESTAMP", time.Now().Unix())
	cmd.Env = append(cmd.Env, instanceNameEnv, instanceNamespaceEnv, instanceIdEnv, deploymentIdEnv, instanceTTLEnv, instanceHostnameEnv, imTokenEnv, homeEnv, imCreationTimestamp)

	cmd.Env = injectEnv(cmd.Env, "HOSTNAME")
	cmd.Env = injectEnv(cmd.Env, "AWS_ACCESS_KEY_ID")
	cmd.Env = injectEnv(cmd.Env, "AWS_SECRET_ACCESS_KEY")
	cmd.Env = injectEnv(cmd.Env, "AWS_DEFAULT_REGION")
	cmd.Env = injectEnv(cmd.Env, "AWS_REGION")
	cmd.Env = injectEnv(cmd.Env, "AWS_ROLE_ARN")
	cmd.Env = injectEnv(cmd.Env, "AWS_WEB_IDENTITY_TOKEN_FILE")

	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_SERVICE_PORT")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_PORT")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_PORT_443_TCP_ADDR")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_PORT_443_TCP_PORT")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_PORT_443_TCP_PROTO")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_PORT_443_TCP")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_SERVICE_PORT_HTTPS")
	cmd.Env = injectEnv(cmd.Env, "KUBERNETES_SERVICE_HOST")

	for name, parameter := range instance.Parameters {
		instanceEnv := fmt.Sprintf("%s=%s", name, parameter.Value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}

	for parameter, value := range stackParameters {
		instanceEnv := fmt.Sprintf("%s=%s", parameter, value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}
}

func injectEnv(envs []string, env string) []string {
	if value, exists := os.LookupEnv(env); exists {
		cmdEnv := fmt.Sprintf("%s=%s", env, value)
		envs = append(envs, cmdEnv)
	} else {
		log.Println("WARNING!!! Env not found:", env)
	}

	return envs
}
