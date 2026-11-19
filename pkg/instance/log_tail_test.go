package instance

import (
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogTailLines(t *testing.T) {
	lines, err := logTailLines("")
	require.NoError(t, err)
	require.NotNil(t, lines)
	assert.Equal(t, DefaultLogTailLines, *lines)

	lines, err = logTailLines("50")
	require.NoError(t, err)
	require.NotNil(t, lines)
	assert.EqualValues(t, 50, *lines)

	lines, err = logTailLines("0")
	require.NoError(t, err)
	assert.Nil(t, lines, "0 streams the whole log")

	for _, tail := range []string{"-1", "all", "1.5", " "} {
		_, err = logTailLines(tail)
		require.Error(t, err, "tail %q", tail)
		assert.True(t, errdef.IsBadRequest(err), "tail %q should be a bad request, got %v", tail, err)
	}
}
