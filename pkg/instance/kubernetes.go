package instance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/watch"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"k8s.io/apimachinery/pkg/types"

	"github.com/dhis2-sre/im-manager/pkg/model"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type kubernetesService struct {
	client *kubernetes.Clientset
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewKubernetesService(config *model.ClusterConfiguration) (*kubernetesService, error) {
	client, err := newClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kube client: %v", err)
	}

	return &kubernetesService{client: client}, nil
}

func (ks kubernetesService) GetClient() *kubernetes.Clientset {
	return ks.client
}

func commandExecutor(cmd *exec.Cmd, configuration *model.ClusterConfiguration) (stdout []byte, stderr []byte, err error) {
	if configuration == nil {
		return runCommand(cmd)
	}

	kubeCfg, err := decryptYaml(configuration.KubernetesConfiguration)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt kubernetes config: %v", err)
	}

	file, err := os.CreateTemp("", "kubectl")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		// remove the file even if closing it fails. os.Remove is actually making syscall unlink
		// unlink deletes a name and the file if the name was the last link to the file.
		// If we fail to close the file it will remain in existence until the last file descriptor
		// referring to it is closed. As we don't return the file, this should be done once a GC
		// occurs.

		errC := file.Close()
		errR := os.Remove(file.Name())
		errMsg := joinErrors(err, errC, errR)
		if errMsg != "" {
			err = fmt.Errorf("error handling kube config %q: %s", file.Name(), errMsg)
		}
	}()

	_, err = file.Write(kubeCfg)
	if err != nil {
		return nil, nil, err
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", file.Name()))
	return runCommand(cmd)
}

func joinErrors(errs ...error) string {
	var errMsgs []string
	for _, err := range errs {
		if err != nil {
			errMsgs = append(errMsgs, err.Error())
		}
	}
	return strings.Join(errMsgs, ", ")
}

func runCommand(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}

func newClient(configuration *model.ClusterConfiguration) (*kubernetes.Clientset, error) {
	var restClientConfig *rest.Config
	if configuration != nil && len(configuration.KubernetesConfiguration) > 0 {
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

func (ks kubernetesService) getLogs(instance *model.DeploymentInstance, typeSelector string) (io.ReadCloser, error) {
	pod, err := ks.getPod(instance.ID, typeSelector)
	if err != nil {
		return nil, err
	}

	podLogOptions := v1.PodLogOptions{
		Follow: true,
	}

	return ks.client.
		CoreV1().
		Pods(pod.Namespace).
		GetLogs(pod.Name, &podLogOptions).
		Stream(context.TODO())
}

func (ks kubernetesService) getPod(instanceID uint, typeSelector string) (v1.Pod, error) {
	selector, err := labelSelector(instanceID, typeSelector)
	if err != nil {
		return v1.Pod{}, err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	pods, err := ks.client.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		return v1.Pod{}, fmt.Errorf("error getting pod for instance %d and selector %q: %v", instanceID, selector, err)
	}

	if len(pods.Items) == 0 {
		return v1.Pod{}, errdef.NewNotFound("failed to find pod using the selector: %q", selector)
	}
	if len(pods.Items) > 1 {
		return v1.Pod{}, errdef.NewConflict("multiple pods found using the selector: %q", selector)
	}

	return pods.Items[0], nil
}

// labelSelector returns a selector with requirements for im-id=instanceId and either the im-default
// or im-type=typeSelector.
func labelSelector(instanceID uint, typeSelector string) (string, error) {
	labels := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"im-id": fmt.Sprint(instanceID),
		},
	}
	if typeSelector == "" {
		labels.MatchLabels["im-default"] = "true"
	} else {
		labels.MatchLabels["im-type"] = typeSelector
	}

	sl, err := metav1.LabelSelectorAsSelector(labels)
	if err != nil {
		return "", fmt.Errorf("error creating label selector: %v", err)
	}

	return sl.String(), nil
}

func (ks kubernetesService) restart(instance *model.DeploymentInstance, typeSelector string, instanceStack *model.Stack) error {
	selector, err := labelSelector(instance.ID, typeSelector)
	if err != nil {
		return err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	if instanceStack.KubernetesResource == model.StatefulSetResource {
		statefulSets := ks.client.AppsV1().StatefulSets(instance.GroupName)
		statefulSetsList, err := statefulSets.List(context.TODO(), listOptions)
		if err != nil {
			return err
		}

		statefulSetsItems := statefulSetsList.Items
		if len(statefulSetsItems) == 0 {
			return fmt.Errorf("no deployment found using the selector: %q", selector)
		}
		if len(statefulSetsItems) > 1 {
			return fmt.Errorf("multiple deployments found using the selector: %q", selector)
		}

		statefulSet := statefulSetsItems[0]
		data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format(time.RFC3339))
		_, err = statefulSets.Patch(context.TODO(), statefulSet.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("error restarting %q: %v", statefulSet.Name, err)
		}

		return nil
	}

	if instanceStack.KubernetesResource == model.DeploymentResource {
		deployments := ks.client.AppsV1().Deployments(instance.GroupName)
		deploymentList, err := deployments.List(context.TODO(), listOptions)
		if err != nil {
			return err
		}

		deploymentItems := deploymentList.Items
		if len(deploymentItems) == 0 {
			return fmt.Errorf("no deployment found using the selector: %q", selector)
		}
		if len(deploymentItems) > 1 {
			return fmt.Errorf("multiple deployments found using the selector: %q", selector)
		}

		deployment := deploymentItems[0]
		data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format(time.RFC3339))
		_, err = deployments.Patch(context.TODO(), deployment.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("error restarting %q: %v", deployment.Name, err)
		}

		return nil
	}

	return fmt.Errorf("kubernetes resource not supported: %s", instanceStack.KubernetesResource)
}

func (ks kubernetesService) pause(instance *model.Instance) error {
	err := ks.scale(instance, 0)
	if err != nil {
		return fmt.Errorf("failed to pause instance %q: %v", instance.ID, err)
	}

	return nil
}

func (ks kubernetesService) resume(instance *model.Instance) error {
	err := ks.scale(instance, 1)
	if err != nil {
		return fmt.Errorf("failed to resume instance %q: %v", instance.ID, err)
	}

	return nil
}

func (ks kubernetesService) deletePersistentVolumeClaim_deployment(instance *model.DeploymentInstance) error {
	// TODO: This should be stack metadata
	labelMap := map[string][]string{
		"dhis2":    {"app.kubernetes.io/instance=%s-database", "app.kubernetes.io/instance=%s-redis"},
		"dhis2-db": {"app.kubernetes.io/instance=%s-database"},
	}

	labelPatterns := labelMap[instance.StackName]
	if labelPatterns == nil {
		return nil
	}

	pvcs := ks.client.CoreV1().PersistentVolumeClaims(instance.GroupName)

	for _, pattern := range labelPatterns {
		selector := fmt.Sprintf(pattern, instance.Name)
		listOptions := metav1.ListOptions{LabelSelector: selector}
		list, err := pvcs.List(context.TODO(), listOptions)
		if err != nil {
			return fmt.Errorf("error finding pvcs using selector %q: %v", selector, err)
		}

		if len(list.Items) > 1 {
			return fmt.Errorf("multiple pvcs found using the selector: %q", selector)
		}

		if len(list.Items) == 1 {
			name := list.Items[0].Name
			err := pvcs.Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed to delete pvc: %v", err)
			}
		}
	}

	return nil
}

func (ks kubernetesService) deletePersistentVolumeClaim(instance *model.Instance) error {
	// TODO: This should be stack metadata
	labelMap := map[string][]string{
		"dhis2":    {"app.kubernetes.io/instance=%s-database", "app.kubernetes.io/instance=%s-redis"},
		"dhis2-db": {"app.kubernetes.io/instance=%s-database"},
	}

	labelPatterns := labelMap[instance.StackName]
	if labelPatterns == nil {
		return nil
	}

	pvcs := ks.client.CoreV1().PersistentVolumeClaims(instance.GroupName)

	for _, pattern := range labelPatterns {
		selector := fmt.Sprintf(pattern, instance.Name)
		listOptions := metav1.ListOptions{LabelSelector: selector}
		list, err := pvcs.List(context.TODO(), listOptions)
		if err != nil {
			return fmt.Errorf("error finding pvcs using selector %q: %v", selector, err)
		}

		if len(list.Items) > 1 {
			return fmt.Errorf("multiple pvcs found using the selector: %q", selector)
		}

		if len(list.Items) == 1 {
			name := list.Items[0].Name
			err := pvcs.Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed to delete pvc: %v", err)
			}
		}
	}

	return nil
}

func (ks kubernetesService) watch(group string) (<-chan watch.Event, error) {
	options := metav1.ListOptions{
		LabelSelector: "im=true",
	}
	w, err := ks.client.CoreV1().Pods(group).Watch(context.Background(), options)
	if err != nil {
		return nil, err
	}

	return w.ResultChan(), nil
}

func (ks kubernetesService) scale(instance *model.Instance, replicas uint) error {
	labelSelector := fmt.Sprintf("im-id=%d", instance.ID)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	deployments := ks.client.AppsV1().Deployments(instance.GroupName)
	deploymentList, err := deployments.List(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error finding deployments using selector %q: %v", labelSelector, err)
	}

	for _, d := range deploymentList.Items {
		_, err = scale(deployments, d.Name, int32(replicas))
		if err != nil {
			return err
		}
	}

	sets := ks.client.AppsV1().StatefulSets(instance.GroupName)
	setsList, err := sets.List(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error finding StatefulSets using selector %q: %v", labelSelector, err)
	}

	for _, s := range setsList.Items {
		_, err = scale(sets, s.Name, int32(replicas))
		if err != nil {
			return err
		}
	}

	return nil
}

// scaler allows updating the desired scale of a resource as well as getting the current desired and
// actual scale.
type scaler interface {
	GetScale(ctx context.Context, name string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, name string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

// scale updates the number of replicas on a scaler. The desired number of replicas before scaling
// was updated is returned.
func scale(sc scaler, name string, replicas int32) (int32, error) {
	scale, err := sc.GetScale(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get scale of %q: %v", name, err)
	}

	prevReplicas := scale.Spec.Replicas
	scale.Spec.Replicas = replicas

	_, err = sc.UpdateScale(context.TODO(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to update scale of %q to %d: %v", name, replicas, err)
	}

	return prevReplicas, nil
}

type Status string

const (
	NotDeployed        Status = "NotDeployed"
	Pending            Status = "Pending"
	Booting            Status = "Booting"
	BootingWithRestart Status = "Booting (%d)"
	Running            Status = "Running"
	Error              Status = "Error"
	Terminating        Status = "Terminating"
)

func PodStatus(pod *v1.Pod) (Status, error) {
	switch pod.Status.Phase {
	case v1.PodPending:
		initContainerErrorIndex := slices.IndexFunc(pod.Status.InitContainerStatuses, func(status v1.ContainerStatus) bool {
			return status.State.Waiting != nil && status.State.Waiting.Reason == "ImagePullBackOff"
		})
		if initContainerErrorIndex != -1 {
			status := pod.Status.InitContainerStatuses[initContainerErrorIndex]
			return Status(string(Error) + ": " + status.State.Waiting.Message), nil
		}

		containerErrorIndex := slices.IndexFunc(pod.Status.ContainerStatuses, func(status v1.ContainerStatus) bool {
			return status.State.Waiting != nil && status.State.Waiting.Reason == "ImagePullBackOff"
		})
		if containerErrorIndex != -1 {
			status := pod.Status.ContainerStatuses[containerErrorIndex]
			return Status(string(Error) + ": " + status.State.Waiting.Message), nil
		}
		return Pending, nil
	case v1.PodFailed:
		return Error, nil
	case v1.PodRunning:
		booting := slices.ContainsFunc(pod.Status.Conditions, func(condition v1.PodCondition) bool {
			return condition.Status == v1.ConditionFalse
		})
		if booting {
			initContainerStatuses := pod.Status.InitContainerStatuses
			if initContainerStatuses != nil {
				initContainerError := slices.ContainsFunc(initContainerStatuses, func(status v1.ContainerStatus) bool {
					return status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.Reason == "Error"
				})
				if initContainerError {
					status := fmt.Sprintf(string(BootingWithRestart), initContainerStatuses[0].RestartCount)
					return Status(status), nil
				}
			}

			containerError := slices.ContainsFunc(pod.Status.ContainerStatuses, func(status v1.ContainerStatus) bool {
				return status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.Reason == "Error"
			})
			if containerError {
				status := fmt.Sprintf(string(BootingWithRestart), pod.Status.ContainerStatuses[0].RestartCount)
				return Status(status), nil
			}

			return Booting, nil
		}
		return Running, nil
	}
	return "", fmt.Errorf("failed to get instance status")
}
