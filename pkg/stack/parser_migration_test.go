package stack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// This test is to assert that using the same input (stacks), we do get the same output using the
// old and new way of parsing stacks
// Delete once changes are agreed upon and before merging!

func TestNewOldParser(t *testing.T) {
	require := require.New(t)

	dir := "../../stacks/"
	stacksNew, err := parseStacks(dir)
	require.NoError(err, "error parsing stacks using NEW parseStacks")

	stacksOld, err := parseStacksOld(dir)
	require.NoError(err, "error parsing stacks using OLD parseStacks")

	require.Equal(stacksNew, stacksOld)
}
