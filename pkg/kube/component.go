package kube

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Operation is the name of an action a component supports, as exposed to API clients.
type Operation string

const (
	OperationRestart         Operation = "restart"
	OperationRestartReplica  Operation = "restartReplica"
	OperationFilestoreBackup Operation = "filestoreBackup"
)

// CapabilityPredicate decides whether a capability applies given the instance's decrypted parameters.
type CapabilityPredicate func(params model.DeploymentInstanceParameters) bool

// Capability declares an operation a component supports beyond the base set. A nil When means the
// operation is always supported.
type Capability struct {
	Operation Operation
	When      CapabilityPredicate
}

// Replica is a live pod backing a component. Replicas are discovered from the cluster on demand
// and never persisted.
type Replica struct {
	Name      string    `json:"name"`
	Phase     string    `json:"phase"`
	Ready     bool      `json:"ready"`
	Restarts  int32     `json:"restarts"`
	CreatedAt time.Time `json:"createdAt"`
}

// Component is a single addressable part of a deployed stack (e.g. dhis2 core, its database).
// Components are static stack metadata; pkg/stack owns the registry mapping each stack to its
// components. Each concrete type knows how to restart its own underlying Kubernetes resource.
type Component interface {
	ComponentName() string
	Restart(ctx context.Context, client *Client, instance *model.DeploymentInstance) error
	RestartReplica(ctx context.Context, client *Client, instance *model.DeploymentInstance, podName string) error
	Replicas(ctx context.Context, client *Client, instance *model.DeploymentInstance) ([]Replica, error)
	PVCSelectors(instance *model.DeploymentInstance) []string
	SupportedOperations(params model.DeploymentInstanceParameters) []Operation
}

// BaseComponent supplies the shared name, capability evaluation, replica discovery and PVC-selector
// formatting; concrete component types embed it and implement Restart.
type BaseComponent struct {
	Name         string
	PVCPatterns  []string
	Capabilities []Capability
}

func (b BaseComponent) ComponentName() string {
	return b.Name
}

// SupportedOperations returns the base operations every component supports plus the capabilities
// whose predicate passes on the instance's decrypted parameters.
func (b BaseComponent) SupportedOperations(params model.DeploymentInstanceParameters) []Operation {
	operations := []Operation{OperationRestart, OperationRestartReplica}
	for _, capability := range b.Capabilities {
		if capability.When == nil || capability.When(params) {
			operations = append(operations, capability.Operation)
		}
	}
	return operations
}

// Replicas lists the pods currently backing this component in the instance's namespace.
func (b BaseComponent) Replicas(ctx context.Context, client *Client, instance *model.DeploymentInstance) ([]Replica, error) {
	pods, err := b.pods(ctx, client, instance)
	if err != nil {
		return nil, err
	}

	replicas := make([]Replica, len(pods))
	for i, pod := range pods {
		replicas[i] = newReplica(pod)
	}
	return replicas, nil
}

// RestartReplica deletes the named pod after validating it belongs to this component, letting the
// owning Deployment/StatefulSet controller recreate it.
func (b BaseComponent) RestartReplica(ctx context.Context, client *Client, instance *model.DeploymentInstance, podName string) error {
	pods, err := b.pods(ctx, client, instance)
	if err != nil {
		return err
	}

	if !slices.ContainsFunc(pods, func(pod v1.Pod) bool { return pod.Name == podName }) {
		return errdef.NewNotFound("pod %q not found for component %q", podName, b.Name)
	}

	err = client.Clientset.CoreV1().Pods(instance.Group.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting pod %q: %v", podName, err)
	}
	return nil
}

func (b BaseComponent) pods(ctx context.Context, client *Client, instance *model.DeploymentInstance) ([]v1.Pod, error) {
	selector, err := labelSelector(instance.ID, b.Name)
	if err != nil {
		return nil, err
	}
	return client.ListPods(ctx, instance.Group.Namespace, selector)
}

func newReplica(pod v1.Pod) Replica {
	var restarts int32
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
	}

	ready := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			ready = condition.Status == v1.ConditionTrue
			break
		}
	}

	return Replica{
		Name:      pod.Name,
		Phase:     string(pod.Status.Phase),
		Ready:     ready,
		Restarts:  restarts,
		CreatedAt: pod.CreationTimestamp.Time,
	}
}

// PVCSelectors formats each PVC pattern with the instance's "<name>-<groupID>" unique name.
func (b BaseComponent) PVCSelectors(instance *model.DeploymentInstance) []string {
	uniqueName := fmt.Sprintf("%s-%d", instance.Name, instance.Group.ID)
	selectors := make([]string, len(b.PVCPatterns))
	for i, pattern := range b.PVCPatterns {
		selectors[i] = fmt.Sprintf(pattern, uniqueName)
	}
	return selectors
}

// DeploymentComponent restarts a workload backed by a Deployment.
type DeploymentComponent struct {
	BaseComponent
}

func (c DeploymentComponent) Restart(_ context.Context, client *Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(instance, c.Name)
}

// StatefulSetComponent restarts a workload backed by a StatefulSet.
type StatefulSetComponent struct {
	BaseComponent
}

func (c StatefulSetComponent) Restart(_ context.Context, client *Client, instance *model.DeploymentInstance) error {
	return client.RestartStatefulSet(instance, c.Name)
}

// FindComponent returns the component with the given name, or a not-found error.
func FindComponent(components []Component, name string) (Component, error) {
	for _, component := range components {
		if component.ComponentName() == name {
			return component, nil
		}
	}
	return nil, errdef.NewNotFound("component not found: %s", name)
}
