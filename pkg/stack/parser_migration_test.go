package stack

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO remove before merging!
// This test is to assert that using the same input (stacks), we do get the same output using the
// old and new way of parsing stacks.

func TestNewOldParser(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dir := "../../stacks"
	stacksNew, err := parseStacks(dir)
	require.NoError(err, "error parsing stacks using NEW parseStacks")

	stacksOld, err := parseStacksOld(dir)
	require.NoError(err, "error parsing stacks using OLD parseStacks")

	// Transform []*model.Stack into maps so I can get a diff that is easier to read using a per
	// stack comparison rather than a slice to slice of stacks comparison
	var newStacks []string
	newS := make(map[string]*model.Stack)
	for _, s := range stacksNew {
		newStacks = append(newStacks, s.Name)
		newS[s.Name] = s
	}

	var oldStacks []string
	oldS := make(map[string]*model.Stack)
	for _, s := range stacksOld {
		oldStacks = append(oldStacks, s.Name)
		oldS[s.Name] = s
	}

	assert.Equal(newStacks, oldStacks)
	for _, o := range oldS {
		n, ok := newS[o.Name]
		require.True(ok, "stack %q does not exist in NEW parseStacks", o.Name)
		assert.Equal(o.HostnamePattern, n.HostnamePattern, "stack %q HostnamePattern was parsed differently in OLD and NEW")
		assert.Equal(o.HostnameVariable, n.HostnameVariable, "stack %q HostnameVariable was parsed differently in OLD and NEW")
		assert.ElementsMatchf(o.RequiredParameters, n.RequiredParameters, "stack %q was parsed differently in OLD (list A) and NEW (list B)", o.Name)
		assert.ElementsMatchf(o.OptionalParameters, n.OptionalParameters, "stack %q was parsed differently in OLD (list A) and NEW (list B)", o.Name)
	}
}
