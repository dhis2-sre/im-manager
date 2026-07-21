package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c *Client) RestartStatefulSet(instance *model.DeploymentInstance, componentName string) error {
	selector, err := labelSelector(instance.ID, componentName)
	if err != nil {
		return err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	statefulSets := c.Clientset.AppsV1().StatefulSets(instance.Group.Namespace)
	statefulSetsList, err := statefulSets.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	statefulSetsItems := statefulSetsList.Items
	if len(statefulSetsItems) == 0 {
		return fmt.Errorf("no stateful set found using the selector: %q", selector)
	}
	if len(statefulSetsItems) > 1 {
		return fmt.Errorf("multiple stateful sets found using the selector: %q", selector)
	}

	statefulSet := statefulSetsItems[0]
	data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format(time.RFC3339))
	_, err = statefulSets.Patch(context.TODO(), statefulSet.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error restarting %q: %v", statefulSet.Name, err)
	}

	return nil
}

func (c *Client) RestartDeployment(instance *model.DeploymentInstance, componentName string) error {
	selector, err := labelSelector(instance.ID, componentName)
	if err != nil {
		return err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	deployments := c.Clientset.AppsV1().Deployments(instance.Group.Namespace)
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

func (c *Client) Pause(instance *model.DeploymentInstance) error {
	err := c.scale(instance, 0)
	if err != nil {
		return fmt.Errorf("failed to pause instance %d: %v", instance.ID, err)
	}

	return nil
}

func (c *Client) Resume(instance *model.DeploymentInstance) error {
	err := c.scale(instance, 1)
	if err != nil {
		return fmt.Errorf("failed to resume instance %d: %v", instance.ID, err)
	}

	return nil
}

func (c *Client) DeletePersistentVolumeClaim(instance *model.DeploymentInstance) error {
	// TODO: This should be stack metadata
	labelMap := map[string][]string{
		"dhis2":      {"app.kubernetes.io/instance=%s-database", "app.kubernetes.io/instance=%s-redis"},
		"dhis2-core": {"app.kubernetes.io/instance=%s", "app.kubernetes.io/instance=%s-minio"},
		"dhis2-db":   {"app.kubernetes.io/instance=%s-database"},
		"minio":      {"app.kubernetes.io/instance=%s-minio"},
	}

	labelPatterns := labelMap[instance.StackName]
	if labelPatterns == nil {
		return nil
	}

	pvcs := c.Clientset.CoreV1().PersistentVolumeClaims(instance.Group.Namespace)

	for _, pattern := range labelPatterns {
		selector := fmt.Sprintf(pattern, fmt.Sprintf("%s-%d", instance.Name, instance.Group.ID))
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

func (c *Client) scale(instance *model.DeploymentInstance, replicas int32) error {
	labelSelector := fmt.Sprintf("im-id=%d", instance.ID)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	deployments := c.Clientset.AppsV1().Deployments(instance.Group.Namespace)
	deploymentList, err := deployments.List(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error finding deployments using selector %q: %v", labelSelector, err)
	}

	for _, d := range deploymentList.Items {
		_, err = scale(deployments, d.Name, replicas)
		if err != nil {
			return err
		}
	}

	sets := c.Clientset.AppsV1().StatefulSets(instance.Group.Namespace)
	setsList, err := sets.List(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error finding StatefulSets using selector %q: %v", labelSelector, err)
	}

	for _, s := range setsList.Items {
		_, err = scale(sets, s.Name, replicas)
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
