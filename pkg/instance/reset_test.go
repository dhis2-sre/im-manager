package instance

import (
	"log/slog"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A reset gets the instance from a decrypted deployment, so checking the component must not decrypt
// its sensitive parameters again: the default CUSTOM_DHIS2_CONFIG of " " is no ciphertext at all.
func TestCheckComponentResetLeavesDecryptedParametersAlone(t *testing.T) {
	stackService := stack.NewService(stack.Stacks{stack.DHIS2V2.Name: stack.DHIS2V2})
	service := NewService(nil, &repository{instanceParameterEncryptionKey: testKey}, nil, stackService, nil, nil, "", kube.NewClients(slog.Default()))
	instance := &model.DeploymentInstance{
		StackName: stack.DHIS2V2.Name,
		Parameters: model.DeploymentInstanceParameters{
			"CUSTOM_DHIS2_CONFIG": {ParameterName: "CUSTOM_DHIS2_CONFIG", Value: " "},
			"STORAGE_TYPE":        {ParameterName: "STORAGE_TYPE", Value: "minio"},
		},
	}

	require.NoError(t, service.CheckComponentReset(instance, "db"))

	assert.Equal(t, " ", instance.Parameters["CUSTOM_DHIS2_CONFIG"].Value)
}
