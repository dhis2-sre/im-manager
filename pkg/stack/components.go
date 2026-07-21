package stack

import (
	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/kube"
)

// components maps a stack name to its components. Component names equal the im-type label value
// each stack's helmfile applies. PVCPatterns reproduce the historic hardcoded map exactly; %s is
// filled with the instance's "<name>-<groupID>" unique name.
var components = map[string][]kube.Component{
	"dhis2-db": {
		kube.StatefulSetComponent{BaseComponent: kube.BaseComponent{
			Name:        "db",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s-database"},
		}},
	},
	"minio": {
		kube.DeploymentComponent{BaseComponent: kube.BaseComponent{
			Name:        "minio",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s-minio"},
		}},
	},
	"dhis2-core": {
		kube.DeploymentComponent{BaseComponent: kube.BaseComponent{
			Name: "dhis2",
			PVCPatterns: []string{
				"app.kubernetes.io/instance=%s",
				"app.kubernetes.io/instance=%s-minio",
			},
		}},
	},
	"dhis2": {
		kube.DeploymentComponent{BaseComponent: kube.BaseComponent{Name: "dhis2"}},
		kube.StatefulSetComponent{BaseComponent: kube.BaseComponent{
			Name: "db",
			PVCPatterns: []string{
				"app.kubernetes.io/instance=%s-database",
				// The redis release carries no im labels yet, so it has no component; its PVC still
				// rides here until redis labels land (roadmap step 4).
				"app.kubernetes.io/instance=%s-redis",
			},
		}},
	},
	"pgadmin": {
		kube.StatefulSetComponent{BaseComponent: kube.BaseComponent{Name: "pgadmin"}},
	},
	"whoami-go": {
		kube.DeploymentComponent{BaseComponent: kube.BaseComponent{Name: "whoami"}},
	},
	"im-job-runner": {
		kube.PodComponent{BaseComponent: kube.BaseComponent{Name: "job"}},
	},
	"chap-db": {
		kube.StatefulSetComponent{BaseComponent: kube.BaseComponent{Name: "chap-db"}},
	},
	"chap-valkey": {
		kube.StatefulSetComponent{BaseComponent: kube.BaseComponent{Name: "chap-valkey"}},
	},
	"chap-worker": {
		kube.DeploymentComponent{BaseComponent: kube.BaseComponent{Name: "chap-worker"}},
	},
	"chap-core": {
		kube.DeploymentComponent{BaseComponent: kube.BaseComponent{Name: "chap-core"}},
	},
}

// Components returns the components of the named stack.
func (s Service) Components(stackName string) ([]kube.Component, error) {
	c, ok := components[stackName]
	if !ok {
		return nil, errdef.NewNotFound("stack not found: %s", stackName)
	}
	return c, nil
}
