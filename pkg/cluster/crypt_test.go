package cluster

import (
	"testing"

	"filippo.io/age"
	"github.com/getsops/sops/v3"
	sops_age "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateKeyGroup(t *testing.T) {
	t.Run("age key from env", func(t *testing.T) {
		identity, err := age.GenerateX25519Identity()
		require.NoError(t, err)

		t.Setenv("SOPS_KMS_ARN", "")
		t.Setenv(sops_age.SopsAgeKeyEnv, identity.String())

		keyGroups, err := createKeyGroup()

		require.NoError(t, err)
		assert.Len(t, keyGroups, 1)
		assert.NotEmpty(t, keyGroups[0])
	})

	t.Run("no key provided", func(t *testing.T) {
		t.Setenv("SOPS_KMS_ARN", "")
		t.Setenv(sops_age.SopsAgeKeyEnv, "")

		_, err := createKeyGroup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no encryption key provided")
	})
}

func TestEncryptYaml(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err, "failed to generate age key pair")

	t.Setenv("SOPS_KMS_ARN", "")
	t.Setenv(sops_age.SopsAgeKeyEnv, identity.String())

	ageKeys, err := sops_age.MasterKeysFromRecipients(identity.Recipient().String())
	require.NoError(t, err, "failed to get master keys from age recipient")

	var ageMasterKeys []keys.MasterKey
	for _, k := range ageKeys {
		ageMasterKeys = append(ageMasterKeys, k)
	}
	keyGroups := []sops.KeyGroup{ageMasterKeys}

	t.Run("valid yaml encryption", func(t *testing.T) {
		yamlData := []byte("database:\n  host: localhost\n  port: 5432\n")
		encrypted, err := EncryptYaml(yamlData, keyGroups)

		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Contains(t, string(encrypted), "sops:")
		assert.Contains(t, string(encrypted), "ENC[AES256_GCM")
		assert.NotContains(t, string(encrypted), "localhost")
		assert.NotContains(t, string(encrypted), "5432")
	})

	t.Run("nested yaml structure", func(t *testing.T) {
		yamlData := []byte(`config:
  database:
    host: localhost
    port: 5432
  cache:
    enabled: true
    ttl: 300
`)
		encrypted, err := EncryptYaml(yamlData, keyGroups)

		require.NoError(t, err)
		assert.NotContains(t, string(encrypted), "localhost")
		assert.NotContains(t, string(encrypted), "5432")
		assert.NotContains(t, string(encrypted), "enabled: true")
		assert.NotContains(t, string(encrypted), "ttl: 300")
	})

	t.Run("empty yaml document", func(t *testing.T) {
		yamlData := []byte("---")
		encrypted, err := EncryptYaml(yamlData, keyGroups)

		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Contains(t, string(encrypted), "sops:")
	})

	t.Run("list yaml", func(t *testing.T) {
		yamlData := []byte(`servers:
  - name: server1
    ip: 192.168.1.1
  - name: server2
    ip: 192.168.1.2
`)
		encrypted, err := EncryptYaml(yamlData, keyGroups)

		require.NoError(t, err)
		assert.NotContains(t, string(encrypted), "192.168.1.1")
		assert.NotContains(t, string(encrypted), "192.168.1.2")
		assert.NotContains(t, string(encrypted), "server1")
		assert.NotContains(t, string(encrypted), "server2")
	})

	t.Run("invalid yaml", func(t *testing.T) {
		yamlData := []byte("invalid: yaml: content:")
		_, err := EncryptYaml(yamlData, keyGroups)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load yaml")
	})
}
