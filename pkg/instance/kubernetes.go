package instance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
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

func NewKubernetesService(config *models.ClusterConfiguration) (*kubernetesService, error) {
	client, err := newClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kube client: %v", err)
	}

	return &kubernetesService{client: client}, nil
}

func commandExecutor(cmd *exec.Cmd, configuration *models.ClusterConfiguration) (stdout []byte, stderr []byte, err error) {
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

func (ks kubernetesService) getLogs(instance *model.Instance, selector string) (io.ReadCloser, error) {
	pod, err := ks.getPod(instance, selector)
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

func (ks kubernetesService) getPod(instance *model.Instance, selector string) (v1.Pod, error) {
	var labelSelector string
	if selector == "" {
		labelSelector = fmt.Sprintf("im-id=%d", instance.ID)
	} else {
		labelSelector = fmt.Sprintf("im-%s-id=%d", selector, instance.ID)
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	podList, err := ks.client.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		return v1.Pod{}, fmt.Errorf("error getting pod for instance %d and selector %q: %v", instance.ID, selector, err)
	}

	if len(podList.Items) > 1 {
		return v1.Pod{}, fmt.Errorf("multiple pods found using the selector: %q", labelSelector)
	}

	return podList.Items[0], nil
}

func (ks kubernetesService) restart(instance *model.Instance) error {
	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", instance.Name)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	deployments := ks.client.AppsV1().Deployments(instance.GroupName)
	deploymentList, err := deployments.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	items := deploymentList.Items
	if len(items) > 1 {
		return fmt.Errorf("multiple deployments found using the selector: %q", labelSelector)
	}
	if len(items) < 1 {
		return fmt.Errorf("no deployment found using the selector: %q", labelSelector)
	}

	name := items[0].Name

	// Scale down
	prevReplicas, err := scale(deployments, name, 0)
	if err != nil {
		return err
	}

	// Scale up
	_, err = scale(deployments, name, prevReplicas)

	return err
}

func (ks kubernetesService) pause(instance *model.Instance) error {
	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", instance.Name)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	deployments := ks.client.AppsV1().Deployments(instance.GroupName)
	deploymentList, err := deployments.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	items := deploymentList.Items
	if len(items) > 1 {
		return fmt.Errorf("multiple deployments found using the selector: %q", labelSelector)
	}
	if len(items) < 1 {
		return fmt.Errorf("no deployment found using the selector: %q", labelSelector)
	}

	_, err = scale(deployments, items[0].Name, 0)
	if err != nil {
		return err
	}

	labelSelector = fmt.Sprintf("app.kubernetes.io/instance=%s-database", instance.Name)
	listOptions = metav1.ListOptions{LabelSelector: labelSelector}

	sets := ks.client.AppsV1().StatefulSets(instance.GroupName)
	setsList, err := sets.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	setsItems := setsList.Items
	if len(setsItems) > 1 {
		return fmt.Errorf("multiple StatefulSets found using the selector: %q", labelSelector)
	}
	if len(setsItems) < 1 {
		return fmt.Errorf("no StatefulSet found using the selector: %q", labelSelector)
	}

	_, err = scale(sets, setsItems[0].Name, 0)

	return err
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
		return 0, err
	}

	prevReplicas := scale.Spec.Replicas
	scale.Spec.Replicas = replicas

	_, err = sc.UpdateScale(context.TODO(), name, scale, metav1.UpdateOptions{})

	return prevReplicas, err
}
