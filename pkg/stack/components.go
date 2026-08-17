package stack

import (
	"context"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
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

const bitnamiPostgresContainer = "postgresql"

// PostgresPod returns the component's single pod, located by the im labels its chart applies.
func (c BitnamiPostgresComponent) PostgresPod(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) (string, string, error) {
	replicas, err := c.Replicas(ctx, client, instance)
	if err != nil {
		return "", "", err
	}
	if len(replicas) == 0 {
		return "", "", errdef.NewNotFound("no postgres pod found for component %q", c.Name)
	}
	return replicas[0].Name, bitnamiPostgresContainer, nil
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

const cnpgPostgresContainer = "postgres"

// PostgresPod returns the CNPG cluster's current primary, located by the operator's role label.
func (c CNPGPostgresComponent) PostgresPod(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) (string, string, error) {
	clusterName := fmt.Sprintf(c.ClusterPattern, fmt.Sprintf("%s-%d", instance.Name, instance.Group.ID))
	selector := fmt.Sprintf("cnpg.io/cluster=%s,cnpg.io/instanceRole=primary", clusterName)
	pods, err := client.ListPods(ctx, instance.Group.Namespace, selector)
	if err != nil {
		return "", "", err
	}
	if len(pods) != 1 {
		return "", "", errdef.NewNotFound("expected one primary pod for cluster %q, found %d", clusterName, len(pods))
	}
	return pods[0].Name, cnpgPostgresContainer, nil
}

// DorisComponent operates on one tier of a Doris cluster managed by the doris-operator. Frontends
// and backends are separate components although one custom resource holds both: the operator rolls
// and scales them independently, they run different images sized differently, and a replica count
// that is really two numbers is not something a single component can express. Restart goes through
// that custom resource so the operator performs the rollout rather than us patching the statefulsets
// it owns.
type DorisComponent struct {
	kube.BaseComponent
	// ClusterPattern names the DorisCluster resource, formatted with the instance's "<name>-<groupID>"
	// unique name, e.g. "%s-doris".
	ClusterPattern string
	// Tier is the operator's name for the role, "fe" or "be".
	Tier string
}

func (c DorisComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	clusterName := fmt.Sprintf(c.ClusterPattern, fmt.Sprintf("%s-%d", instance.Name, instance.Group.ID))
	return client.RestartDorisTier(ctx, instance.Group.Namespace, clusterName, c.Tier)
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

// ValkeyComponent operates on the valkey chart's Deployment. The chart only deploys a StatefulSet
// when replicas are enabled, which neither of our stacks does.
type ValkeyComponent struct {
	kube.BaseComponent
}

func (c ValkeyComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}

// ChapAPIComponent operates on the chap chart's api Deployment.
type ChapAPIComponent struct {
	kube.BaseComponent
}

func (c ChapAPIComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}

// ChapRegisterComponent tracks the chap chart's DHIS 2 registration Job. The job is the only
// evidence that registration ran, and its logs say why it failed, so it is a component even though a
// Job is immutable: helm creates one per revision, and it is redeployment, not a restart, that runs
// registration again. It therefore supports no operations, and its pods are the job runs kept
// around by the chart's ttlSecondsAfterFinished.
type ChapRegisterComponent struct {
	kube.BaseComponent
}

func (c ChapRegisterComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return errdef.NewBadRequest("component %q is a job and cannot be restarted, redeploy the instance to register again", c.Name)
}

func (c ChapRegisterComponent) RestartReplica(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance, podName string) error {
	return errdef.NewBadRequest("component %q is a job and its pods cannot be restarted, redeploy the instance to register again", c.Name)
}

// SupportedOperations returns nothing rather than the base restart pair, so clients do not offer
// what a job cannot do.
func (c ChapRegisterComponent) SupportedOperations(params model.DeploymentInstanceParameters) []kube.Operation {
	return nil
}

// ChapWorkerComponent operates on the chap chart's worker Deployment.
type ChapWorkerComponent struct {
	kube.BaseComponent
}

func (c ChapWorkerComponent) Restart(ctx context.Context, client *kube.Client, instance *model.DeploymentInstance) error {
	return client.RestartDeployment(ctx, instance, c.Name)
}
