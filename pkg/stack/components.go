package stack

import (
	"context"
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Components returns the components of the named stack.
func (s Service) Components(stackName string) ([]kube.Component, error) {
	stack, err := s.Find(stackName)
	if err != nil {
		return nil, err
	}
	return stack.Components, nil
}

// Concrete component types are named for the chart/technology they operate on; the Kubernetes
// workload kind behind each is an implementation detail of its Restart. This is what lets a future
// dhis2 stack swap BitnamiPostgresComponent for CNPGPostgresComponent in its definition alone,
// bringing operations specific to that technology (e.g. CNPG's native S3 backup) without any
// dispatch changes.

// DHIS2CoreComponent operates on the dhis2-core chart's Deployment.
type DHIS2CoreComponent struct {
	kube.BaseComponent
}

func (c DHIS2CoreComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}

// BitnamiPostgresComponent operates on the Bitnami PostgreSQL chart's StatefulSet.
type BitnamiPostgresComponent struct {
	kube.BaseComponent
}

func (c BitnamiPostgresComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartStatefulSet(ctx, instance, c.Name)
}

// CNPGPostgresComponent operates on a CloudNativePG-managed PostgreSQL cluster. Restart goes
// through the Cluster custom resource so the operator performs the rollout; patching the bare pods
// it manages would fight it.
type CNPGPostgresComponent struct {
	kube.BaseComponent
	// ClusterPattern names the Cluster resource, formatted with the instance's "<name>-<groupID>"
	// unique name, e.g. "%s-dhis2-postgresql".
	ClusterPattern string
}

func (c CNPGPostgresComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	clusterName := fmt.Sprintf(c.ClusterPattern, fmt.Sprintf("%s-%d", instance.Name, instance.Group.ID))
	return client.RestartCNPGCluster(ctx, instance.Group.Namespace, clusterName)
}

// MinioComponent operates on the Bitnami MinIO chart's Deployment.
type MinioComponent struct {
	kube.BaseComponent
}

func (c MinioComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}

// PgAdminComponent operates on the runix pgadmin4 chart's StatefulSet.
type PgAdminComponent struct {
	kube.BaseComponent
}

func (c PgAdminComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartStatefulSet(ctx, instance, c.Name)
}

// WhoamiComponent operates on the whoami-go chart's Deployment.
type WhoamiComponent struct {
	kube.BaseComponent
}

func (c WhoamiComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}

// ValkeyComponent operates on the Bitnami Valkey chart's StatefulSet.
type ValkeyComponent struct {
	kube.BaseComponent
}

func (c ValkeyComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartStatefulSet(ctx, instance, c.Name)
}

// ChapWorkerComponent operates on the chap-worker chart's Deployment.
type ChapWorkerComponent struct {
	kube.BaseComponent
}

func (c ChapWorkerComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}

// ChapCoreComponent operates on the chap-core chart's Deployment.
type ChapCoreComponent struct {
	kube.BaseComponent
}

func (c ChapCoreComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}
