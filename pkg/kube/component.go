package kube

import (
	"context"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Component is a single addressable part of a deployed stack (e.g. dhis2 core, its database).
// Components are static stack metadata; pkg/stack owns the registry mapping each stack to its
// components. Each concrete type knows how to restart its own underlying Kubernetes resource.
type Component interface {
	ComponentName() string
	Restart(ctx context.Context, client *Client, instance *model.DeploymentInstance) error
	PVCSelectors(instance *model.DeploymentInstance) []string
}

// BaseComponent supplies the shared name and PVC-selector formatting; concrete component types
// embed it and implement Restart.
type BaseComponent struct {
	Name        string
	PVCPatterns []string
}

func (b BaseComponent) ComponentName() string {
	return b.Name
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

// PodComponent restarts charts that label only pods (no workload controller carrying the im
// labels) by deleting the matching pods; the chart's controller recreates them.
type PodComponent struct {
	BaseComponent
}

func (c PodComponent) Restart(ctx context.Context, client *Client, instance *model.DeploymentInstance) error {
	return client.DeletePods(ctx, instance, c.Name)
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
