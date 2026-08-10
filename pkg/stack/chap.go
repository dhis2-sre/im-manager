package stack

import (
	"github.com/dhis2-sre/im-manager/pkg/kube"
)

// Chap deploys the chap umbrella chart: one release bundling the CHAP api, its Celery worker,
// a CloudNativePG PostgreSQL cluster and Valkey, replacing the deployment-level composition of
// chap-db, chap-valkey, chap-worker and chap-core.
var Chap = withGroupedParameters(Stack{
	Name: "chap",
	ParameterGroups: []ParameterGroup{
		{Name: "chap", Title: "CHAP", Parameters: StackParameters{
			"IMAGE_TAG":         {Priority: 1, DisplayName: "Image Tag", DefaultValue: &chapDefaults.imageTag},
			"IMAGE_PULL_POLICY": {Priority: 2, DisplayName: "Image Pull Policy", DefaultValue: &chapDefaults.imagePullPolicy, Validator: imagePullPolicy},
			"CHART_VERSION":     {Priority: 3, DisplayName: "Chart Version", DefaultValue: &chapDefaults.chartVersion},
			"DHIS2_REGISTER":    {Priority: 4, DisplayName: "Register as a route in DHIS 2", DefaultValue: &chapDefaults.dhis2Register},
		}},
		{Name: "dhis2", Title: "DHIS 2 registration", When: whenDhis2RegisterIsEnabled, Parameters: StackParameters{
			"DHIS2_USERNAME": {Priority: 5, DisplayName: "DHIS2 Username", DefaultValue: &chapDefaults.dhis2Username},
			"DHIS2_PASSWORD": {Priority: 6, DisplayName: "DHIS2 Password", DefaultValue: &chapDefaults.dhis2Password, Sensitive: true},
		}},
		{Name: "db", Title: "PostgreSQL", Parameters: StackParameters{
			"DATABASE_NAME":     {Priority: 7, DisplayName: "Database Name", DefaultValue: &chapDefaults.dbName},
			"DATABASE_PASSWORD": {Priority: 8, DisplayName: "Database Password", DefaultValue: &chapDefaults.dbPassword, Sensitive: true},
			"DATABASE_SIZE":     {Priority: 9, DisplayName: "Database Size", DefaultValue: &chapDefaults.dbSize},
			"DATABASE_VERSION":  {Priority: 10, DisplayName: "Database Version", DefaultValue: &chapDefaults.dbVersion},
		}},
		{Name: "valkey", Title: "Valkey", Parameters: StackParameters{
			"REDIS_PASSWORD":     {Priority: 11, DisplayName: "Valkey Password", DefaultValue: &chapDefaults.redisPassword, Sensitive: true},
			"REDIS_STORAGE_SIZE": {Priority: 12, DisplayName: "Valkey Storage Size", DefaultValue: &chapDefaults.redisStorageSize},
		}},
	},
	Components: []kube.Component{
		ChapAPIComponent{BaseComponent: kube.BaseComponent{Name: "api"}},
		ChapWorkerComponent{BaseComponent: kube.BaseComponent{Name: "worker"}},
		CNPGPostgresComponent{BaseComponent: kube.BaseComponent{
			Name:        "db",
			PVCPatterns: []string{"cnpg.io/cluster=%s-chap-db"},
		}, ClusterPattern: "%s-chap-db"},
		ValkeyComponent{BaseComponent: kube.BaseComponent{
			Name:        "valkey",
			PVCPatterns: []string{"app.kubernetes.io/instance=%s-chap,app.kubernetes.io/name=valkey"},
		}},
	},
})

// The DHIS 2 credentials are only needed when CHAP registers itself as a route, which mirrors the
// chart's own gating of the register job on dhis2.enabled.
var whenDhis2RegisterIsEnabled = &kube.Condition{Parameter: "DHIS2_REGISTER", Equals: "true"}

// Gates chap as a companion on the DEPLOY_CHAP parameter of the DHIS 2 stack offering it, which is
// the same parameter that opens DHIS 2 up to the route chap registers.
var whenChapIsDeployed = &kube.Condition{Parameter: "DEPLOY_CHAP", Equals: "true"}

var chapDefaults = struct {
	chartVersion     string
	imageTag         string
	imagePullPolicy  string
	dhis2Register    string
	dhis2Username    string
	dhis2Password    string
	dbName           string
	dbPassword       string
	dbSize           string
	dbVersion        string
	redisPassword    string
	redisStorageSize string
}{
	chartVersion:     "1.0.0",
	imageTag:         "v2.1.0",
	imagePullPolicy:  ifNotPresent,
	dhis2Register:    "true",
	dhis2Username:    "system",
	dhis2Password:    "System123",
	dbName:           "chap_core",
	dbPassword:       "chap",
	dbSize:           "10Gi",
	dbVersion:        "17",
	redisPassword:    "chap",
	redisStorageSize: "10Gi",
}
