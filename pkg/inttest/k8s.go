package inttest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/orlangure/gnomock/preset/k3s"
	"k8s.io/client-go/kubernetes"

	"github.com/orlangure/gnomock"
	"github.com/stretchr/testify/require"
)

// SetupK8s creates an K8s container (using k3s).
func SetupK8s(t *testing.T) *K8sClient {
	t.Helper()

	container, err := gnomock.Start(
		k3s.Preset(
			k3s.WithVersion("v1.26.7-k3s1"),
			func(p *k3s.P) {
				p.K3sServerFlags = []string{"--debug"} // TODO(ivo) remove this flag before merging?
			},
		),
		gnomock.WithDebugMode(), // TODO(ivo) remove this config before merging?
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

func (k K8sClient) AssertPodIsReady(t *testing.T, group string, instance string) {
	ctx, cancel := context.WithCancel(context.Background())
	watch, err := k.Client.CoreV1().Pods(group).Watch(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=" + instance,
	})
	require.NoErrorf(t, err, "failed to find pod for instance %q", instance)

	timeout := 20 * time.Second
	tm := time.NewTimer(timeout)
	defer tm.Stop()
	for {
		select {
		case <-tm.C:
			assert.Fail(t, "timed out waiting on pod")
			cancel()
			return
		case event := <-watch.ResultChan():
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				assert.Failf(t, "failed to get pod event", "want pod event instead got %T", event.Object)
				if !tm.Stop() {
					<-tm.C
				}
				cancel()
				return
			}

			t.Logf("watching pod conditions: %#v\n", pod.Status.Conditions)
			for _, condition := range pod.Status.Conditions {
				if condition.Type == v1.PodReady {
					t.Logf("pod for instance %q is ready", instance)
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
