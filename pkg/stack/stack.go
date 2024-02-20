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
	"strconv"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dominikbraun/graph"
	"golang.org/x/exp/slices"
	k8s "k8s.io/api/core/v1"
)

// Stacks represents all deployable stacks.
type Stacks map[string]model.Stack

// New creates stacks ensuring consumed parameters are provided by required stacks.
func New(stacks ...model.Stack) (Stacks, error) {
	_, err := ValidateNoCycles(stacks)
	if err != nil {
		return nil, err
	}

	err = ValidateConsumedParameters(stacks)
	if err != nil {
		return nil, err
	}

	result := make(Stacks, len(stacks))
	for _, s := range stacks {
		result[s.Name] = s
	}
	return result, nil
}

// ValidateNoCycles validates that the stacks graph does not contain a cycle. The stacks form a
// graph via the required stacks forming a directed edge from the stack to the required stack.
// Stacks with cycles would lead to undeployable instances. There would not be an order (no solution
// to topological sort) in which we could deploy instances in.
func ValidateNoCycles(stacks []model.Stack) (graph.Graph[string, model.Stack], error) {
	g := graph.New(func(stack model.Stack) string {
		return stack.Name
	}, graph.Directed(), graph.PreventCycles())

	for _, stack := range stacks {
		err := g.AddVertex(stack)
		if err != nil {
			return nil, fmt.Errorf("failed adding vertex for stack %q: %v", stack.Name, err)
		}
	}

	for _, src := range stacks {
		for _, dest := range src.Requires {
			err := g.AddEdge(src.Name, dest.Name)
			if err != nil {
				if errors.Is(err, graph.ErrEdgeAlreadyExists) {
					return nil, fmt.Errorf("stack %q requires %q more than once", src.Name, dest.Name)
				} else if errors.Is(err, graph.ErrEdgeCreatesCycle) {
					return nil, fmt.Errorf("edge from stack %q to stack %q creates a cycle", src.Name, dest.Name)
				}
				return nil, fmt.Errorf("failed adding edge from stack %q to stack %q: %v", src.Name, dest.Name, err)
			}
		}
	}

	return g, nil
}

// ValidateConsumedParameters validates all consumed parameters are provided by exactly one of the
// required stacks. Required stacks need to provide at least one consumed parameter.
func ValidateConsumedParameters(stacks []model.Stack) error {
	var errs []error
	for _, stack := range stacks { // validate each stacks consumed parameters are provided by its required stacks
		requiredStacks := make(map[string]int)
		for _, requiredStack := range stack.Requires {
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
			for parameterName, parameter := range requiredStack.Parameters {
				if parameter.Consumed { // consumed parameters cannot be provided
					continue
				}
				_, ok := consumedParameterProviders[parameterName]
				if ok {
					consumedParameterProviders[parameterName]++
					requiredStacks[requiredStack.Name]++
				}
			}
			for parameterName := range requiredStack.ParameterProviders {
				_, ok := consumedParameterProviders[parameterName]
				if ok {
					consumedParameterProviders[parameterName]++
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
	// TODO: Remove HostnamePattern once stacks 2.0 are the default
	HostnamePattern: "%s-database-postgresql.%s.svc",
	Name:            "dhis2-db",
	Parameters: model.StackParameters{
		"DATABASE_ID":               {Priority: 1, DisplayName: "Database"},
		"DATABASE_SIZE":             {Priority: 2, DisplayName: "Database Size", DefaultValue: &dhis2DBDefaults.dbSize},
		"DATABASE_NAME":             {Priority: 3, DisplayName: "Database Name", DefaultValue: &dhis2DBDefaults.dbName},
		"DATABASE_PASSWORD":         {Priority: 4, DisplayName: "Database Password", DefaultValue: &dhis2DBDefaults.dbPassword},
		"DATABASE_USERNAME":         {Priority: 5, DisplayName: "Database Username", DefaultValue: &dhis2DBDefaults.dbUsername},
		"DATABASE_VERSION":          {Priority: 6, DisplayName: "Database Version", DefaultValue: &dhis2DBDefaults.dbVersion},
		"RESOURCES_REQUESTS_CPU":    {Priority: 7, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2DBDefaults.resourcesRequestsCPU},
		"RESOURCES_REQUESTS_MEMORY": {Priority: 8, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2DBDefaults.resourcesRequestsMemory},
		"CHART_VERSION":             {Priority: 9, DisplayName: "Chart Version", DefaultValue: &dhis2DBDefaults.chartVersion},
	},
	ParameterProviders: model.ParameterProviders{
		"DATABASE_HOSTNAME": postgresHostnameProvider,
	},
	KubernetesResource: model.StatefulSetResource,
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
	dbSize:                  "20Gi",
	dbUsername:              "dhis",
	dbVersion:               "13",
	resourcesRequestsCPU:    "250m",
	resourcesRequestsMemory: "256Mi",
}

// Stack representing ../../stacks/dhis2-core/helmfile.yaml
var DHIS2Core = model.Stack{
	Name: "dhis2-core",
	Parameters: model.StackParameters{
		"IMAGE_TAG":                       {Priority: 1, DisplayName: "Image Tag", DefaultValue: &dhis2CoreDefaults.imageTag},
		"IMAGE_REPOSITORY":                {Priority: 2, DisplayName: "Image Repository", DefaultValue: &dhis2CoreDefaults.imageRepository},
		"IMAGE_PULL_POLICY":               {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &dhis2CoreDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"DHIS2_HOME":                      {Priority: 4, DisplayName: "DHIS2 Home Directory", DefaultValue: &dhis2CoreDefaults.dhis2Home},
		"FLYWAY_MIGRATE_OUT_OF_ORDER":     {Priority: 5, DisplayName: "Flyway Migrate Out Of Order", DefaultValue: &dhis2CoreDefaults.flywayMigrateOutOfOrder},
		"FLYWAY_REPAIR_BEFORE_MIGRATION":  {Priority: 6, DisplayName: "Flyway Repair Before Migration", DefaultValue: &dhis2CoreDefaults.flywayRepairBeforeMigration},
		"RESOURCES_REQUESTS_CPU":          {Priority: 7, DisplayName: "Resources Requests CPU", DefaultValue: &dhis2CoreDefaults.resourcesRequestsCPU},
		"RESOURCES_REQUESTS_MEMORY":       {Priority: 8, DisplayName: "Resources Requests Memory", DefaultValue: &dhis2CoreDefaults.resourcesRequestsMemory},
		"MIN_READY_SECONDS":               {Priority: 9, DisplayName: "Minimum Ready Seconds", DefaultValue: &dhis2CoreDefaults.minReadySeconds},
		"LIVENESS_PROBE_TIMEOUT_SECONDS":  {Priority: 10, DisplayName: "Liveness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.livenessProbeTimeoutSeconds},
		"READINESS_PROBE_TIMEOUT_SECONDS": {Priority: 11, DisplayName: "Readiness Probe Timeout Seconds", DefaultValue: &dhis2CoreDefaults.readinessProbeTimeoutSeconds},
		"STARTUP_PROBE_FAILURE_THRESHOLD": {Priority: 12, DisplayName: "Startup Probe Failure Threshold", DefaultValue: &dhis2CoreDefaults.startupProbeFailureThreshold},
		"STARTUP_PROBE_PERIOD_SECONDS":    {Priority: 13, DisplayName: "Startup Probe Period Seconds", DefaultValue: &dhis2CoreDefaults.startupProbePeriodSeconds},
		"JAVA_OPTS":                       {Priority: 14, DisplayName: "JAVA_OPTS", DefaultValue: &dhis2CoreDefaults.javaOpts},
		"CHART_VERSION":                   {Priority: 15, DisplayName: "Chart Version", DefaultValue: &dhis2CoreDefaults.chartVersion},
		"GOOGLE_AUTH_PROJECT_ID":          {Priority: 0, DisplayName: "Google auth project id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId},
		"GOOGLE_AUTH_PRIVATE_KEY":         {Priority: 0, DisplayName: "Google auth private key", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKey},
		"GOOGLE_AUTH_PRIVATE_KEY_ID":      {Priority: 0, DisplayName: "Google auth private key id", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKeyId},
		"GOOGLE_AUTH_CLIENT_EMAIL":        {Priority: 0, DisplayName: "Google auth client email", DefaultValue: &dhis2CoreDefaults.googleAuthClientEmail},
		"GOOGLE_AUTH_CLIENT_ID":           {Priority: 0, DisplayName: "Google auth client id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId},
		"DATABASE_HOSTNAME":               {Priority: 0, DisplayName: "Database Hostname", Consumed: true},
		"DATABASE_NAME":                   {Priority: 0, DisplayName: "Database Name", Consumed: true},
		"DATABASE_PASSWORD":               {Priority: 0, DisplayName: "Database Password", Consumed: true},
		"DATABASE_USERNAME":               {Priority: 0, DisplayName: "Database Username", Consumed: true},
	},
	Requires: []model.Stack{
		DHIS2DB,
	},
	KubernetesResource: model.DeploymentResource,
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
	minReadySeconds              string
	livenessProbeTimeoutSeconds  string
	readinessProbeTimeoutSeconds string
	resourcesRequestsCPU         string
	resourcesRequestsMemory      string
	startupProbeFailureThreshold string
	startupProbePeriodSeconds    string
	googleAuthProjectId          string
	googleAuthPrivateKey         string
	googleAuthPrivateKeyId       string
	googleAuthClientEmail        string
	googleAuthClientId           string
}{
	chartVersion:                 "0.16.0",
	dhis2Home:                    "/opt/dhis2",
	flywayMigrateOutOfOrder:      "false",
	flywayRepairBeforeMigration:  "false",
	imagePullPolicy:              ifNotPresent,
	imageRepository:              "core",
	imageTag:                     "2.40.2",
	javaOpts:                     " ", // " " is used here since an empty string would be interpreted by helmfile as the environment variable not being set. And since all variables are required an empty string would result in an error
	minReadySeconds:              "0",
	livenessProbeTimeoutSeconds:  "1",
	readinessProbeTimeoutSeconds: "1",
	resourcesRequestsCPU:         "250m",
	resourcesRequestsMemory:      "1500Mi",
	startupProbeFailureThreshold: "26",
	startupProbePeriodSeconds:    "5",
	googleAuthProjectId:          " ", // TODO: " " doesn't need to be used here as with `javaOpts` since the googleAuth* parameters are stack parameters and therefor always populated
	googleAuthPrivateKey:         " ", // However the web client currently doesn't support these empty parameter so for now
	googleAuthPrivateKeyId:       " ",
	googleAuthClientEmail:        " ",
	googleAuthClientId:           " ",
}

// Stack representing ../../stacks/dhis2/helmfile.yaml
var DHIS2 = model.Stack{
	// TODO: Remove HostnamePattern once stacks 2.0 are the default
	HostnamePattern: "%s-database-postgresql.%s.svc",
	Name:            "dhis2",
	Parameters: model.StackParameters{
		"IMAGE_TAG":                       {Priority: 1, DisplayName: "Image Tag", DefaultValue: &dhis2CoreDefaults.imageTag},
		"IMAGE_REPOSITORY":                {Priority: 2, DisplayName: "Image Repository", DefaultValue: &dhis2CoreDefaults.imageRepository},
		"IMAGE_PULL_POLICY":               {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &dhis2CoreDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"DATABASE_ID":                     {Priority: 4, DisplayName: "Database"},
		"DATABASE_NAME":                   {Priority: 5, DisplayName: "Database Name", DefaultValue: &dhis2DBDefaults.dbName},
		"DATABASE_PASSWORD":               {Priority: 6, DisplayName: "Database Password", DefaultValue: &dhis2DBDefaults.dbPassword},
		"DATABASE_SIZE":                   {Priority: 7, DisplayName: "Database Size", DefaultValue: &dhis2DBDefaults.dbSize},
		"DATABASE_USERNAME":               {Priority: 8, DisplayName: "Database Username", DefaultValue: &dhis2DBDefaults.dbUsername},
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
		"GOOGLE_AUTH_PROJECT_ID":          {Priority: 0, DisplayName: "Google auth project id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId},
		"GOOGLE_AUTH_PRIVATE_KEY":         {Priority: 0, DisplayName: "Google auth private key", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKey},
		"GOOGLE_AUTH_PRIVATE_KEY_ID":      {Priority: 0, DisplayName: "Google auth private key id", DefaultValue: &dhis2CoreDefaults.googleAuthPrivateKeyId},
		"GOOGLE_AUTH_CLIENT_EMAIL":        {Priority: 0, DisplayName: "Google auth client email", DefaultValue: &dhis2CoreDefaults.googleAuthClientEmail},
		"GOOGLE_AUTH_CLIENT_ID":           {Priority: 0, DisplayName: "Google auth client id", DefaultValue: &dhis2CoreDefaults.googleAuthClientId},
	},
	ParameterProviders: model.ParameterProviders{
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
		"PGADMIN_USERNAME":  {Priority: 1, DisplayName: "pgAdmin Username"},
		"PGADMIN_PASSWORD":  {Priority: 2, DisplayName: "pgAdmin Password"},
		"CHART_VERSION":     {Priority: 3, DisplayName: "Chart Version", DefaultValue: &pgAdminDefaults.chartVersion},
		"DATABASE_HOSTNAME": {Priority: 0, DisplayName: "Database Hostname", Consumed: true},
		"DATABASE_NAME":     {Priority: 0, DisplayName: "Database Name", Consumed: true},
		"DATABASE_USERNAME": {Priority: 0, DisplayName: "Database Username", Consumed: true},
	},
	Requires: []model.Stack{
		DHIS2DB,
	},
	KubernetesResource: model.StatefulSetResource,
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
		"IMAGE_TAG":         {Priority: 1, DisplayName: "Image Tag", DefaultValue: &whoamiGoDefaults.imageTag},
		"IMAGE_REPOSITORY":  {Priority: 2, DisplayName: "Image Repository", DefaultValue: &whoamiGoDefaults.imageRepository},
		"IMAGE_PULL_POLICY": {Priority: 3, DisplayName: "Image Pull Policy", DefaultValue: &whoamiGoDefaults.imagePullPolicy, Validator: imagePullPolicy},
		"REPLICA_COUNT":     {Priority: 4, DisplayName: "Replica Count", DefaultValue: &whoamiGoDefaults.replicaCount},
		"CHART_VERSION":     {Priority: 5, DisplayName: "Chart Version", DefaultValue: &whoamiGoDefaults.chartVersion},
	},
	KubernetesResource: model.DeploymentResource,
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

// Stack representing ../../stacks/im-job-runner/helmfile.yaml
var IMJobRunner = model.Stack{
	Name: "im-job-runner",
	Parameters: model.StackParameters{
		"COMMAND":                 {Priority: 0, DisplayName: "Command"},
		"PAYLOAD":                 {Priority: 0, DisplayName: "Payload", DefaultValue: &imJobRunnerDefaults.payload},
		"DHIS2_DATABASE_DATABASE": {Priority: 0, DisplayName: "DHIS2 Database Name", DefaultValue: &dhis2DBDefaults.dbName},
		"DHIS2_DATABASE_HOSTNAME": {Priority: 0, DisplayName: "DHIS2 Database Hostname", DefaultValue: &imJobRunnerDefaults.dbHostname},
		"DHIS2_DATABASE_PASSWORD": {Priority: 0, DisplayName: "DHIS2 Database Password", DefaultValue: &dhis2DBDefaults.dbPassword},
		"DHIS2_DATABASE_PORT":     {Priority: 0, DisplayName: "DHIS2 Database Port", DefaultValue: &imJobRunnerDefaults.dbPort},
		"DHIS2_DATABASE_USERNAME": {Priority: 0, DisplayName: "DHIS2 Database Username", DefaultValue: &dhis2DBDefaults.dbUsername},
		"DHIS2_HOSTNAME":          {Priority: 0, DisplayName: "DHIS2 Hostname", DefaultValue: &imJobRunnerDefaults.dhis2Hostname},
		"CHART_VERSION":           {Priority: 0, DisplayName: "Chart Version", DefaultValue: &imJobRunnerDefaults.chartVersion},
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
var postgresHostnameProvider = model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
	return fmt.Sprintf("%s-database-postgresql.%s.svc", instance.Name, instance.GroupName), nil
})

// imagePullPolicy validates a value is a valid Kubernetes image pull policy.
var imagePullPolicy = OneOf(string(k8s.PullAlways), string(k8s.PullNever), string(k8s.PullIfNotPresent))

// OneOf creates a function returning an error when called with a value that is not any of the given
// validValues.
func OneOf(validValues ...string) func(value string) error {
	fmtErrorArg := quoteStrings(validValues)

	return func(value string) error {
		if slices.Contains(validValues, value) {
			return nil
		}

		return fmt.Errorf("%q is not valid, only %s are allowed", value, fmtErrorArg)
	}
}

// qotesStrings quotes values and comma separates them into a joint string.
func quoteStrings(values []string) string {
	var result strings.Builder
	for i, validValue := range values {
		result.WriteString(strconv.Quote(validValue))
		if i+1 < len(values) {
			result.WriteString(", ")
		}
	}
	return result.String()
}
