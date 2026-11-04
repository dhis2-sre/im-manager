package kube

import (
	"fmt"
	"os"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"k8s.io/client-go/rest"
)

// InCluster reports whether a cluster without a kubernetes configuration, and a group without a
// cluster, mean the cluster instance manager runs in. IN_CLUSTER decides: auto, the default, when
// instance manager runs inside a cluster, true always, and false never.
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

// Reachable returns a bad request error when instance manager has no way to reach the cluster:
// it has no kubernetes configuration, and instance manager does not deploy to the cluster it runs
// in.
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
		return errdef.NewBadRequest("no cluster is assigned, and instance manager does not deploy to the cluster it runs in")
	}
	return errdef.NewBadRequest("cluster %q has no kubernetes configuration, and instance manager does not deploy to the cluster it runs in", cluster.Name)
}
