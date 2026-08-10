package stack

import "github.com/dhis2-sre/im-manager/pkg/kube"

// Stack representing ../../stacks/dhis2-core/helmfile.yaml.gotmpl
var DHIS2Core = Stack{
	Name: "dhis2-core",
	Parameters: StackParameters{
		"IMAGE_TAG":                       {Priority: 1, DisplayName: "Image Tag", DefaultValue: &dhis2CoreDefaults.imageTag},
		"IMAGE_REPOSITORY":                {Priority: 2, DisplayName: "Image Repository", DefaultValue: &dhis2CoreDefaults.imageRepository},
		"IMAGE_PULL_POLICY":               {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &dhis2CoreDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"STORAGE_TYPE":                    {Priority: 4, DisplayName: "Storage type", DefaultValue: &dhis2CoreDefaults.storageType, Validator: storage, RequireCompanion: storageCompanionProvider},
		"S3_BUCKET":                       {Priority: 5, DisplayName: "S3 bucket", DefaultValue: &dhis2CoreDefaults.s3Bucket},
		"S3_REGION":                       {Priority: 6, DisplayName: "S3 region", DefaultValue: &dhis2CoreDefaults.s3Region, Sensitive: true},
		"S3_IDENTITY":                     {Priority: 7, DisplayName: "S3 identity", DefaultValue: &dhis2CoreDefaults.s3Identity, Sensitive: true},
		"S3_SECRET":                       {Priority: 8, DisplayName: "S3 secret", DefaultValue: &dhis2CoreDefaults.s3Secret, Sensitive: true},
		"DHIS2_HOME":                      {Priority: 9, DisplayName: "DHIS2 Home Directory", DefaultValue: &dhis2CoreDefaults.dhis2Home},
		"FLYWAY_MIGRATE_OUT_OF_ORDER":     {Priority: 10, DisplayName: "Flyway Migrate Out Of Order", DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		"FLYWAY_REPAIR_BEFORE_MIGRATION":  {Priority: 11, DisplayName: "Flyway Repair Before Migration", DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		"RESOURCES_REQUESTS_CPU":          {Priority: 12, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		"RESOURCES_REQUESTS_MEMORY":       {Priority: 13, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		"MIN_READY_SECONDS":               {Priority: 14, DisplayName: "Minimum Ready Seconds", DefaultValue: &dhis2CoreDefaults.minReadySeconds},
		"LIVENESS_PROBE_TIMEOUT_SECONDS":  {Priority: 15, DisplayName: "Liveness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		"READINESS_PROBE_TIMEOUT_SECONDS": {Priority: 16, DisplayName: "Readiness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		"STARTUP_PROBE_FAILURE_THRESHOLD": {Priority: 17, DisplayName: "Startup Probe Failure Threshold", DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		"STARTUP_PROBE_PERIOD_SECONDS":    {Priority: 18, DisplayName: "Startup Probe Period Seconds", DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
		"JAVA_OPTS":                       {Priority: 19, DisplayName: "JAVA_OPTS", DefaultValue: &dhis2CoreDefaults.javaOpts},
		"CHART_VERSION":                   {Priority: 20, DisplayName: "Chart Version", DefaultValue: &dhis2CoreDefaults.chartVersion},
		"ENABLE_QUERY_LOGGING":            {Priority: 21, DisplayName: "Enable Query Logging", DefaultValue: &dhis2CoreDefaults.enableQueryLogging},
		"FILESYSTEM_VOLUME_SIZE":          {Priority: 22, DisplayName: "Filesystem volume size (only in effect if \"Storage\" is set to \"filesystem\")", DefaultValue: &dhis2CoreDefaults.filesystemVolumeSize, Sensitive: true},
		"SAME_SITE_COOKIES":               {Priority: 23, DisplayName: "Same site cookies", DefaultValue: &dhis2CoreDefaults.sameSiteCookies, Validator: sameSiteCookies},
		"CUSTOM_DHIS2_CONFIG":             {Priority: 24, DisplayName: "Custom DHIS2 config (applied to top of dhis.conf)", DefaultValue: &dhis2CoreDefaults.customDhis2Config, Sensitive: true},
		"ALLOW_SUSPEND":                   {Priority: 25, DisplayName: "Allow the application to be suspended", DefaultValue: &dhis2CoreDefaults.allowSuspend},
		"DEPLOY_GLOWROOT":                 {Priority: 26, DisplayName: "Deploy Glowroot", DefaultValue: &dhis2CoreDefaults.deployGlowroot},
		"DEPLOY_CHAP":                     {Priority: 27, DisplayName: "Deploy CHAP", DefaultValue: &dhis2CoreDefaults.deployChap},
		"GOOGLE_AUTH_PROJECT_ID":          {Priority: 0, DisplayName: "Google auth project id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId, Sensitive: true},
		"GOOGLE_AUTH_PRIVATE_KEY":         {Priority: 0, DisplayName: "Google auth private key", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKey, Sensitive: true},
		"GOOGLE_AUTH_PRIVATE_KEY_ID":      {Priority: 0, DisplayName: "Google auth private key id", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKeyId, Sensitive: true},
		"GOOGLE_AUTH_CLIENT_EMAIL":        {Priority: 0, DisplayName: "Google auth client email", DefaultValue: &dhis2CoreDefaults.googleAuthClientEmail, Sensitive: true},
		"GOOGLE_AUTH_CLIENT_ID":           {Priority: 0, DisplayName: "Google auth client id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId, Sensitive: true},
		"DATABASE_HOSTNAME":               {Priority: 0, DisplayName: "Database Hostname", Consumed: true},
		"DATABASE_NAME":                   {Priority: 0, DisplayName: "Database Name", Consumed: true},
		"DATABASE_PASSWORD":               {Priority: 0, DisplayName: "Database Password", Consumed: true, Sensitive: true},
		"DATABASE_USERNAME":               {Priority: 0, DisplayName: "Database Username", Consumed: true, Sensitive: true},
	},
	Requires: []Stack{
		DHIS2DB,
	},
	Companions: []Stack{
		MINIO,
		Chap,
	},
	Components: []kube.Component{
		DHIS2CoreComponent{BaseComponent: kube.BaseComponent{
			Name: "dhis2",
			PVCPatterns: []string{
				"app.kubernetes.io/instance=%s",
				"app.kubernetes.io/instance=%s-minio",
			},
			// All STORAGE_TYPE backends (minio, filesystem, s3) support filestore backup, so no
			// predicate; the first parameter-gated capability arrives with CNPG-backed stacks.
			Capabilities: []kube.Capability{
				{Operation: kube.OperationFilestoreBackup},
			},
		}},
	},
}

var dhis2CoreDefaults = struct {
	chartVersion                 string
	storageType                  string
	sameSiteCookies              string
	filesystemVolumeSize         string
	s3Bucket                     string
	s3Region                     string
	s3Identity                   string
	s3Secret                     string
	dhis2Home                    string
	flywayMigrateOutOfOrder      string
	flywayRepairBeforeMigration  string
	enableQueryLogging           string
	imagePullPolicy              string
	imageRepository              string
	imageTag                     string
	javaOpts                     string
	minReadySeconds              string
	livenessProbeTimeoutSeconds  string
	readinessProbeTimeoutSeconds string
	resourcesRequestsCPU         string
	resourcesRequestsMemory      string
	startupProbeFailureThreshold string
	startupProbePeriodSeconds    string
	customDhis2Config            string
	allowSuspend                 string
	deployGlowroot               string
	deployChap                   string
	googleAuthProjectId          string
	googleAuthPrivateKey         string
	googleAuthPrivateKeyId       string
	googleAuthClientEmail        string
	googleAuthClientId           string
}{
	chartVersion:                 "0.34.11",
	storageType:                  minIOStorage,
	sameSiteCookies:              lax,
	filesystemVolumeSize:         "8Gi",
	s3Bucket:                     "dhis2",
	s3Region:                     "eu-west-1",
	s3Identity:                   "-",
	s3Secret:                     "-",
	dhis2Home:                    "/opt/dhis2",
	flywayMigrateOutOfOrder:      "false",
	flywayRepairBeforeMigration:  "false",
	enableQueryLogging:           "false",
	imagePullPolicy:              ifNotPresent,
	imageRepository:              "core",
	imageTag:                     "2.40.2",
	javaOpts:                     " ", // " " is used here since an empty string would be interpreted by helmfile as the environment variable not being set. And since all variables are required an empty string would result in an error
	minReadySeconds:              "5",
	livenessProbeTimeoutSeconds:  "1",
	readinessProbeTimeoutSeconds: "1",
	resourcesRequestsCPU:         "250m",
	resourcesRequestsMemory:      "1500Mi",
	startupProbeFailureThreshold: "26",
	startupProbePeriodSeconds:    "5",
	customDhis2Config:            " ",
	allowSuspend:                 "true",
	deployGlowroot:               "false",
	deployChap:                   "false",
	googleAuthProjectId:          " ", // TODO: " " doesn't need to be used here as with `javaOpts` since the googleAuth* parameters are stack parameters and therefor always populated
	googleAuthPrivateKey:         " ", // However the web client currently doesn't support these empty parameter so for now
	googleAuthPrivateKeyId:       " ",
	googleAuthClientEmail:        " ",
	googleAuthClientId:           " ",
}

// Stack representing ../../stacks/dhis2/helmfile.yaml.gotmpl
