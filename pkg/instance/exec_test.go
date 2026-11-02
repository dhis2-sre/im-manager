package instance

import (
	"os/exec"
	"testing"

	"filippo.io/age"
	"github.com/getsops/sops/v3"
	sopsage "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dhis2-sre/im-manager/pkg/cluster"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

func encryptedClusterConfiguration(t *testing.T) []byte {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	t.Setenv("SOPS_AGE_KEY", identity.String())

	ageKeys, err := sopsage.MasterKeysFromRecipients(identity.Recipient().String())
	require.NoError(t, err)
	var masterKeys []keys.MasterKey
	for _, key := range ageKeys {
		masterKeys = append(masterKeys, key)
	}

	configuration, err := cluster.EncryptYaml([]byte("apiVersion: v1\nkind: Config\n"), []sops.KeyGroup{masterKeys})
	require.NoError(t, err)
	return configuration
}

// A failed command used to come back as "error handling kube config", which pointed at the cleanup
// of the kubeconfig rather than at the command that failed.
func TestCommandExecutorReportsTheCommandsOwnFailure(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo boom >&2; exit 3")

	_, stderr, err := commandExecutor(cmd, model.Cluster{Configuration: encryptedClusterConfiguration(t)})

	require.EqualError(t, err, "exit status 3")
	assert.Equal(t, "boom\n", string(stderr))
}
