package stack

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Stack representing ../../stacks/minio/helmfile.yaml.gotmpl
var MINIO = Stack{
	Name: "minio",
	Parameters: StackParameters{
		"MINIO_STORAGE_SIZE":  {Priority: 1, DisplayName: "Storage Size", DefaultValue: &minIODefaults.storageSize},
		"MINIO_CHART_VERSION": {Priority: 2, DisplayName: "Chart Version", DefaultValue: &minIODefaults.chartVersion},
		"IMAGE_PULL_POLICY":   {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &minIODefaults.imagePullPolicy, Validator: imagePullPolicy},
		"DATABASE_ID":         {Priority: 0, DisplayName: "Database", Consumed: true},
	},
	ParameterProviders: ParameterProviders{
		"MINIO_HOSTNAME": minioHostnameProvider,
	},
	Requires: []Stack{DHIS2DB},
	Components: []kube.Component{
		MinioComponent{BaseComponent: kube.BaseComponent{
			Name:        "minio",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s-minio"},
		}},
	},
}

// Provides the Minio hostname of an instance.

// Provides the Minio hostname of an instance.
var minioHostnameProvider = ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-minio.%s.svc", instance.Name, instance.Group.Namespace), nil
})

var storageCompanionProvider = RequireCompanionFunc(func(parameter model.DeploymentInstanceParameter) (*Stack, error) {
	if parameter.Value == minIOStorage {
		return &MINIO, nil
	}
	return nil, nil
})

var minIODefaults = struct {
	chartVersion    string
	storageSize     string
	imagePullPolicy string
}{
	chartVersion:    "14.7.5",
	storageSize:     "8Gi",
	imagePullPolicy: ifNotPresent,
}

// Stack representing ../../stacks/dhis2-core/helmfile.yaml.gotmpl
