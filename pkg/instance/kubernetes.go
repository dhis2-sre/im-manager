package instance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"

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
func NewKubernetesService(config model.Cluster) (*kubernetesService, error) {
	client, err := newClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kube client: %v", err)
	}

	return &kubernetesService{client: client}, nil
}

func commandExecutor(cmd *exec.Cmd, cluster model.Cluster) (stdout []byte, stderr []byte, err error) {
	if cluster.Configuration == nil {
		return runCommand(cmd)
	}

	kubeCfg, err := decryptYaml(cluster.Configuration)
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

func newMetricsClient(cluster model.Cluster) (*metricsv1beta1.Clientset, error) {
	restClientConfig, err := newRestConfig(cluster)
	if err != nil {
		return nil, err
	}

	metricsClient, err := metricsv1beta1.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	return metricsClient, nil
}

func newClient(configuration model.Cluster) (*kubernetes.Clientset, error) {
	restClientConfig, err := newRestConfig(configuration)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func newRestConfig(cluster model.Cluster) (*rest.Config, error) {
	var restClientConfig *rest.Config
	if cluster.Configuration != nil {
		kubeCfg, err := decryptYaml(cluster.Configuration)
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

	return restClientConfig, nil
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

	// 'Evicted' pods are safe to filter out, as for each pod
	// there will be another pod created in a different state inplace of it.
	pods.Items = slices.DeleteFunc(pods.Items, func(pod v1.Pod) bool {
		return pod.Status.Phase == v1.PodFailed && pod.Status.Reason == "Evicted"
	})

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
		statefulSets := ks.client.AppsV1().StatefulSets(instance.Group.Namespace)
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
		deployments := ks.client.AppsV1().Deployments(instance.Group.Namespace)
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

func (ks kubernetesService) pause(instance *model.DeploymentInstance) error {
	err := ks.scale(instance, 0)
	if err != nil {
		return fmt.Errorf("failed to pause instance %q: %v", instance.ID, err)
	}

	return nil
}

func (ks kubernetesService) resume(instance *model.DeploymentInstance) error {
	err := ks.scale(instance, 1)
	if err != nil {
		return fmt.Errorf("failed to resume instance %q: %v", instance.ID, err)
	}

	return nil
}

func (ks kubernetesService) deletePersistentVolumeClaim(instance *model.DeploymentInstance) error {
	// TODO: This should be stack metadata
	labelMap := map[string][]string{
		"dhis2":      {"app.kubernetes.io/instance=%s-database", "app.kubernetes.io/instance=%s-redis"},
		"dhis2-core": {"app.kubernetes.io/instance=%s", "app.kubernetes.io/instance=%s-minio"},
		"dhis2-db":   {"app.kubernetes.io/instance=%s-database"},
	}

	labelPatterns := labelMap[instance.StackName]
	if labelPatterns == nil {
		return nil
	}

	pvcs := ks.client.CoreV1().PersistentVolumeClaims(instance.Group.Namespace)

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

func (ks kubernetesService) scale(instance *model.DeploymentInstance, replicas uint) error {
	labelSelector := fmt.Sprintf("im-id=%d", instance.ID)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	deployments := ks.client.AppsV1().Deployments(instance.Group.Namespace)
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

	sets := ks.client.AppsV1().StatefulSets(instance.Group.Namespace)
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

type ClusterResources struct {
	CPU        string
	Memory     string
	Autoscaled bool
	Nodes      int
}

func FindResources(cluster model.Cluster) (ClusterResources, error) {
	client, err := newClient(cluster)
	if err != nil {
		return ClusterResources{}, err
	}

	metricsClient, err := newMetricsClient(cluster)
	if err != nil {
		return ClusterResources{}, err
	}

	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ClusterResources{}, err
	}

	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ClusterResources{}, err
	}

	var totalCPUUsed, totalMemUsed, totalCPUAlloc, totalMemAlloc resource.Quantity

	for _, node := range nodes.Items {
		name := node.Name
		allocCPU := node.Status.Allocatable["cpu"]
		allocMem := node.Status.Allocatable["memory"]

		var usedCPU, usedMem resource.Quantity
		for _, metric := range nodeMetrics.Items {
			if metric.Name == name {
				usedCPU = metric.Usage["cpu"]
				usedMem = metric.Usage["memory"]
				break
			}
		}
		/*
			cpuPercent := percent(usedCPU.MilliValue(), allocCPU.MilliValue())
			memPercent := percent(usedMem.Value(), allocMem.Value())

			fmt.Printf("- %s\n", name)
			fmt.Printf("  CPU: %s / %s (%.1f%%)\n", usedCPU.String(), allocCPU.String(), cpuPercent)
			fmt.Printf("  MEM: %s / %s (%.1f%%)\n", usedMem.String(), allocMem.String(), memPercent)
		*/
		totalCPUUsed.Add(usedCPU)
		totalMemUsed.Add(usedMem)
		totalCPUAlloc.Add(allocCPU)
		totalMemAlloc.Add(allocMem)
	}

	clusterCPUPercent := percent(totalCPUUsed.MilliValue(), totalCPUAlloc.MilliValue())
	clusterMemPercent := percent(totalMemUsed.Value(), totalMemAlloc.Value())

	return ClusterResources{
		CPU:    fmt.Sprintf("%.1f%%", clusterCPUPercent),
		Memory: fmt.Sprintf("%.1f%%", clusterMemPercent),
		Nodes:  len(nodes.Items),
	}, nil
}

func percent(used, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return (float64(used) / float64(total)) * 100
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
