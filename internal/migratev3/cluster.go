package migratev3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const pollInterval = 5 * time.Second

// Cluster is the namespace of one group on the cluster its instances run on.
type Cluster struct {
	client     *kube.Client
	kubeconfig []byte
	namespace  string
}

func NewCluster(cluster model.Cluster, namespace string) (*Cluster, error) {
	client, err := kube.NewClient(cluster)
	if err != nil {
		return nil, err
	}
	var kubeconfig []byte
	if cluster.Configuration != nil {
		kubeconfig, err = kube.DecryptYaml(cluster.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt the kubeconfig of cluster %q: %v", cluster.Name, err)
		}
	}
	return &Cluster{client: client, kubeconfig: kubeconfig, namespace: namespace}, nil
}

func (c *Cluster) statefulSet(ctx context.Context, name string) (*appsv1.StatefulSet, error) {
	return c.client.Clientset.AppsV1().StatefulSets(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Cluster) deployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	return c.client.Clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Cluster) deploymentsBySelector(ctx context.Context, selector string) ([]appsv1.Deployment, error) {
	list, err := c.client.Clientset.AppsV1().Deployments(c.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *Cluster) scaleStatefulSet(ctx context.Context, name string, replicas int32) error {
	statefulSets := c.client.Clientset.AppsV1().StatefulSets(c.namespace)
	scale, err := statefulSets.GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to read the scale of statefulset %q: %v", name, err)
	}
	scale.Spec.Replicas = replicas
	if _, err := statefulSets.UpdateScale(ctx, name, scale, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to scale statefulset %q to %d: %v", name, replicas, err)
	}
	return nil
}

func (c *Cluster) scaleDeployment(ctx context.Context, name string, replicas int32) error {
	deployments := c.client.Clientset.AppsV1().Deployments(c.namespace)
	scale, err := deployments.GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to read the scale of deployment %q: %v", name, err)
	}
	scale.Spec.Replicas = replicas
	if _, err := deployments.UpdateScale(ctx, name, scale, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to scale deployment %q to %d: %v", name, replicas, err)
	}
	return nil
}

// readyPod waits for a running pod matching the selector whose containers are all ready.
func (c *Cluster) readyPod(ctx context.Context, selector string, timeout time.Duration) (v1.Pod, error) {
	var ready v1.Pod
	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := c.client.Clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp == nil && pod.Status.Phase == v1.PodRunning && podReady(pod) {
				ready = pod
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return v1.Pod{}, fmt.Errorf("no ready pod matching %q: %v", selector, err)
	}
	return ready, nil
}

func podReady(pod v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

// waitForNoPods waits until no pod matches the selector, so a scale down has actually stopped
// writes before a snapshot is taken.
func (c *Cluster) waitForNoPods(ctx context.Context, selector string, timeout time.Duration) error {
	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := c.client.Clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			// Pods of completed jobs carry the same labels and never go away on their own.
			if pod.Status.Phase != v1.PodSucceeded && pod.Status.Phase != v1.PodFailed {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("pods matching %q are still running: %v", selector, err)
	}
	return nil
}

// deletePVC deletes the claim and waits until it is gone, since a release about to be installed
// may create one of the same name.
func (c *Cluster) deletePVC(ctx context.Context, name string, timeout time.Duration) error {
	claims := c.client.Clientset.CoreV1().PersistentVolumeClaims(c.namespace)
	if err := claims.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pvc %q: %v", name, err)
	}
	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := claims.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("pvc %q is still there: %v", name, err)
	}
	return nil
}

// jobSucceeded reports whether the job has completed, and fails once it has given up.
func (c *Cluster) jobSucceeded(ctx context.Context, name string) (bool, error) {
	job, err := c.client.Clientset.BatchV1().Jobs(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, condition := range job.Status.Conditions {
		if condition.Status != v1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobComplete:
			return true, nil
		case batchv1.JobFailed:
			return false, fmt.Errorf("job %q failed: %s", name, condition.Message)
		}
	}
	return false, nil
}

// deletePVCsBySelector deletes every claim matching the selector.
func (c *Cluster) deletePVCsBySelector(ctx context.Context, selector string) error {
	_, err := c.client.DeletePVCs(ctx, c.namespace, []string{selector})
	return err
}

func (c *Cluster) exec(ctx context.Context, pod v1.Pod, container string, command []string, stdout io.Writer) error {
	var stderr bytes.Buffer
	if err := c.client.Exec(ctx, c.namespace, pod.Name, container, command, stdout, &stderr); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}
	return nil
}

// uninstall removes a helm release, treating one that is already gone as removed.
func (c *Cluster) uninstall(ctx context.Context, release string) error {
	return c.helm(ctx, "uninstall", release, "--namespace", c.namespace, "--ignore-not-found", "--wait", "--timeout", "10m")
}

func (c *Cluster) releaseExists(ctx context.Context, release string) (bool, error) {
	err := c.helm(ctx, "status", release, "--namespace", c.namespace)
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "release: not found") {
		return false, nil
	}
	return false, err
}

func (c *Cluster) helm(ctx context.Context, arguments ...string) error {
	// The image runs as a user without a writable home, where helm keeps its configuration and cache.
	home, err := os.MkdirTemp("", "migrate-v3-helm")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(home) }()

	command := exec.CommandContext(ctx, "helm", arguments...)
	command.Env = append(os.Environ(), "HELM_CONFIG_HOME="+home, "HELM_CACHE_HOME="+home, "HELM_DATA_HOME="+home)
	if c.kubeconfig != nil {
		file, err := os.CreateTemp("", "migrate-v3-kubeconfig")
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(file.Name()) }()
		if _, err := file.Write(c.kubeconfig); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		command.Env = append(command.Env, "KUBECONFIG="+file.Name())
	}
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("helm %v: %v: %s", arguments, err, output.String())
	}
	return nil
}
