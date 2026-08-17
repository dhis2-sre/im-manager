package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func (c *Client) RestartStatefulSet(ctx context.Context, instance *model.DeploymentInstance, componentName string) error {
	selector, err := labelSelector(instance.ID, componentName)
	if err != nil {
		return err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	statefulSets := c.Clientset.AppsV1().StatefulSets(instance.Group.Namespace)
	statefulSetsList, err := statefulSets.List(ctx, listOptions)
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
	_, err = statefulSets.Patch(ctx, statefulSet.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error restarting %q: %v", statefulSet.Name, err)
	}

	return nil
}

func (c *Client) RestartDeployment(ctx context.Context, instance *model.DeploymentInstance, componentName string) error {
	selector, err := labelSelector(instance.ID, componentName)
	if err != nil {
		return err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	deployments := c.Clientset.AppsV1().Deployments(instance.Group.Namespace)
	deploymentList, err := deployments.List(ctx, listOptions)
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
	_, err = deployments.Patch(ctx, deployment.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error restarting %q: %v", deployment.Name, err)
	}

	return nil
}

func (c *Client) Pause(ctx context.Context, instance *model.DeploymentInstance) error {
	err := c.scale(ctx, instance, 0)
	if err != nil {
		return fmt.Errorf("failed to pause instance %d: %v", instance.ID, err)
	}

	return nil
}

func (c *Client) Resume(ctx context.Context, instance *model.DeploymentInstance) error {
	err := c.scale(ctx, instance, 1)
	if err != nil {
		return fmt.Errorf("failed to resume instance %d: %v", instance.ID, err)
	}

	return nil
}

var cnpgClusterResource = schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}

// RestartCNPGCluster triggers a rolling restart of a CloudNativePG cluster by stamping the
// standard restartedAt annotation on the Cluster resource, the same mechanism as the cnpg kubectl
// plugin's restart command. Patching the operator-managed pods directly would fight the operator.
func (c *Client) RestartCNPGCluster(ctx context.Context, namespace, name string) error {
	data := fmt.Sprintf(`{"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}`, time.Now().Format(time.RFC3339))
	_, err := c.Dynamic.Resource(cnpgClusterResource).Namespace(namespace).Patch(ctx, name, types.MergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error restarting %q: %v", name, err)
	}
	return nil
}

var dorisClusterResource = schema.GroupVersionResource{Group: "doris.selectdb.com", Version: "v1", Resource: "dorisclusters"}

// RestartDorisTier triggers a rolling restart of one tier of a Doris cluster, "fe" or "be", by
// stamping the operator's restart annotation for that tier on the DorisCluster resource. Frontends
// and backends roll independently, which is why the tier is a parameter rather than both being
// stamped at once. The operator compares the timestamp against what it last acted on, insists on
// RFC3339 with an offset, and rejects anything it cannot parse, hence the explicit format rather
// than time.Now formatted loosely.
func (c *Client) RestartDorisTier(ctx context.Context, namespace, name, tier string) error {
	restartedAt := time.Now().Format(time.RFC3339)
	data := fmt.Sprintf(`{"metadata": {"annotations": {"apache.doris.%s/restartedAt": %q}}}`, tier, restartedAt)
	_, err := c.Dynamic.Resource(dorisClusterResource).Namespace(namespace).Patch(ctx, name, types.MergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error restarting %q: %v", name, err)
	}
	return nil
}

// DeletePVCs deletes the persistent volume claims matching each label selector in the given
// namespace. Selectors are pre-formatted by the caller (a component's PVCSelectors).
func (c *Client) DeletePVCs(ctx context.Context, namespace string, selectors []string) error {
	pvcs := c.Clientset.CoreV1().PersistentVolumeClaims(namespace)

	for _, selector := range selectors {
		listOptions := metav1.ListOptions{LabelSelector: selector}
		list, err := pvcs.List(ctx, listOptions)
		if err != nil {
			return fmt.Errorf("error finding pvcs using selector %q: %v", selector, err)
		}

		if len(list.Items) > 1 {
			return fmt.Errorf("multiple pvcs found using the selector: %q", selector)
		}

		if len(list.Items) == 1 {
			name := list.Items[0].Name
			err := pvcs.Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed to delete pvc: %v", err)
			}
		}
	}

	return nil
}

func (c *Client) scale(ctx context.Context, instance *model.DeploymentInstance, replicas int32) error {
	labelSelector := fmt.Sprintf("im-id=%d", instance.ID)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}

	deployments := c.Clientset.AppsV1().Deployments(instance.Group.Namespace)
	deploymentList, err := deployments.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("error finding deployments using selector %q: %v", labelSelector, err)
	}

	for _, d := range deploymentList.Items {
		_, err = scale(ctx, deployments, d.Name, replicas)
		if err != nil {
			return err
		}
	}

	sets := c.Clientset.AppsV1().StatefulSets(instance.Group.Namespace)
	setsList, err := sets.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("error finding StatefulSets using selector %q: %v", labelSelector, err)
	}

	for _, s := range setsList.Items {
		_, err = scale(ctx, sets, s.Name, replicas)
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
func scale(ctx context.Context, sc scaler, name string, replicas int32) (int32, error) {
	scale, err := sc.GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get scale of %q: %v", name, err)
	}

	prevReplicas := scale.Spec.Replicas
	scale.Spec.Replicas = replicas

	_, err = sc.UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to update scale of %q to %d: %v", name, replicas, err)
	}

	return prevReplicas, nil
}
