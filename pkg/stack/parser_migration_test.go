package stack

import (
	"fmt"
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
	// NOTE: the parseStacksOld parses default values as strings while the new parsing parses them
	// as any so int, string, ... Turn them into strings here for comparison.
	// We are converting them to strings LoadStacks when converting them into the
	// model.StackOptionalParameter in the same way
	for _, s := range stacksNew {
		for k, v := range s.envs {
			s.envs[k] = fmt.Sprintf("%v", v)
		}
	}

	stacksOld, err := parseStacksOld(dir)
	require.NoError(err, "error parsing stacks using OLD parseStacks")

	require.Equal(stacksNew, stacksOld)
}
