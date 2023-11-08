package instance

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGroupWithInstances_Order(t *testing.T) {
	instancesA := &model.Instance{GroupName: "a"}
	instancesB := &model.Instance{GroupName: "b"}
	instancesMap := map[string][]*model.Instance{
		"b": {instancesB},
		"a": {instancesA},
	}
	groupMap := map[string]model.Group{
		"a": {Name: "a"},
		"b": {Name: "b"},
	}

	actual := groupWithInstances(instancesMap, groupMap)

	expected := []GroupsWithInstances{
		{Name: "a", Instances: []*model.Instance{instancesA}},
		{Name: "b", Instances: []*model.Instance{instancesB}},
	}
	assert.Equal(t, expected, actual)
}
