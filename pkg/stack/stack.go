package stack

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

const ifNotPresent = "IfNotPresent"

// Stack representing ../../stacks/dhis2-db/helmfile.yaml
var DHIS2DB = model.Stack{
	Name: "dhis2-db",
	Parameters: []model.StackParameter{
		{Name: "CHART_VERSION", DefaultValue: &dhis2DBDefaults.chartVersion},
		{Name: "DATABASE_ID"},
		{Name: "DATABASE_NAME", DefaultValue: &dhis2DBDefaults.dbName},
		{Name: "DATABASE_PASSWORD", DefaultValue: &dhis2DBDefaults.dbPassword},
		{Name: "DATABASE_SIZE", DefaultValue: &dhis2DBDefaults.dbSize},
		{Name: "DATABASE_USERNAME", DefaultValue: &dhis2DBDefaults.dbUsername},
		{Name: "DATABASE_VERSION", DefaultValue: &dhis2DBDefaults.dbVersion},
		{Name: "RESOURCES_REQUESTS_CPU", DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		{Name: "RESOURCES_REQUESTS_MEMORY", DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
	},
	Providers: map[string]model.Provider{
		"DATABASE_HOSTNAME": postgresHostnameProvider,
	},
}

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
	chartVersion:            "12.6.2",
	dbName:                  "dhis2",
	dbPassword:              "dhis",
	dbSize:                  "5Gi",
	dbUsername:              "dhis",
	dbVersion:               "13",
	resourcesRequestsCPU:    "250m",
	resourcesRequestsMemory: "256Mi",
}

// Stack representing ../../stacks/dhis2-core/helmfile.yaml
var DHIS2Core = model.Stack{
	Name: "dhis2-core",
	Parameters: []model.StackParameter{
		{Name: "CHART_VERSION", DefaultValue: &dhis2CoreDefaults.chartVersion},
		{Name: "DATABASE_HOSTNAME", Consumed: true},
		{Name: "DATABASE_NAME", Consumed: true},
		{Name: "DATABASE_PASSWORD", Consumed: true},
		{Name: "DATABASE_USERNAME", Consumed: true},
		{Name: "DHIS2_HOME", DefaultValue: &dhis2CoreDefaults.dhis2Home},
		{Name: "FLYWAY_MIGRATE_OUT_OF_ORDER", DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		{Name: "FLYWAY_REPAIR_BEFORE_MIGRATION", DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		{Name: "IMAGE_PULL_POLICY", DefaultValue: &dhis2CoreDefaults.imagePullPolicy},
		{Name: "IMAGE_REPOSITORY", DefaultValue: &dhis2CoreDefaults.imageRepository},
		{Name: "IMAGE_TAG", DefaultValue: &dhis2CoreDefaults.imageTag},
		{Name: "JAVA_OPTS", DefaultValue: &dhis2CoreDefaults.javaOpts},
		{Name: "LIVENESS_PROBE_TIMEOUT_SECONDS", DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		{Name: "READINESS_PROBE_TIMEOUT_SECONDS", DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		{Name: "RESOURCES_REQUESTS_CPU", DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		{Name: "RESOURCES_REQUESTS_MEMORY", DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		{Name: "STARTUP_PROBE_FAILURE_THRESHOLD", DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		{Name: "STARTUP_PROBE_PERIOD_SECONDS", DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
	},
	Requires: []model.Stack{
		DHIS2DB,
	},
}

var dhis2CoreDefaults = struct {
	chartVersion                 string
	dhis2Home                    string
	flywayMigrateOutOfOrder      string
	flywayRepairBeforeMigration  string
	imagePullPolicy              string
	imageRepository              string
	imageTag                     string
	javaOpts                     string
	livenessProbeTimeoutSeconds  string
	readinessProbeTimeoutSeconds string
	resourcesRequestsCPU         string
	resourcesRequestsMemory      string
	startupProbeFailureThreshold string
	startupProbePeriodSeconds    string
}{
	chartVersion:                 "0.14.0",
	dhis2Home:                    "/opt/dhis2",
	flywayMigrateOutOfOrder:      "false",
	flywayRepairBeforeMigration:  "false",
	imagePullPolicy:              ifNotPresent,
	imageRepository:              "core",
	imageTag:                     "2.40.0",
	javaOpts:                     "",
	livenessProbeTimeoutSeconds:  "1",
	readinessProbeTimeoutSeconds: "1",
	resourcesRequestsCPU:         "250m",
	resourcesRequestsMemory:      "256Mi",
	startupProbeFailureThreshold: "26",
	startupProbePeriodSeconds:    "5",
}

// Stack representing ../../stacks/dhis2/helmfile.yaml
var DHIS2 = model.Stack{
	Name: "dhis2",
	Parameters: []model.StackParameter{
		{Name: "CHART_VERSION", DefaultValue: &dhis2CoreDefaults.chartVersion},
		{Name: "CORE_RESOURCES_REQUESTS_CPU", DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		{Name: "CORE_RESOURCES_REQUESTS_MEMORY", DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		{Name: "DATABASE_ID"},
		{Name: "DATABASE_NAME", DefaultValue: &dhis2DBDefaults.dbName},
		{Name: "DATABASE_PASSWORD", DefaultValue: &dhis2DBDefaults.dbPassword},
		{Name: "DATABASE_SIZE", DefaultValue: &dhis2DBDefaults.dbSize},
		{Name: "DATABASE_USERNAME", DefaultValue: &dhis2DBDefaults.dbUsername},
		{Name: "DATABASE_VERSION", DefaultValue: &dhis2DBDefaults.dbVersion},
		{Name: "DB_RESOURCES_REQUESTS_CPU", DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		{Name: "DB_RESOURCES_REQUESTS_MEMORY", DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
		{Name: "DHIS2_HOME", DefaultValue: &dhis2CoreDefaults.dhis2Home},
		{Name: "FLYWAY_MIGRATE_OUT_OF_ORDER", DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		{Name: "FLYWAY_REPAIR_BEFORE_MIGRATION", DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		{Name: "IMAGE_PULL_POLICY", DefaultValue: &dhis2CoreDefaults.imagePullPolicy},
		{Name: "IMAGE_REPOSITORY", DefaultValue: &dhis2CoreDefaults.imageRepository},
		{Name: "IMAGE_TAG", DefaultValue: &dhis2CoreDefaults.imageTag},
		{Name: "INSTALL_REDIS", DefaultValue: &dhis2Defaults.installRedis},
		{Name: "JAVA_OPTS", DefaultValue: &dhis2CoreDefaults.javaOpts},
		{Name: "LIVENESS_PROBE_TIMEOUT_SECONDS", DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		{Name: "READINESS_PROBE_TIMEOUT_SECONDS", DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		{Name: "STARTUP_PROBE_FAILURE_THRESHOLD", DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		{Name: "STARTUP_PROBE_PERIOD_SECONDS", DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
	},
	Providers: map[string]model.Provider{
		"DATABASE_HOSTNAME": postgresHostnameProvider,
	},
}

var dhis2Defaults = struct {
	installRedis string
}{
	installRedis: "false",
}

// Stack representing ../../stacks/pgadmin/helmfile.yaml
var PgAdmin = model.Stack{
	Name: "pgadmin",
	Parameters: []model.StackParameter{
		{Name: "CHART_VERSION", DefaultValue: &pgAdminDefaults.chartVersion},
		{Name: "DATABASE_HOSTNAME", Consumed: true},
		{Name: "DATABASE_NAME", Consumed: true},
		{Name: "DATABASE_USERNAME", Consumed: true},
		{Name: "PGADMIN_PASSWORD"},
		{Name: "PGADMIN_USERNAME"},
	},
	Requires: []model.Stack{
		DHIS2DB,
	},
}

var pgAdminDefaults = struct {
	chartVersion string
}{
	chartVersion: "1.11.0",
}

// Stack representing ../../stacks/whoami-go/helmfile.yaml
var WhoamiGo = model.Stack{
	Name: "whoami-go",
	Parameters: []model.StackParameter{
		{Name: "CHART_VERSION", DefaultValue: &whoamiGoDefaults.chartVersion},
		{Name: "IMAGE_PULL_POLICY", DefaultValue: &whoamiGoDefaults.imagePullPolicy},
		{Name: "IMAGE_REPOSITORY", DefaultValue: &whoamiGoDefaults.imageRepository},
		{Name: "IMAGE_TAG", DefaultValue: &whoamiGoDefaults.imageTag},
		{Name: "REPLICA_COUNT", DefaultValue: &whoamiGoDefaults.replicaCount},
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
	imageRepository: "core",
	imageTag:        "0.6.0",
	replicaCount:    "1",
}

// Stack representing ../../stacks/im-job-runner/helmfile.yaml
var IMJobRunner = model.Stack{
	Name: "im-job-runner",
	Parameters: []model.StackParameter{
		{Name: "CHART_VERSION", DefaultValue: &imJobRunnerDefaults.chartVersion},
		{Name: "COMMAND"},
		{Name: "DHIS2_DATABASE_DATABASE", DefaultValue: &dhis2DBDefaults.dbName},
		{Name: "DHIS2_DATABASE_HOSTNAME", DefaultValue: &imJobRunnerDefaults.dbHostname},
		{Name: "DHIS2_DATABASE_PASSWORD", DefaultValue: &dhis2DBDefaults.dbPassword},
		{Name: "DHIS2_DATABASE_PORT", DefaultValue: &imJobRunnerDefaults.dbPort},
		{Name: "DHIS2_DATABASE_USERNAME", DefaultValue: &dhis2DBDefaults.dbUsername},
		{Name: "DHIS2_HOSTNAME", DefaultValue: &imJobRunnerDefaults.dhis2Hostname},
		{Name: "PAYLOAD", DefaultValue: &imJobRunnerDefaults.payload},
	},
}

var imJobRunnerDefaults = struct {
	chartVersion  string
	dbHostname    string
	dbPort        string
	dhis2Hostname string
	payload       string
}{
	chartVersion:  "0.1.0",
	dbHostname:    "-",
	dbPort:        "5432",
	dhis2Hostname: "-",
	payload:       "-",
}

// Provides the PostgreSQL hostname of an instance.
var postgresHostnameProvider = model.ProviderFunc(func(instance model.Instance) (string, error) {
	return fmt.Sprintf("%s-database-postgresql.%s.svc", instance.Name, instance.GroupName), nil
})
