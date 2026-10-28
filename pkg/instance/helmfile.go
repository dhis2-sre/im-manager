package instance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewHelmfileService(logger *slog.Logger, stackService stackService, stackFolder string, classification string) (helmfileService, error) {
	helmfileBinary, err := resolveBinary("helmfile")
	if err != nil {
		return helmfileService{}, err
	}
	helmBinary, err := resolveBinary("helm")
	if err != nil {
		return helmfileService{}, err
	}
	logger.Info("Resolved helmfile service binaries", "helmfile", helmfileBinary, "helm", helmBinary)

	return helmfileService{
		logger:         logger,
		stackService:   stackService,
		stackFolder:    stackFolder,
		classification: classification,
		helmfileBinary: helmfileBinary,
		helmBinary:     helmBinary,
	}, nil
}

// resolveBinary returns an absolute path to name, resolved once at startup via PATH.
func resolveBinary(name string) (string, error) {
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s binary not found in PATH: %w", name, err)
	}
	return resolved, nil
}

type helmfileService struct {
	logger         *slog.Logger
	stackService   stackService
	stackFolder    string
	classification string
	helmfileBinary string
	helmBinary     string
}

type stackService interface {
	Find(name string) (*stack.Stack, error)
}

func (h helmfileService) sync(ctx context.Context, token string, instance *model.DeploymentInstance, group *model.Group, ttl uint, extraEnv map[string]string) (*exec.Cmd, error) {
	return h.executeHelmfileCommand(ctx, token, instance, group, ttl, extraEnv, "sync")
}

// uninstall removes the instance's release by name. Unlike a helmfile destroy it renders nothing,
// so an instance whose parameters no longer render, or whose release was never installed, can still
// be destroyed.
func (h helmfileService) uninstall(ctx context.Context, instance *model.DeploymentInstance, group *model.Group) (*exec.Cmd, error) {
	//goland:noinspection GoImportUsedAsName
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		return nil, err
	}

	release := stack.ReleaseName(instanceName(instance, group))
	cmd := exec.CommandContext(ctx, h.helmBinary, "uninstall", release, "--namespace", group.Namespace, "--ignore-not-found") // #nosec
	cmd.WaitDelay = 10 * time.Second
	h.logger.InfoContext(ctx, "Executing helm command", "command", cmd.String())
	cmd.Env = h.clusterEnvironment(ctx, cmd.Env)

	return cmd, nil
}

// instanceName is the name the instance's kubernetes resources and helm release are named after.
func instanceName(instance *model.DeploymentInstance, group *model.Group) string {
	return fmt.Sprintf("%s-%d", instance.Name, group.ID)
}

// **Security considerations**
// * No shell is invoked - So && and ; doesn't work even if an attacker managed to inject them
// * STACKS_FOLDER is configured on the host so if that can be tampered the attacker already has access
// * stack.Name is populated by reading the name of a folder and even if that folder name could contain something malicious it won't be running in a shell anyway
// * stackPath is concatenated using path.Join which also cleans the path and furthermore it's existence is validated
// * Binaries are executed using an absolute path resolved once at startup (via exec.LookPath or an explicit env override); PATH is not consulted per request
func (h helmfileService) executeHelmfileCommand(ctx context.Context, token string, instance *model.DeploymentInstance, group *model.Group, ttl uint, extraEnv map[string]string, operation string) (*exec.Cmd, error) {
	//goland:noinspection GoImportUsedAsName
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		return nil, err
	}

	stackPath := path.Join(h.stackFolder, "/", stack.Name, "/helmfile.yaml.gotmpl")
	if _, err = os.Stat(stackPath); err != nil {
		return nil, err
	}

	stackParameters, err := h.loadStackParameters(stack.Name)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, h.helmfileBinary, "--helm-binary", h.helmBinary, "-f", stackPath, operation) // #nosec
	// Output is collected through pipes, so Wait blocks until every writer closes them. A helm hook
	// job that outlives the kill would otherwise hold the deadline open indefinitely.
	cmd.WaitDelay = 10 * time.Second
	h.logger.InfoContext(ctx, "Executing helmfile command", "command", cmd.String())
	h.configureInstanceEnvironment(ctx, token, instance, group, ttl, stackParameters, extraEnv, cmd)

	return cmd, nil
}

// clusterEnvironment is what helm needs to reach a cluster: a writable home, the AWS credentials an
// EKS kubeconfig authenticates with, and the service address of the cluster IM runs in.
func (h helmfileService) clusterEnvironment(ctx context.Context, env []string) []string {
	env = append(env, fmt.Sprintf("%s=%s", "HOME", "/tmp"))

	env = h.injectEnv(ctx, env, "AWS_ACCESS_KEY_ID")
	env = h.injectEnv(ctx, env, "AWS_SECRET_ACCESS_KEY")
	env = h.injectEnv(ctx, env, "AWS_DEFAULT_REGION")
	env = h.injectEnv(ctx, env, "AWS_REGION")
	env = h.injectEnv(ctx, env, "AWS_ROLE_ARN")
	env = h.injectEnv(ctx, env, "AWS_WEB_IDENTITY_TOKEN_FILE")

	env = h.injectEnv(ctx, env, "KUBERNETES_SERVICE_PORT")
	env = h.injectEnv(ctx, env, "KUBERNETES_PORT")
	env = h.injectEnv(ctx, env, "KUBERNETES_SERVICE_PORT_HTTPS")
	env = h.injectEnv(ctx, env, "KUBERNETES_SERVICE_HOST")

	return env
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

	b, err := kube.DecryptYaml(data)
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

func (h helmfileService) configureInstanceEnvironment(ctx context.Context, accessToken string, instance *model.DeploymentInstance, group *model.Group, ttl uint, stackParameters stackParameters, extraEnv map[string]string, cmd *exec.Cmd) {
	// INSTANCE_NAME: unique name for K8s/Helm (release names, services). INSTANCE_PATH_NAME: deployment name for URL path/context (group-based subdomain already makes URL unique).
	// IM_ACCESS_TOKEN is kept for stacks that need IM API access (e.g. im-job-runner). Seed stacks use per-deployment signed URLs via extraEnv instead.
	instanceNameEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAME", instanceName(instance, group))
	instancePathNameEnv := fmt.Sprintf("%s=%s", "INSTANCE_PATH_NAME", instance.Name)
	instanceNamespaceEnv := fmt.Sprintf("%s=%s", "INSTANCE_NAMESPACE", group.Namespace)
	instanceIdEnv := fmt.Sprintf("%s=%d", "INSTANCE_ID", instance.ID)
	deploymentIdEnv := fmt.Sprintf("%s=%d", "DEPLOYMENT_ID", instance.DeploymentID)
	instanceTTLEnv := fmt.Sprintf("%s=%d", "INSTANCE_TTL", ttl)
	instanceHostnameEnv := fmt.Sprintf("%s=%s", "INSTANCE_HOSTNAME", group.Hostname)
	imTokenEnv := fmt.Sprintf("%s=%s", "IM_ACCESS_TOKEN", accessToken)
	imCreationTimestamp := fmt.Sprintf("%s=%d", "INSTANCE_CREATION_TIMESTAMP", time.Now().Unix())
	cmd.Env = append(cmd.Env, instanceNameEnv, instancePathNameEnv, instanceNamespaceEnv, instanceIdEnv, deploymentIdEnv, instanceTTLEnv, instanceHostnameEnv, imTokenEnv, imCreationTimestamp)

	cmd.Env = h.injectEnv(ctx, cmd.Env, "HOSTNAME")
	cmd.Env = h.clusterEnvironment(ctx, cmd.Env)

	ingressClass, err := kube.DiscoverIngressClass(ctx, group.Cluster)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to discover ingress class", "error", err)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("INGRESS_CLASS=%s", ingressClass))

	certIssuer, err := kube.DiscoverCertIssuer(ctx, group.Cluster)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to discover cert issuer", "error", err)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("CERT_ISSUER=%s", certIssuer))

	for name, parameter := range instance.Parameters {
		instanceEnv := fmt.Sprintf("%s=%s", name, parameter.Value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}

	for parameter, value := range stackParameters {
		instanceEnv := fmt.Sprintf("%s=%s", parameter, value)
		cmd.Env = append(cmd.Env, instanceEnv)
	}

	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
}

func (h helmfileService) injectEnv(ctx context.Context, envs []string, env string) []string {
	if value, exists := os.LookupEnv(env); exists {
		cmdEnv := fmt.Sprintf("%s=%s", env, value)
		envs = append(envs, cmdEnv)
	} else {
		h.logger.WarnContext(ctx, "Environment variable not found", "key", env)
	}

	return envs
}
