package stack

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Stack representing ../../stacks/dhis2-db/helmfile.yaml.gotmpl
var DHIS2DB = Stack{
	// TODO: Remove HostnamePattern once stacks 2.0 are the default
	HostnamePattern: "%s-database-postgresql.%s.svc",
	Name:            "dhis2-db",
	Parameters: StackParameters{
		"DATABASE_ID":               {Priority: 1, DisplayName: "Database"},
		"DATABASE_SIZE":             {Priority: 2, DisplayName: "Database Size", DefaultValue: &dhis2DBDefaults.dbSize},
		"DATABASE_NAME":             {Priority: 3, DisplayName: "Database Name", DefaultValue: &dhis2DBDefaults.dbName},
		"DATABASE_PASSWORD":         {Priority: 4, DisplayName: "Database Password", DefaultValue: &dhis2DBDefaults.dbPassword, Sensitive: true},
		"DATABASE_USERNAME":         {Priority: 5, DisplayName: "Database Username", DefaultValue: &dhis2DBDefaults.dbUsername, Sensitive: true},
		"DATABASE_VERSION":          {Priority: 6, DisplayName: "Database Version", DefaultValue: &dhis2DBDefaults.dbVersion},
		"RESOURCES_REQUESTS_CPU":    {Priority: 7, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		"RESOURCES_REQUESTS_MEMORY": {Priority: 8, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
		"CHART_VERSION":             {Priority: 9, DisplayName: "Chart Version", DefaultValue: &dhis2DBDefaults.chartVersion},
	},
	ParameterProviders: ParameterProviders{
		"DATABASE_HOSTNAME": postgresHostnameProvider,
	},
	Components: []kube.Component{
		BitnamiPostgresComponent{BaseComponent: kube.BaseComponent{
			Name:        "db",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s-database"},
		}},
	},
}

// Provides the PostgreSQL hostname of an instance.

// Provides the PostgreSQL hostname of an instance.
var postgresHostnameProvider = ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-%d-database-postgresql.%s.svc", instance.Name, instance.Group.ID, instance.Group.Namespace), nil
})

var dhis2DBDefaults = struct {
	chartVersion            string
	dbID                    string
	dbName                  string
	dbPassword              string
	dbSize                  string
	dbUsername              string
	dbVersion               string
	resourcesRequestsCPU    string
	resourcesRequestsMemory string
}{
	chartVersion:            "16.4.5",
	dbName:                  "dhis2",
	dbPassword:              "dhis",
	dbSize:                  "20Gi",
	dbUsername:              "dhis",
	dbVersion:               "16",
	resourcesRequestsCPU:    "250m",
	resourcesRequestsMemory: "256Mi",
}

// Stack representing ../../stacks/minio/helmfile.yaml.gotmpl
