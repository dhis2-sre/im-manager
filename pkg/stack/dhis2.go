package stack

import "github.com/dhis2-sre/im-manager/pkg/kube"

// Stack representing ../../stacks/dhis2/helmfile.yaml.gotmpl
var DHIS2 = Stack{
	// TODO: Remove HostnamePattern once stacks 2.0 are the default
	HostnamePattern: "%s-database-postgresql.%s.svc",
	Name:            "dhis2",
	Parameters: StackParameters{
		"IMAGE_TAG":                       {Priority: 1, DisplayName: "Image Tag", DefaultValue: &dhis2CoreDefaults.imageTag},
		"IMAGE_REPOSITORY":                {Priority: 2, DisplayName: "Image Repository", DefaultValue: &dhis2CoreDefaults.imageRepository},
		"IMAGE_PULL_POLICY":               {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &dhis2CoreDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"DATABASE_ID":                     {Priority: 4, DisplayName: "Database"},
		"DATABASE_NAME":                   {Priority: 5, DisplayName: "Database Name", DefaultValue: &dhis2DBDefaults.dbName},
		"DATABASE_PASSWORD":               {Priority: 6, DisplayName: "Database Password", DefaultValue: &dhis2DBDefaults.dbPassword, Sensitive: true},
		"DATABASE_SIZE":                   {Priority: 7, DisplayName: "Database Size", DefaultValue: &dhis2DBDefaults.dbSize},
		"DATABASE_USERNAME":               {Priority: 8, DisplayName: "Database Username", DefaultValue: &dhis2DBDefaults.dbUsername, Sensitive: true},
		"DATABASE_VERSION":                {Priority: 9, DisplayName: "Database Version", DefaultValue: &dhis2DBDefaults.dbVersion},
		"INSTALL_REDIS":                   {Priority: 10, DisplayName: "Install Redis", DefaultValue: &dhis2Defaults.installRedis},
		"DHIS2_HOME":                      {Priority: 11, DisplayName: "DHIS2 Home Directory", DefaultValue: &dhis2CoreDefaults.dhis2Home},
		"FLYWAY_MIGRATE_OUT_OF_ORDER":     {Priority: 12, DisplayName: "Flyway Migrate Out Of Order", DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		"FLYWAY_REPAIR_BEFORE_MIGRATION":  {Priority: 13, DisplayName: "Flyway Repair Before Migration", DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		"CORE_RESOURCES_REQUESTS_CPU":     {Priority: 14, DisplayName: "Core Resources Requests CPU", DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		"CORE_RESOURCES_REQUESTS_MEMORY":  {Priority: 15, DisplayName: "Core Resources Requests Memory", DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		"DB_RESOURCES_REQUESTS_CPU":       {Priority: 16, DisplayName: "DB Resources Requests CPU", DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		"DB_RESOURCES_REQUESTS_MEMORY":    {Priority: 17, DisplayName: "DB Resources Requests Memory", DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
		"MIN_READY_SECONDS":               {Priority: 18, DisplayName: "Minimum Ready Seconds", DefaultValue: &dhis2CoreDefaults.minReadySeconds},
		"LIVENESS_PROBE_TIMEOUT_SECONDS":  {Priority: 19, DisplayName: "Liveness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		"READINESS_PROBE_TIMEOUT_SECONDS": {Priority: 20, DisplayName: "Readiness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		"STARTUP_PROBE_FAILURE_THRESHOLD": {Priority: 21, DisplayName: "Startup Probe Failure Threshold", DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		"STARTUP_PROBE_PERIOD_SECONDS":    {Priority: 22, DisplayName: "Startup Probe Period Seconds", DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
		"CHART_VERSION":                   {Priority: 23, DisplayName: "Chart Version", DefaultValue: &dhis2CoreDefaults.chartVersion},
		"JAVA_OPTS":                       {Priority: 24, DisplayName: "JAVA Options", DefaultValue: &dhis2CoreDefaults.javaOpts},
		"ENABLE_QUERY_LOGGING":            {Priority: 25, DisplayName: "Enable Query Logging", DefaultValue: &dhis2CoreDefaults.enableQueryLogging},
		"GOOGLE_AUTH_PROJECT_ID":          {Priority: 0, DisplayName: "Google auth project id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId, Sensitive: true},
		"GOOGLE_AUTH_PRIVATE_KEY":         {Priority: 0, DisplayName: "Google auth private key", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKey, Sensitive: true},
		"GOOGLE_AUTH_PRIVATE_KEY_ID":      {Priority: 0, DisplayName: "Google auth private key id", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKeyId, Sensitive: true},
		"GOOGLE_AUTH_CLIENT_EMAIL":        {Priority: 0, DisplayName: "Google auth client email", DefaultValue: &dhis2CoreDefaults.googleAuthClientEmail, Sensitive: true},
		"GOOGLE_AUTH_CLIENT_ID":           {Priority: 0, DisplayName: "Google auth client id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId, Sensitive: true},
	},
	ParameterProviders: ParameterProviders{
		"DATABASE_HOSTNAME": postgresHostnameProvider,
	},
	Components: []kube.Component{
		DHIS2CoreComponent{BaseComponent: kube.BaseComponent{Name: "dhis2"}},
		BitnamiPostgresComponent{BaseComponent: kube.BaseComponent{
			Name: "db",
			PVCPatterns: []string{
				"app.kubernetes.io/instance=%s-database",
				// The redis release carries no im labels yet, so it has no component; its PVC
				// still rides here until redis labels land (roadmap step 4).
				"app.kubernetes.io/instance=%s-redis",
			},
		}},
	},
}

// DHIS2V2 deploys the dhis2 umbrella chart: one release bundling DHIS 2 core, PostgreSQL and an
// optional MinIO file store, replacing the deployment-level composition of dhis2-db, dhis2-core
// and the minio companion.

var dhis2Defaults = struct {
	installRedis string
}{
	installRedis: "false",
}

// Stack representing ../../stacks/pgadmin/helmfile.yaml.gotmpl
