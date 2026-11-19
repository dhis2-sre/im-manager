package instance

import (
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDHIS2V2HelmTimeoutEndsBeforeTheDeployIsKilled asserts helm gives up on the seed hook before
// DeployTimeout kills helmfile, so a slow seed is reported by helm rather than cut off mid-wait.
func TestDHIS2V2HelmTimeoutEndsBeforeTheDeployIsKilled(t *testing.T) {
	helmfile, err := os.ReadFile("../../stacks/dhis2-v2/helmfile.yaml.gotmpl")
	require.NoError(t, err)

	match := regexp.MustCompile(`(?m)^helmDefaults:\n(?:  .*\n)*?  timeout: (\d+)$`).FindSubmatch(helmfile)
	require.NotNil(t, match, "the dhis2-v2 helmfile sets no helmDefaults timeout")
	seconds, err := strconv.Atoi(string(match[1]))
	require.NoError(t, err)

	timeout := time.Duration(seconds) * time.Second
	assert.Greater(t, timeout, 5*time.Minute, "no longer than helm's own default")
	assert.Less(t, timeout, DeployTimeout)
}
