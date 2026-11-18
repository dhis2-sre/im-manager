package instance

import (
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkDeploymentNames(t *testing.T) {
	t.Run("NamesPrefixOneToCount", func(t *testing.T) {
		names, err := bulkDeploymentNames("load", 3)

		require.NoError(t, err)
		assert.Equal(t, []string{"load-1", "load-2", "load-3"}, names)
	})

	t.Run("AllowsTheCap", func(t *testing.T) {
		names, err := bulkDeploymentNames("load", MaxBulkDeployments)

		require.NoError(t, err)
		assert.Len(t, names, MaxBulkDeployments)
		assert.Equal(t, "load-50", names[len(names)-1])
	})

	t.Run("RefusesMoreThanTheCap", func(t *testing.T) {
		_, err := bulkDeploymentNames("load", MaxBulkDeployments+1)

		require.Error(t, err)
		assert.True(t, errdef.IsBadRequest(err))
	})
}
