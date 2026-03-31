package stack

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Stack representing ../../stacks/chap-db/helmfile.yaml.gotmpl
var ChapDB = model.Stack{
	Name: "chap-db",
	Parameters: model.StackParameters{
		"DATABASE_SIZE":     {Priority: 1, DisplayName: "Database Size", DefaultValue: &chapDBDefaults.dbSize},
		"DATABASE_NAME":     {Priority: 2, DisplayName: "Database Name", DefaultValue: &chapDBDefaults.dbName},
		"DATABASE_PASSWORD": {Priority: 3, DisplayName: "Database Password", DefaultValue: &chapDBDefaults.dbPassword, Sensitive: true},
		"DATABASE_VERSION":  {Priority: 4, DisplayName: "Database Version", DefaultValue: &chapDBDefaults.dbVersion},
		"CHART_VERSION":     {Priority: 5, DisplayName: "Chart Version", DefaultValue: &chapDBDefaults.chartVersion},
	},
	ParameterProviders: model.ParameterProviders{
		"DATABASE_HOSTNAME": chapDBHostnameProvider,
		"DATABASE_SECRET":   chapDBSecretProvider,
	},
	KubernetesResource: model.StatefulSetResource,
}

var chapDBHostnameProvider = model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-chap-db-postgres-rw.%s.svc", instance.Name, instance.Group.Namespace), nil
})

var chapDBSecretProvider = model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-chap-db-postgres", instance.Name), nil
})

var chapDBDefaults = struct {
	chartVersion string
	dbSize       string
	dbName       string
	dbPassword   string
	dbVersion    string
}{
	chartVersion: "0.1.0",
	dbSize:       "10Gi",
	dbName:       "chap_core",
	dbPassword:   "chap",
	dbVersion:    "17",
}

// Stack representing ../../stacks/chap-valkey/helmfile.yaml.gotmpl
var ChapValkey = model.Stack{
	Name: "chap-valkey",
	Parameters: model.StackParameters{
		"REDIS_STORAGE_SIZE": {Priority: 1, DisplayName: "Redis Storage Size", DefaultValue: &chapValkeyDefaults.storageSize},
		"REDIS_PASSWORD":     {Priority: 2, DisplayName: "Redis Password", DefaultValue: &chapValkeyDefaults.password, Sensitive: true},
		"CHART_VERSION":      {Priority: 3, DisplayName: "Chart Version", DefaultValue: &chapValkeyDefaults.chartVersion},
	},
	ParameterProviders: model.ParameterProviders{
		"REDIS_HOST":   chapValkeyHostnameProvider,
		"REDIS_SECRET": chapValkeySecretProvider,
	},
	KubernetesResource: model.StatefulSetResource,
}

var chapValkeyHostnameProvider = model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-chap-valkey.%s.svc", instance.Name, instance.Group.Namespace), nil
})

var chapValkeySecretProvider = model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-chap-valkey-auth", instance.Name), nil
})

var chapValkeyDefaults = struct {
	chartVersion string
	storageSize  string
	password     string
}{
	chartVersion: "0.9.2",
	storageSize:  "10Gi",
	password:     "chap",
}

// Stack representing ../../stacks/chap-worker/helmfile.yaml.gotmpl
var ChapWorker = model.Stack{
	Name: "chap-worker",
	Parameters: model.StackParameters{
		"IMAGE_TAG":         {Priority: 1, DisplayName: "Image Tag", DefaultValue: &chapWorkerDefaults.imageTag},
		"IMAGE_PULL_POLICY": {Priority: 2, DisplayName: "Image Pull Policy", DefaultValue: &chapWorkerDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"CHART_VERSION":     {Priority: 3, DisplayName: "Chart Version", DefaultValue: &chapWorkerDefaults.chartVersion},
		"DATABASE_HOSTNAME": {Priority: 0, DisplayName: "Database Hostname", Consumed: true},
		"DATABASE_SECRET":   {Priority: 0, DisplayName: "Database Secret", Consumed: true},
		"DATABASE_NAME":     {Priority: 0, DisplayName: "Database Name", Consumed: true},
		"REDIS_HOST":        {Priority: 0, DisplayName: "Redis Host", Consumed: true},
		"REDIS_SECRET":      {Priority: 0, DisplayName: "Redis Secret", Consumed: true},
	},
	Requires:           []model.Stack{ChapDB, ChapValkey},
	KubernetesResource: model.DeploymentResource,
}

var chapWorkerDefaults = struct {
	chartVersion    string
	imageTag        string
	imagePullPolicy string
}{
	chartVersion:    "0.1.0",
	imageTag:        "latest",
	imagePullPolicy: ifNotPresent,
}

// Stack representing ../../stacks/chap-core/helmfile.yaml.gotmpl
var ChapCore = model.Stack{
	Name: "chap-core",
	Parameters: model.StackParameters{
		"IMAGE_TAG":                          {Priority: 1, DisplayName: "Image Tag", DefaultValue: &chapCoreDefaults.imageTag},
		"IMAGE_PULL_POLICY":                  {Priority: 2, DisplayName: "Image Pull Policy", DefaultValue: &chapCoreDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"CHART_VERSION":                      {Priority: 3, DisplayName: "Chart Version", DefaultValue: &chapCoreDefaults.chartVersion},
		"GOOGLE_SERVICE_ACCOUNT_EMAIL":       {Priority: 4, DisplayName: "Google Service Account Email", DefaultValue: &chapCoreDefaults.googleServiceAccountEmail, Sensitive: true},
		"GOOGLE_SERVICE_ACCOUNT_PRIVATE_KEY": {Priority: 5, DisplayName: "Google Service Account Key", DefaultValue: &chapCoreDefaults.googleServiceAccountPrivateKey, Sensitive: true},
		"DATABASE_HOSTNAME":                  {Priority: 0, DisplayName: "Database Hostname", Consumed: true},
		"DATABASE_SECRET":                    {Priority: 0, DisplayName: "Database Secret", Consumed: true},
		"DATABASE_NAME":                      {Priority: 0, DisplayName: "Database Name", Consumed: true},
		"REDIS_HOST":                         {Priority: 0, DisplayName: "Redis Host", Consumed: true},
		"REDIS_SECRET":                       {Priority: 0, DisplayName: "Redis Secret", Consumed: true},
	},
	Requires:           []model.Stack{ChapDB, ChapValkey},
	Companions:         []model.Stack{ChapWorker},
	KubernetesResource: model.DeploymentResource,
}

var chapCoreDefaults = struct {
	chartVersion                   string
	imageTag                       string
	imagePullPolicy                string
	googleServiceAccountEmail      string
	googleServiceAccountPrivateKey string
}{
	chartVersion:                   "0.1.0",
	imageTag:                       "latest",
	imagePullPolicy:                ifNotPresent,
	googleServiceAccountEmail:      " ",
	googleServiceAccountPrivateKey: " ",
}
