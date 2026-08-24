package stack

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// DHIS2V2 deploys the dhis2 umbrella chart: one release bundling DHIS 2 core, PostgreSQL and an
// optional MinIO file store, replacing the deployment-level composition of dhis2-db, dhis2-core
// and the minio companion.
var DHIS2V2 = withGroupedParameters(Stack{
	Name: "dhis2-v2",
	ParameterGroups: []ParameterGroup{
		{Name: "dhis2", Title: "DHIS 2 Core", Parameters: StackParameters{
			"IMAGE_TAG":                       {Priority: 1, DisplayName: "Image Tag", DefaultValue: &dhis2CoreDefaults.imageTag},
			"IMAGE_REPOSITORY":                {Priority: 2, DisplayName: "Image Repository", DefaultValue: &dhis2CoreDefaults.imageRepository},
			"IMAGE_PULL_POLICY":               {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &dhis2CoreDefaults.imagePullPolicy, Validator: imagePullPolicy},
			"STORAGE_TYPE":                    {Priority: 10, DisplayName: "Storage type", DefaultValue: &dhis2CoreDefaults.storageType, Validator: storage},
			"DHIS2_HOME":                      {Priority: 17, DisplayName: "DHIS2 Home Directory", DefaultValue: &dhis2CoreDefaults.dhis2Home},
			"FLYWAY_MIGRATE_OUT_OF_ORDER":     {Priority: 18, DisplayName: "Flyway Migrate Out Of Order", DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
			"FLYWAY_REPAIR_BEFORE_MIGRATION":  {Priority: 19, DisplayName: "Flyway Repair Before Migration", DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
			"CORE_RESOURCES_REQUESTS_CPU":     {Priority: 20, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
			"CORE_RESOURCES_REQUESTS_MEMORY":  {Priority: 21, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
			"MIN_READY_SECONDS":               {Priority: 24, DisplayName: "Minimum Ready Seconds", DefaultValue: &dhis2CoreDefaults.minReadySeconds},
			"LIVENESS_PROBE_TIMEOUT_SECONDS":  {Priority: 25, DisplayName: "Liveness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
			"READINESS_PROBE_TIMEOUT_SECONDS": {Priority: 26, DisplayName: "Readiness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
			"STARTUP_PROBE_FAILURE_THRESHOLD": {Priority: 27, DisplayName: "Startup Probe Failure Threshold", DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
			"STARTUP_PROBE_PERIOD_SECONDS":    {Priority: 28, DisplayName: "Startup Probe Period Seconds", DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
			"CHART_VERSION":                   {Priority: 29, DisplayName: "Chart Version", DefaultValue: &dhis2V2Defaults.chartVersion},
			"JAVA_OPTS":                       {Priority: 30, DisplayName: "JAVA Options", DefaultValue: &dhis2CoreDefaults.javaOpts},
			"ENABLE_QUERY_LOGGING":            {Priority: 31, DisplayName: "Enable Query Logging", DefaultValue: &dhis2CoreDefaults.enableQueryLogging},
			"SAME_SITE_COOKIES":               {Priority: 32, DisplayName: "Same site cookies", DefaultValue: &dhis2CoreDefaults.sameSiteCookies, Validator: sameSiteCookies},
			"CUSTOM_DHIS2_CONFIG":             {Priority: 33, DisplayName: "Custom DHIS2 config (applied to top of dhis.conf)", DefaultValue: &dhis2CoreDefaults.customDhis2Config, Sensitive: true},
			"GOOGLE_AUTH_PROJECT_ID":          {Priority: 34, DisplayName: "Google auth project id", DefaultValue: &dhis2CoreDefaults.googleAuthProjectId, Sensitive: true},
			"GOOGLE_AUTH_PRIVATE_KEY":         {Priority: 35, DisplayName: "Google auth private key", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKey, Sensitive: true},
			"GOOGLE_AUTH_PRIVATE_KEY_ID":      {Priority: 36, DisplayName: "Google auth private key id", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKeyId, Sensitive: true},
			"GOOGLE_AUTH_CLIENT_EMAIL":        {Priority: 37, DisplayName: "Google auth client email", DefaultValue: &dhis2CoreDefaults.googleAuthClientEmail, Sensitive: true},
			"GOOGLE_AUTH_CLIENT_ID":           {Priority: 38, DisplayName: "Google auth client id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId, Sensitive: true},
			"DEPLOY_CHAP":                     {Priority: 39, DisplayName: "Deploy CHAP", DefaultValue: &dhis2CoreDefaults.deployChap},
			"ENABLE_DORIS":                    {Priority: 40, DisplayName: "Use Doris as the analytics database", DefaultValue: &dhis2V2Defaults.enableDoris},
		}},
		{Name: "db", Title: "PostgreSQL", Parameters: StackParameters{
			"DATABASE_ID":                  {Priority: 4, DisplayName: "Database"},
			"DATABASE_NAME":                {Priority: 5, DisplayName: "Database Name", DefaultValue: &dhis2DBDefaults.dbName},
			"DATABASE_PASSWORD":            {Priority: 6, DisplayName: "Database Password", DefaultValue: &dhis2DBDefaults.dbPassword, Sensitive: true},
			"DATABASE_SIZE":                {Priority: 7, DisplayName: "Database Size", DefaultValue: &dhis2DBDefaults.dbSize},
			"DATABASE_USERNAME":            {Priority: 8, DisplayName: "Database Username", DefaultValue: &dhis2DBDefaults.dbUsername, Sensitive: true},
			"DB_RESOURCES_REQUESTS_CPU":    {Priority: 22, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
			"DB_RESOURCES_REQUESTS_MEMORY": {Priority: 23, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
		}},
		{Name: "minio", Title: "Storage: MinIO", When: whenStorageIsMinio, Parameters: StackParameters{
			"MINIO_STORAGE_SIZE": {Priority: 11, DisplayName: "Storage size", DefaultValue: &minIODefaults.storageSize},
		}},
		{Name: "s3", Title: "Storage: S3", When: whenStorageIsS3, Parameters: StackParameters{
			"S3_BUCKET":   {Priority: 13, DisplayName: "Bucket", DefaultValue: &dhis2CoreDefaults.s3Bucket},
			"S3_REGION":   {Priority: 14, DisplayName: "Region", DefaultValue: &dhis2CoreDefaults.s3Region, Sensitive: true},
			"S3_IDENTITY": {Priority: 15, DisplayName: "Identity", DefaultValue: &dhis2CoreDefaults.s3Identity, Sensitive: true},
			"S3_SECRET":   {Priority: 16, DisplayName: "Secret", DefaultValue: &dhis2CoreDefaults.s3Secret, Sensitive: true},
		}},
		// The settings that belong to the cluster rather than to either tier live with the frontend,
		// which is what DHIS 2 and the provisioning job connect to, the same way chap keeps its
		// release-wide settings with its api.
		{Name: "doris-fe", Title: "Doris frontend", When: whenDorisIsEnabled, Parameters: StackParameters{
			"DORIS_FRONTEND_VERSION":                   {Priority: 41, DisplayName: "Doris Version", DefaultValue: &dhis2V2Defaults.dorisVersion},
			"DORIS_DATABASE":                           {Priority: 42, DisplayName: "Database Name", DefaultValue: &dhis2V2Defaults.dorisDatabase},
			"DORIS_USERNAME":                           {Priority: 43, DisplayName: "Username", DefaultValue: &dhis2V2Defaults.dorisUsername, Sensitive: true},
			"DORIS_PASSWORD":                           {Priority: 44, DisplayName: "Password", DefaultValue: &dhis2V2Defaults.dorisPassword, Sensitive: true},
			"DORIS_FRONTEND_REPLICAS":                  {Priority: 45, DisplayName: "Replicas", DefaultValue: &dhis2V2Defaults.dorisReplicas},
			"DORIS_FRONTEND_RESOURCES_REQUESTS_CPU":    {Priority: 46, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2V2Defaults.dorisFrontendRequestsCPU},
			"DORIS_FRONTEND_RESOURCES_REQUESTS_MEMORY": {Priority: 47, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2V2Defaults.dorisFrontendRequestsMemory},
			"DORIS_FRONTEND_RESOURCES_LIMITS_CPU":      {Priority: 48, DisplayName: "Resources Limits CPU", DefaultValue: &dhis2V2Defaults.dorisFrontendLimitsCPU},
			"DORIS_FRONTEND_RESOURCES_LIMITS_MEMORY":   {Priority: 49, DisplayName: "Resources Limits Memory", DefaultValue: &dhis2V2Defaults.dorisFrontendLimitsMemory},
		}},
		// The backend carries its own version because the tiers are separate images, though they are
		// meant to run the same version outside of an upgrade. It is sized separately because it is
		// the tier that holds the data and does the work.
		{Name: "doris-be", Title: "Doris backend", When: whenDorisIsEnabled, Parameters: StackParameters{
			"DORIS_BACKEND_VERSION":                   {Priority: 50, DisplayName: "Doris Version", DefaultValue: &dhis2V2Defaults.dorisVersion},
			"DORIS_BACKEND_REPLICAS":                  {Priority: 51, DisplayName: "Replicas", DefaultValue: &dhis2V2Defaults.dorisReplicas},
			"DORIS_BACKEND_RESOURCES_REQUESTS_CPU":    {Priority: 52, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2V2Defaults.dorisBackendRequestsCPU},
			"DORIS_BACKEND_RESOURCES_REQUESTS_MEMORY": {Priority: 53, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2V2Defaults.dorisBackendRequestsMemory},
			"DORIS_BACKEND_RESOURCES_LIMITS_CPU":      {Priority: 54, DisplayName: "Resources Limits CPU", DefaultValue: &dhis2V2Defaults.dorisBackendLimitsCPU},
			"DORIS_BACKEND_RESOURCES_LIMITS_MEMORY":   {Priority: 55, DisplayName: "Resources Limits Memory", DefaultValue: &dhis2V2Defaults.dorisBackendLimitsMemory},
		}},
		{Name: "filesystem", Title: "Storage: Filesystem", When: whenStorageIsFilesystem, Parameters: StackParameters{
			"FILESYSTEM_VOLUME_SIZE": {Priority: 12, DisplayName: "Volume size", DefaultValue: &dhis2CoreDefaults.filesystemVolumeSize, Sensitive: true},
		}},
	},
	ParameterProviders: ParameterProviders{
		"DATABASE_HOSTNAME": dhis2V2PostgresHostnameProvider,
	},
	Companions: []Companion{
		{Stack: Chap, When: whenChapIsDeployed},
		// Unconditional: pgAdmin consumes the database connection parameters this stack provides, so
		// it can always be offered. The opt-in is the form's checkbox rather than a condition.
		{Stack: PgAdmin},
	},
	Components: []kube.Component{
		DHIS2CoreComponent{BaseComponent: kube.BaseComponent{
			Name:        "dhis2",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s,app.kubernetes.io/name=dhis2"},
			Capabilities: []kube.Capability{
				{Operation: kube.OperationFilestoreBackup},
			},
		}},
		CNPGPostgresComponent{BaseComponent: kube.BaseComponent{
			Name:        "db",
			PVCPatterns: []string{"cnpg.io/cluster=%s-dhis2-postgresql"},
			Capabilities: []kube.Capability{
				{Operation: kube.OperationDatabaseSave},
			},
		}, ClusterPattern: "%s-dhis2-postgresql"},
		MinioComponent{BaseComponent: kube.BaseComponent{
			Name:        "minio",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s,app.kubernetes.io/name=minio"},
			When:        whenStorageIsMinio,
		}},
		// No PVC patterns: the chart leaves the Doris cluster's persistentVolumeClaim unset, so the
		// operator gives both tiers ephemeral storage and destroy has no volumes to clean up. Giving
		// Doris volumes is a chart change, and this is where the selectors would follow.
		DorisComponent{BaseComponent: kube.BaseComponent{
			Name: "doris-fe",
			When: whenDorisIsEnabled,
		}, ClusterPattern: "%s-doris", Tier: "fe"},
		DorisComponent{BaseComponent: kube.BaseComponent{
			Name: "doris-be",
			When: whenDorisIsEnabled,
		}, ClusterPattern: "%s-doris", Tier: "be"},
	},
})

var dhis2V2Defaults = struct {
	chartVersion  string
	enableDoris   string
	dorisVersion  string
	dorisDatabase string
	dorisUsername string
	dorisPassword string
	dorisReplicas string

	dorisFrontendRequestsCPU    string
	dorisFrontendRequestsMemory string
	dorisFrontendLimitsCPU      string
	dorisFrontendLimitsMemory   string

	dorisBackendRequestsCPU    string
	dorisBackendRequestsMemory string
	dorisBackendLimitsCPU      string
	dorisBackendLimitsMemory   string
}{
	chartVersion: "1.0.1",
	enableDoris:  "false",
	// The tag suffix is dhis2/apache-doris, upstream Doris with the PostgreSQL JDBC driver already in
	// its jdbc_drivers directory, so the chart refers to the driver by name instead of downloading it.
	dorisVersion:  "4.0.8",
	dorisDatabase: "dhis2",
	dorisUsername: "dhis2",
	dorisPassword: "dhis2",
	dorisReplicas: "1",
	// Each tier is sized on its own because they are different programs with different appetites, not
	// two copies of one. The CPU numbers size both down from the 8 per tier the upstream chart asks
	// for, which does not schedule on our nodes.
	//
	// The frontend is a JVM whose fe.conf pins an 8Gi heap, so an 8Gi limit is exactly the heap with
	// no room for the JVM itself and the tier is OOMKilled as soon as it starts. 8Gi requested with
	// 16Gi to grow into is what actually runs.
	dorisFrontendRequestsCPU:    "1",
	dorisFrontendRequestsMemory: "8Gi",
	dorisFrontendLimitsCPU:      "4",
	dorisFrontendLimitsMemory:   "16Gi",

	// The backend is C++ and sizes itself from what the container allows rather than from a pinned
	// heap, so the frontend's floor does not apply to it. It starts at the same numbers because it is
	// the tier holding the data and doing the work, and it should not be the smaller of the two.
	dorisBackendRequestsCPU:    "1",
	dorisBackendRequestsMemory: "8Gi",
	dorisBackendLimitsCPU:      "4",
	dorisBackendLimitsMemory:   "16Gi",
}

// Doris is bundled by the chart rather than being a stack of its own, the same way MinIO is, so the
// condition gates both the parameters and the component.
var whenDorisIsEnabled = &kube.Condition{Parameter: "ENABLE_DORIS", Equals: "true"}

// The storage conditions mirror the chart's own gating of each file store backend on STORAGE_TYPE.
// The minio condition doubles as the minio component's presence predicate, so the deploy form's
// section visibility and the backend's component listing cannot disagree.

// The storage conditions mirror the chart's own gating of each file store backend on STORAGE_TYPE.
// The minio condition doubles as the minio component's presence predicate, so the deploy form's
// section visibility and the backend's component listing cannot disagree.
var (
	whenStorageIsMinio      = &kube.Condition{Parameter: "STORAGE_TYPE", Equals: minIOStorage}
	whenStorageIsS3         = &kube.Condition{Parameter: "STORAGE_TYPE", Equals: s3Storage}
	whenStorageIsFilesystem = &kube.Condition{Parameter: "STORAGE_TYPE", Equals: filesystemStorage}
)

// Provides the PostgreSQL hostname of a dhis2-v2 instance: the CNPG cluster's read-write service,
// named after the chart fullname (<release>-dhis2).

// Provides the PostgreSQL hostname of a dhis2-v2 instance: the CNPG cluster's read-write service,
// named after the chart fullname (<release>-dhis2).
var dhis2V2PostgresHostnameProvider = ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-%d-dhis2-postgresql-rw.%s.svc", instance.Name, instance.Group.ID, instance.Group.Namespace), nil
})
