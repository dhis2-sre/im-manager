package group_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/cluster"
	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupsReportWhyTheirClusterIsUnreachable(t *testing.T) {
	t.Setenv("IN_CLUSTER", "false")

	const noClusterReason = "no cluster is assigned, and instance manager is not set up to deploy to the cluster it is running in"

	db := inttest.SetupDB(t)
	userService := user.NewService("", 900, user.NewRepository(db), fakeDialer{})
	clusterService := cluster.NewService(cluster.NewRepository(db), cluster.Encryptor{})
	groupService := group.NewService(group.NewRepository(db), userService, clusterService)

	configured := model.Cluster{Name: "configured", Configuration: []byte("kubernetes configuration")}
	require.NoError(t, db.Create(&configured).Error)

	_, err := groupService.Create(t.Context(), "without-cluster", "without-cluster", "", "without-cluster.example.org", true, nil)
	require.NoError(t, err)
	_, err = groupService.Create(t.Context(), "with-cluster", "with-cluster", "", "with-cluster.example.org", true, &configured.ID)
	require.NoError(t, err)
	_, err = groupService.Create(t.Context(), "not-deployable", "not-deployable", "", "not-deployable.example.org", false, nil)
	require.NoError(t, err)

	member, err := userService.FindOrCreate(t.Context(), "member@dhis2.org", "oneoneoneoneoneoneone111")
	require.NoError(t, err)
	for _, name := range []string{"without-cluster", "with-cluster"} {
		require.NoError(t, groupService.AddUser(t.Context(), name, member.ID))
	}
	member, err = userService.FindById(t.Context(), member.ID)
	require.NoError(t, err)

	t.Run("Find", func(t *testing.T) {
		withoutCluster, err := groupService.Find(t.Context(), "without-cluster")
		require.NoError(t, err)
		assert.Equal(t, noClusterReason, withoutCluster.ClusterUnreachableReason)

		withCluster, err := groupService.Find(t.Context(), "with-cluster")
		require.NoError(t, err)
		assert.Empty(t, withCluster.ClusterUnreachableReason)
	})

	t.Run("FindWithDetails", func(t *testing.T) {
		withoutCluster, err := groupService.FindWithDetails(t.Context(), "without-cluster")
		require.NoError(t, err)
		assert.Equal(t, noClusterReason, withoutCluster.ClusterUnreachableReason)
	})

	t.Run("FindAll as a member", func(t *testing.T) {
		groups, err := groupService.FindAll(t.Context(), member, false)
		require.NoError(t, err)

		reasons := make(map[string]string)
		for _, group := range groups {
			reasons[group.Name] = group.ClusterUnreachableReason
		}
		assert.Equal(t, map[string]string{"without-cluster": noClusterReason, "with-cluster": ""}, reasons)
	})

	t.Run("FindUnreachable lists only deployable groups", func(t *testing.T) {
		groups, err := groupService.FindUnreachable(t.Context())
		require.NoError(t, err)

		require.Len(t, groups, 1)
		assert.Equal(t, "without-cluster", groups[0].Name)
		assert.Equal(t, noClusterReason, groups[0].ClusterUnreachableReason)
	})
}
