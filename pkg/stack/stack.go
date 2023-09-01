// Package stack contains stacks that can be deployed with the instance manager. Stacks have
// parameters as their input which are used to render helmfile templates. Stacks might depend
// on other stacks to provide a parameter (consumed parameter). Stack parameters declared here are
// kept in sync with the helmfile template. No cycle is allowed within our stacks as this would lead
// to undeployable stacks. No two stacks are allowed to provide the same parameter for another stack
// as this is an ambiguity that cannot be automatically resolved.
package stack

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// Stacks represents all deployable stacks.
type Stacks map[string]model.Stack

// New creates stacks ensuring consumed parameters are provided by required stacks.
func New(stacks ...model.Stack) (Stacks, error) {
	err := validateConsumedParams(stacks)
	if err != nil {
		return nil, err
	}

	result := make(Stacks, len(stacks))
	for _, s := range stacks {
		result[s.Name] = s
	}
	return result, nil
}

// validateConsumedParams validates all consumed parameters are provided by exactly one of the
// required stacks. Required stacks need to provide at least one consumed parameter.
func validateConsumedParams(stacks []model.Stack) error {
	var errs []error
	for _, stack := range stacks { // validate each stacks consumed parameters are provided by its required stacks
		requiredStacks := make(map[string]int)
		for _, requiredStack := range stack.Requires {
			_, ok := requiredStacks[requiredStack.Name]
			if ok {
				return fmt.Errorf("stack %q requires %q more than once", stack.Name, requiredStack.Name)
			}
			requiredStacks[requiredStack.Name] = 0
		}

		// collect all consumed parameters
		consumedParameterProviders := make(map[string]int)
		for name, parameter := range stack.Parameters {
			if !parameter.Consumed {
				continue
			}
			consumedParameterProviders[name] = 0
		}

		// generate frequency map of provided parameters
		for _, requiredStack := range stack.Requires {
			for name := range requiredStack.Parameters {
				_, ok := consumedParameterProviders[name]
				if ok {
					consumedParameterProviders[name]++
					requiredStacks[requiredStack.Name]++
				}
			}
			for name := range requiredStack.Providers {
				_, ok := consumedParameterProviders[name]
				if ok {
					consumedParameterProviders[name]++
					requiredStacks[requiredStack.Name]++
				}
			}
		}
		for parameter, providerCount := range consumedParameterProviders {
			if providerCount == 0 {
				errs = append(errs, fmt.Errorf("no provider for stack %q parameter %q", stack.Name, parameter))
			}
			if providerCount > 1 {
				errs = append(errs, fmt.Errorf("every consumed parameter must have exactly one provider. %d provider(s) for stack %q parameter %q", providerCount, stack.Name, parameter))
			}
		}

		for requiredStackName, providedCount := range requiredStacks {
			if providedCount == 0 {
				errs = append(errs, fmt.Errorf("stack %q requires %q but does not consume from %q", stack.Name, requiredStackName, requiredStackName))
			}
		}
	}

	return errors.Join(errs...)
}

const ifNotPresent = "IfNotPresent"

// Stack representing ../../stacks/dhis2-db/helmfile.yaml
var DHIS2DB = model.Stack{
	Name: "dhis2-db",
	Parameters: model.StackParameters{
		"CHART_VERSION":             {DefaultValue: &dhis2DBDefaults.chartVersion},
		"DATABASE_ID":               {},
		"DATABASE_NAME":             {DefaultValue: &dhis2DBDefaults.dbName},
		"DATABASE_PASSWORD":         {DefaultValue: &dhis2DBDefaults.dbPassword},
		"DATABASE_SIZE":             {DefaultValue: &dhis2DBDefaults.dbSize},
		"DATABASE_USERNAME":         {DefaultValue: &dhis2DBDefaults.dbUsername},
		"DATABASE_VERSION":          {DefaultValue: &dhis2DBDefaults.dbVersion},
		"RESOURCES_REQUESTS_CPU":    {DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		"RESOURCES_REQUESTS_MEMORY": {DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
	},
	Providers: model.Providers{
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
	Parameters: model.StackParameters{
		"CHART_VERSION":                   {DefaultValue: &dhis2CoreDefaults.chartVersion},
		"DATABASE_HOSTNAME":               {Consumed: true},
		"DATABASE_NAME":                   {Consumed: true},
		"DATABASE_PASSWORD":               {Consumed: true},
		"DATABASE_USERNAME":               {Consumed: true},
		"DHIS2_HOME":                      {DefaultValue: &dhis2CoreDefaults.dhis2Home},
		"FLYWAY_MIGRATE_OUT_OF_ORDER":     {DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		"FLYWAY_REPAIR_BEFORE_MIGRATION":  {DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		"IMAGE_PULL_POLICY":               {DefaultValue: &dhis2CoreDefaults.imagePullPolicy},
		"IMAGE_REPOSITORY":                {DefaultValue: &dhis2CoreDefaults.imageRepository},
		"IMAGE_TAG":                       {DefaultValue: &dhis2CoreDefaults.imageTag},
		"JAVA_OPTS":                       {DefaultValue: &dhis2CoreDefaults.javaOpts},
		"LIVENESS_PROBE_TIMEOUT_SECONDS":  {DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		"READINESS_PROBE_TIMEOUT_SECONDS": {DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		"RESOURCES_REQUESTS_CPU":          {DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		"RESOURCES_REQUESTS_MEMORY":       {DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		"STARTUP_PROBE_FAILURE_THRESHOLD": {DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		"STARTUP_PROBE_PERIOD_SECONDS":    {DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
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
	Parameters: model.StackParameters{
		"CHART_VERSION":                   {DefaultValue: &dhis2CoreDefaults.chartVersion},
		"CORE_RESOURCES_REQUESTS_CPU":     {DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		"CORE_RESOURCES_REQUESTS_MEMORY":  {DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		"DATABASE_ID":                     {},
		"DATABASE_NAME":                   {DefaultValue: &dhis2DBDefaults.dbName},
		"DATABASE_PASSWORD":               {DefaultValue: &dhis2DBDefaults.dbPassword},
		"DATABASE_SIZE":                   {DefaultValue: &dhis2DBDefaults.dbSize},
		"DATABASE_USERNAME":               {DefaultValue: &dhis2DBDefaults.dbUsername},
		"DATABASE_VERSION":                {DefaultValue: &dhis2DBDefaults.dbVersion},
		"DB_RESOURCES_REQUESTS_CPU":       {DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		"DB_RESOURCES_REQUESTS_MEMORY":    {DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
		"DHIS2_HOME":                      {DefaultValue: &dhis2CoreDefaults.dhis2Home},
		"FLYWAY_MIGRATE_OUT_OF_ORDER":     {DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		"FLYWAY_REPAIR_BEFORE_MIGRATION":  {DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		"IMAGE_PULL_POLICY":               {DefaultValue: &dhis2CoreDefaults.imagePullPolicy},
		"IMAGE_REPOSITORY":                {DefaultValue: &dhis2CoreDefaults.imageRepository},
		"IMAGE_TAG":                       {DefaultValue: &dhis2CoreDefaults.imageTag},
		"INSTALL_REDIS":                   {DefaultValue: &dhis2Defaults.installRedis},
		"JAVA_OPTS":                       {DefaultValue: &dhis2CoreDefaults.javaOpts},
		"LIVENESS_PROBE_TIMEOUT_SECONDS":  {DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		"READINESS_PROBE_TIMEOUT_SECONDS": {DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		"STARTUP_PROBE_FAILURE_THRESHOLD": {DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		"STARTUP_PROBE_PERIOD_SECONDS":    {DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
	},
	Providers: model.Providers{
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
	Parameters: model.StackParameters{
		"CHART_VERSION":     {DefaultValue: &pgAdminDefaults.chartVersion},
		"DATABASE_HOSTNAME": {Consumed: true},
		"DATABASE_NAME":     {Consumed: true},
		"DATABASE_USERNAME": {Consumed: true},
		"PGADMIN_PASSWORD":  {},
		"PGADMIN_USERNAME":  {},
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
	Parameters: model.StackParameters{
		"CHART_VERSION":     {DefaultValue: &whoamiGoDefaults.chartVersion},
		"IMAGE_PULL_POLICY": {DefaultValue: &whoamiGoDefaults.imagePullPolicy},
		"IMAGE_REPOSITORY":  {DefaultValue: &whoamiGoDefaults.imageRepository},
		"IMAGE_TAG":         {DefaultValue: &whoamiGoDefaults.imageTag},
		"REPLICA_COUNT":     {DefaultValue: &whoamiGoDefaults.replicaCount},
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
	Parameters: model.StackParameters{
		"CHART_VERSION":           {DefaultValue: &imJobRunnerDefaults.chartVersion},
		"COMMAND":                 {},
		"DHIS2_DATABASE_DATABASE": {DefaultValue: &dhis2DBDefaults.dbName},
		"DHIS2_DATABASE_HOSTNAME": {DefaultValue: &imJobRunnerDefaults.dbHostname},
		"DHIS2_DATABASE_PASSWORD": {DefaultValue: &dhis2DBDefaults.dbPassword},
		"DHIS2_DATABASE_PORT":     {DefaultValue: &imJobRunnerDefaults.dbPort},
		"DHIS2_DATABASE_USERNAME": {DefaultValue: &dhis2DBDefaults.dbUsername},
		"DHIS2_HOSTNAME":          {DefaultValue: &imJobRunnerDefaults.dhis2Hostname},
		"PAYLOAD":                 {DefaultValue: &imJobRunnerDefaults.payload},
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
