package stack

import "github.com/dhis2-sre/im-manager/pkg/kube"

// Stack representing ../../stacks/pgadmin/helmfile.yaml.gotmpl
// PgAdmin has no Requires. It consumes the database connection parameters, so any stack providing
// them can offer it as a companion, which is what the hardcoded requirement on dhis2-db prevented.
// Declaring it as a companion the other way round is also what lets these stacks initialise: a
// Requires back onto a stack that lists PgAdmin as a companion is an initialisation cycle.
var PgAdmin = withGroupedParameters(Stack{
	Name: "pgadmin",
	ParameterGroups: []ParameterGroup{
		{Name: "pgadmin", Title: "pgAdmin", Parameters: StackParameters{
			"PGADMIN_USERNAME":  {Priority: 1, DisplayName: "pgAdmin Username", Sensitive: true},
			"PGADMIN_PASSWORD":  {Priority: 2, DisplayName: "pgAdmin Password", Sensitive: true},
			"CHART_VERSION":     {Priority: 3, DisplayName: "Chart Version", DefaultValue: &pgAdminDefaults.chartVersion},
			"DATABASE_HOSTNAME": {Priority: 0, DisplayName: "Database Hostname", Consumed: true},
			"DATABASE_NAME":     {Priority: 0, DisplayName: "Database Name", Consumed: true},
			"DATABASE_USERNAME": {Priority: 0, DisplayName: "Database Username", Consumed: true, Sensitive: true},
		}},
	},
	Components: []kube.Component{
		PgAdminComponent{BaseComponent: kube.BaseComponent{Name: "pgadmin"}},
	},
})

var pgAdminDefaults = struct {
	chartVersion string
	enabled      string
}{
	chartVersion: "1.33.3",
	enabled:      "false",
}

// whenPgAdminIsEnabled gates the companion on the host's own parameter, the same way DEPLOY_CHAP
// gates chap. The host is where the flag has to live, since pgAdmin only exists once the user has
// asked for it, and storing the choice as a parameter is what makes it outlast the deploy form.
var whenPgAdminIsEnabled = &kube.Condition{Parameter: "ENABLE_PGADMIN", Equals: "true"}

// Stack representing ../../stacks/whoami-go/helmfile.yaml.gotmpl
