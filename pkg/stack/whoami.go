package stack

import "github.com/dhis2-sre/im-manager/pkg/kube"

// Stack representing ../../stacks/whoami-go/helmfile.yaml.gotmpl
var WhoamiGo = Stack{
	Name: "whoami-go",
	Parameters: StackParameters{
		"IMAGE_TAG":         {Priority: 1, DisplayName: "Image Tag", DefaultValue: &whoamiGoDefaults.imageTag},
		"IMAGE_REPOSITORY":  {Priority: 2, DisplayName: "Image Repository", DefaultValue: &whoamiGoDefaults.imageRepository},
		"IMAGE_PULL_POLICY": {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &whoamiGoDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"REPLICA_COUNT":     {Priority: 4, DisplayName: "Replica Count", DefaultValue: &whoamiGoDefaults.replicaCount},
		"CHART_VERSION":     {Priority: 5, DisplayName: "Chart Version", DefaultValue: &whoamiGoDefaults.chartVersion},
	},
	Components: []kube.Component{
		WhoamiComponent{BaseComponent: kube.BaseComponent{Name: "whoami"}},
	},
}

var whoamiGoDefaults = struct {
	chartVersion    string
	imagePullPolicy string
	imageRepository string
	imageTag        string
	replicaCount    string
}{
	chartVersion:    "0.9.0",
	imagePullPolicy: ifNotPresent,
	imageRepository: "whoami-go",
	imageTag:        "0.6.0",
	replicaCount:    "1",
}

// Stack representing ../../stacks/im-job-runner/helmfile.yaml.gotmpl
