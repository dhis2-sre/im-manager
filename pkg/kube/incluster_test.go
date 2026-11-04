package kube

import (
	"errors"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func insideCluster() (*rest.Config, error) {
	return &rest.Config{Host: "https://kubernetes.default.svc"}, nil
}

func outsideCluster() (*rest.Config, error) {
	return nil, rest.ErrNotInCluster
}

func TestResolveInCluster(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		inClusterConfig func() (*rest.Config, error)
		want            bool
		wantErr         string
	}{
		{name: "unset inside a cluster", mode: "", inClusterConfig: insideCluster, want: true},
		{name: "unset outside a cluster", mode: "", inClusterConfig: outsideCluster, want: false},
		{name: "auto inside a cluster", mode: "auto", inClusterConfig: insideCluster, want: true},
		{name: "auto outside a cluster", mode: "auto", inClusterConfig: outsideCluster, want: false},
		{name: "true inside a cluster", mode: "true", inClusterConfig: insideCluster, want: true},
		{name: "true outside a cluster", mode: "true", inClusterConfig: outsideCluster, wantErr: "IN_CLUSTER is true, but instance manager is not running inside a cluster"},
		{name: "false inside a cluster", mode: "false", inClusterConfig: insideCluster, want: false},
		{name: "false outside a cluster", mode: "false", inClusterConfig: outsideCluster, want: false},
		{name: "anything else", mode: "yes", inClusterConfig: insideCluster, wantErr: `IN_CLUSTER must be auto, true or false, got "yes"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveInCluster(test.mode, test.inClusterConfig)

			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestReachable(t *testing.T) {
	configured := model.Cluster{ID: 2, Name: "hetzner", Configuration: []byte("encrypted kubeconfig")}
	unconfigured := model.Cluster{ID: 1, Name: "default"}
	unassigned := model.Cluster{}

	t.Run("a configured cluster is reachable from anywhere", func(t *testing.T) {
		assert.NoError(t, reachable(configured, false))
		assert.NoError(t, reachable(configured, true))
	})

	t.Run("in cluster, an unconfigured cluster and an unassigned group mean the cluster instance manager runs in", func(t *testing.T) {
		assert.NoError(t, reachable(unconfigured, true))
		assert.NoError(t, reachable(unassigned, true))
	})

	t.Run("outside a cluster, an unconfigured cluster is a bad request", func(t *testing.T) {
		err := reachable(unconfigured, false)

		require.Error(t, err)
		assert.True(t, errdef.IsBadRequest(err))
		assert.EqualError(t, err, `cluster "default" has no kubernetes configuration, and instance manager does not deploy to the cluster it runs in`)
	})

	t.Run("outside a cluster, an unassigned group is a bad request", func(t *testing.T) {
		err := reachable(unassigned, false)

		require.Error(t, err)
		assert.True(t, errdef.IsBadRequest(err))
		assert.EqualError(t, err, "no cluster is assigned, and instance manager does not deploy to the cluster it runs in")
	})
}

func TestNewRestConfigRefusesAnUnconfiguredClusterOutsideACluster(t *testing.T) {
	t.Setenv("IN_CLUSTER", "false")

	_, err := NewRestConfig(model.Cluster{ID: 1, Name: "default"})

	require.Error(t, err)
	assert.True(t, errdef.IsBadRequest(err))
	assert.False(t, errors.Is(err, rest.ErrNotInCluster))
}
