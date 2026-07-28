package stack

import (
	"context"

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

// CNPGPostgresComponent operates on a CloudNativePG-managed PostgreSQL cluster. Restart currently
// applies the generic StatefulSet patch, which finds nothing against CNPG's bare pods (unchanged
// from before components existed); roadmap step 5 reimplements it via the Cluster custom resource's
// restart annotation.
type CNPGPostgresComponent struct {
	kube.BaseComponent
}

func (c CNPGPostgresComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartStatefulSet(ctx, instance, c.Name)
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
