package kube

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func (c *Client) Logs(instance *model.DeploymentInstance, typeSelector string) (io.ReadCloser, error) {
	pod, err := c.GetPod(instance.ID, typeSelector)
	if err != nil {
		return nil, err
	}

	podLogOptions := v1.PodLogOptions{
		Follow: true,
		// TODO: Just getting the first container isn't ideal. Ideally we would have an endpoint which returns all containers and allow the user to select one. However this is beyond the scope of the current changes and simply getting the first prevents a 500 error
		Container: pod.Spec.Containers[0].Name,
	}

	stream, err := c.Clientset.
		CoreV1().
		Pods(pod.Namespace).
		GetLogs(pod.Name, &podLogOptions).
		Stream(context.TODO())
	if err != nil {
		if strings.Contains(err.Error(), "ContainerCreating") || strings.Contains(err.Error(), "waiting to start") {
			return nil, errdef.NewConflict("instance is still starting up, logs not available yet")
		}
		return nil, err
	}
	return stream, nil
}

func (c *Client) Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error {
	req := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.RestConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
	})
}

func (c *Client) GetPod(instanceID uint, typeSelector string) (v1.Pod, error) {
	selector, err := labelSelector(instanceID, typeSelector)
	if err != nil {
		return v1.Pod{}, err
	}
	return c.podBySelector(selector)
}

func (c *Client) GetPodByLabels(labels map[string]string) (v1.Pod, error) {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return v1.Pod{}, fmt.Errorf("error creating label selector: %v", err)
	}
	return c.podBySelector(selector.String())
}

func (c *Client) podBySelector(selector string) (v1.Pod, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	pods, err := c.Clientset.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		return v1.Pod{}, fmt.Errorf("error getting pod for selector %q: %v", selector, err)
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
