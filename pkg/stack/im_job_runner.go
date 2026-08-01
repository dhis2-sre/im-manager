package stack

// Stack representing ../../stacks/im-job-runner/helmfile.yaml.gotmpl
var IMJobRunner = Stack{
	Name: "im-job-runner",
	Parameters: StackParameters{
		"COMMAND":                 {Priority: 0, DisplayName: "Command"},
		"PAYLOAD":                 {Priority: 0, DisplayName: "Payload", DefaultValue: &imJobRunnerDefaults.payload},
		"DHIS2_DATABASE_DATABASE": {Priority: 0, DisplayName: "DHIS2 Database Name", DefaultValue: &dhis2DBDefaults.dbName},
		"DHIS2_DATABASE_HOSTNAME": {Priority: 0, DisplayName: "DHIS2 Database Hostname", DefaultValue: &imJobRunnerDefaults.dbHostname},
		"DHIS2_DATABASE_PASSWORD": {Priority: 0, DisplayName: "DHIS2 Database Password", DefaultValue: &dhis2DBDefaults.dbPassword, Sensitive: true},
		"DHIS2_DATABASE_PORT":     {Priority: 0, DisplayName: "DHIS2 Database Port", DefaultValue: &imJobRunnerDefaults.dbPort},
		"DHIS2_DATABASE_USERNAME": {Priority: 0, DisplayName: "DHIS2 Database Username", DefaultValue: &dhis2DBDefaults.dbUsername, Sensitive: true},
		"DHIS2_HOSTNAME":          {Priority: 0, DisplayName: "DHIS2 Hostname", DefaultValue: &imJobRunnerDefaults.dhis2Hostname},
		"CHART_VERSION":           {Priority: 0, DisplayName: "Chart Version", DefaultValue: &imJobRunnerDefaults.chartVersion},
	},
	// No components: the job-runner is an old jobs experiment that only labels pods, so no
	// workload is addressable. Jobs get their own design in a separate task.
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

// imagePullPolicy validates a value is a valid Kubernetes image pull policy.
