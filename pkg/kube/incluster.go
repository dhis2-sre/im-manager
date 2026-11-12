package kube

import (
	"fmt"
	"os"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"k8s.io/client-go/rest"
)

// InCluster reports whether instance manager deploys to the cluster it is running in when a cluster
// has no kubernetes configuration, or a group has no cluster. The IN_CLUSTER environment variable
// controls this. With "auto", the default, it does so only when it is running inside a cluster. With
// "true" it always does, and with "false" it never does.
func InCluster() (bool, error) {
	return resolveInCluster(os.Getenv("IN_CLUSTER"), rest.InClusterConfig)
}

func resolveInCluster(mode string, inClusterConfig func() (*rest.Config, error)) (bool, error) {
	switch mode {
	case "", "auto":
		_, err := inClusterConfig()
		return err == nil, nil
	case "true":
		if _, err := inClusterConfig(); err != nil {
			return false, fmt.Errorf("IN_CLUSTER is true, but instance manager is not running inside a cluster: %v", err)
		}
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("IN_CLUSTER must be auto, true or false, got %q", mode)
}

// Reachable returns a bad request error when instance manager cannot reach the cluster. That is the
// case when the cluster has no kubernetes configuration and instance manager is not set up to deploy
// to the cluster it is running in.
func Reachable(cluster model.Cluster) error {
	if len(cluster.Configuration) > 0 {
		return nil
	}

	inCluster, err := InCluster()
	if err != nil {
		return err
	}
	return reachable(cluster, inCluster)
}

func reachable(cluster model.Cluster, inCluster bool) error {
	if len(cluster.Configuration) > 0 || inCluster {
		return nil
	}
	if cluster.ID == 0 {
		return errdef.NewBadRequest("no cluster is assigned, and instance manager is not set up to deploy to the cluster it is running in")
	}
	return errdef.NewBadRequest("cluster %q has no kubernetes configuration, and instance manager is not set up to deploy to the cluster it is running in", cluster.Name)
}
