package inttest

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/orlangure/gnomock/preset/k3s"
	"k8s.io/client-go/kubernetes"

	"github.com/orlangure/gnomock"
	"github.com/stretchr/testify/require"
)

// SetupK8s creates a K8s container (using k3s).
func SetupK8s(t *testing.T) *K8sClient {
	t.Helper()

	container, err := gnomock.Start(
		k3s.Preset(
			k3s.WithVersion("v1.33.2-k3s1"),
			func(p *k3s.P) {
				p.K3sServerFlags = []string{"--debug"}
			},
		),
		gnomock.WithDebugMode(),
	)
	require.NoError(t, err, "failed to start k3s")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop k3s") })

	k8sConfig, err := k3s.Config(container)
	require.NoError(t, err, "failed to get k3s config from container")
	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	require.NoError(t, err, "failed to create k8s client")

	k3sConfigBytes, err := k3s.ConfigBytes(container)
	require.NoError(t, err, "failed to get k3s config from container as bytes")

	return &K8sClient{
		Client: k8sClient,
		Config: k3sConfigBytes,
	}
}

// K8sClient allows making requests to K8s. It does so by wrapping a kubernetes.Clientset. Access
// the actual Clientset for specific use cases where our defaults don't work.
type K8sClient struct {
	Client *kubernetes.Clientset
	Config []byte
}

func (k K8sClient) AssertPodIsNotRunning(t *testing.T, instance model.DeploymentInstance) {
	pods, err := k.Client.CoreV1().Pods(instance.Group.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=" + fmt.Sprintf("%s-%d", instance.Name, instance.Group.ID),
	})
	require.NoError(t, err)

	require.Len(t, pods.Items, 0)
}

func (k K8sClient) AssertPodIsReady(t *testing.T, instance model.DeploymentInstance, namePostfix string, timeoutInSeconds time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	podName := fmt.Sprintf("%s-%d%s", instance.Name, instance.Group.ID, namePostfix)
	watch, err := k.Client.CoreV1().Pods(instance.Group.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=" + podName,
	})
	require.NoErrorf(t, err, "failed to find pod for instance %q", podName)

	t.Log("Waiting for:", podName)
	timeout := timeoutInSeconds * time.Second
	tm := time.NewTimer(timeout)
	defer tm.Stop()
	for {
		select {
		case <-tm.C:
			assert.Fail(t, "timed out waiting on pod: "+instance.Group.Namespace+"/"+podName)
			cancel()

			k.logAllPods(t)
			k.logTargetPodDetails(t, namespace, instance)

			return
		case event := <-watch.ResultChan():
			pod, ok := event.Object.(*v1.Pod)
			t.Log("Received pod updated event...")
			if !ok {
				assert.Failf(t, "failed to get pod event", "want pod event instead got %T", event.Object)
				if !tm.Stop() {
					<-tm.C
				}
				cancel()
				return
			}

			if pod.Status.Phase == v1.PodRunning {
				conditions := pod.Status.Conditions
				index := slices.IndexFunc(conditions, func(condition v1.PodCondition) bool {
					return condition.Type == v1.PodReady
				})
				readyCondition := conditions[index]
				if readyCondition.Status == "True" {
					t.Logf("pod for instance %q is running", podName)
					if !tm.Stop() {
						<-tm.C
					}
					cancel()
					return
				}
			}
		}
	}
}

func (k K8sClient) logAllPods(t *testing.T) {
	pods, err := k.Client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Logf("Failed to retrieve pods for debugging: %v", err)
	}

	t.Log("=== All pods ===")
	for _, pod := range pods.Items {
		t.Logf("Namespace: %s, Name: %s, Phase: %s, Ready: %s", pod.Namespace, pod.Name, pod.Status.Phase, getPodReadyStatus(pod))
	}
	t.Log("=== End of all pods ===")
}

// getPodReadyStatus returns the ready status of a pod
func getPodReadyStatus(pod v1.Pod) string {
	conditions := pod.Status.Conditions
	index := slices.IndexFunc(conditions, func(condition v1.PodCondition) bool {
		return condition.Type == v1.PodReady
	})
	if index >= 0 {
		return string(conditions[index].Status)
	}
	return "Unknown"
}

// logTargetPodDetails logs detailed information about the target pod we're waiting for
func (k K8sClient) logTargetPodDetails(t *testing.T, namespace string, instance string) {
	targetPods, err := k.Client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=" + instance,
	})
	if err != nil {
		t.Logf("Failed to retrieve target pod for instance %q: %v", instance, err)
		return
	}

	if len(targetPods.Items) == 0 {
		t.Logf("No pods found for instance %q in namespace %q", instance, namespace)
		return
	}

	t.Logf("=== Target pod details for instance %q ===", instance)
	for _, pod := range targetPods.Items {
		t.Logf("Name: %s, Namespace: %s, Phase: %s, Ready: %s", pod.Name, pod.Namespace, pod.Status.Phase, getPodReadyStatus(pod))

		if len(pod.Status.Conditions) > 0 {
			k.logConditions(t, pod)
		}

		if len(pod.Status.ContainerStatuses) > 0 {
			k.logStatuses(t, pod)
		}

		k.logEvents(t, namespace, pod)
	}
	t.Log("=== End of target pod details ===")
}

func (k K8sClient) logEvents(t *testing.T, namespace string, pod v1.Pod) {
	events, err := k.Client.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + pod.Name,
	})
	if err != nil {
		t.Logf("Failed to retrieve events for pod %s: %v", pod.Name, err)
		t.Fail()
	}

	if len(events.Items) > 0 {
		t.Log("Recent Events:")
		for _, event := range events.Items {
			t.Logf("  - Type: %s, Reason: %s, Message: %s (Count: %d, Last: %s)",
				event.Type, event.Reason, event.Message, event.Count, event.LastTimestamp)
		}
	}
}

func (k K8sClient) logStatuses(t *testing.T, pod v1.Pod) {
	t.Log("Container Statuses:")
	for _, cs := range pod.Status.ContainerStatuses {
		t.Logf("  - Name: %s, Ready: %t, RestartCount: %d", cs.Name, cs.Ready, cs.RestartCount)
		if cs.State.Waiting != nil {
			t.Logf("    Waiting - Reason: %s, Message: %s", cs.State.Waiting.Reason, cs.State.Waiting.Message)
		}
		if cs.State.Running != nil {
			t.Logf("    Running since: %s", cs.State.Running.StartedAt)
		}
		if cs.State.Terminated != nil {
			t.Logf("    Terminated - Reason: %s, Message: %s, Exit Code: %d",
				cs.State.Terminated.Reason, cs.State.Terminated.Message, cs.State.Terminated.ExitCode)
		}
	}
}

func (k K8sClient) logConditions(t *testing.T, pod v1.Pod) {
	t.Log("Conditions:")
	for _, condition := range pod.Status.Conditions {
		t.Logf("  - Type: %s, Status: %s, Reason: %s, Message: %s",
			condition.Type, condition.Status, condition.Reason, condition.Message)
	}
}
