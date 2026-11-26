package instance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
)

func TestMaxDeploymentNameLength(t *testing.T) {
	t.Run("LeavesRoomForTheGroupIdAndTheLongestClusterSuffix", func(t *testing.T) {
		assert.Equal(t, 31, maxDeploymentNameLength(&model.Group{ID: 1}))
		assert.Equal(t, 30, maxDeploymentNameLength(&model.Group{ID: 42}))
		assert.Equal(t, 28, maxDeploymentNameLength(&model.Group{ID: 1000}))
	})

	t.Run("NoStackCreatesALongerCluster", func(t *testing.T) {
		longest := ""
		for _, s := range stack.All {
			for _, component := range s.Components {
				cluster, ok := component.(stack.CNPGPostgresComponent)
				if !ok {
					continue
				}
				suffix := fmt.Sprintf(cluster.ClusterPattern, "")
				if len(suffix) > len(longest) {
					longest = suffix
				}
			}
		}

		assert.Equal(t, longestClusterSuffix, longest)
	})
}

func TestDeploymentNameReason(t *testing.T) {
	group := &model.Group{ID: 1, Name: "group"}

	t.Run("AcceptsANameOfTheLongestLength", func(t *testing.T) {
		assert.Empty(t, deploymentNameReason(strings.Repeat("a", 31), group))
	})

	t.Run("RejectsALongerNameWithTheLimitOfTheGroup", func(t *testing.T) {
		assert.Equal(t, `name must be at most 31 characters in group "group"`, deploymentNameReason(strings.Repeat("a", 32), group))
	})

	t.Run("RejectsANameThatIsNotADNSLabel", func(t *testing.T) {
		assert.Contains(t, deploymentNameReason("Not_Valid", group), "lowercase letters")
		assert.Contains(t, deploymentNameReason("ends-with-", group), "lowercase letters")
	})
}
