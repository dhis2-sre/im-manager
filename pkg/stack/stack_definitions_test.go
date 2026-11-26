package stack

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assert every stack defined in Go has a helmfile
// assert every stack helmfile has a stack definition in Go
// assert that the parameters their default value and whether they are consumed are in sync
func TestStackDefinitionsAreInSyncWithHelmfile(t *testing.T) {
	helmfileStacks, err := parseStacks("../../stacks")
	require.NoError(t, err)

	helmfileParameters := make(map[string]StackParameters, len(helmfileStacks))
	for stackName, helmfileStack := range helmfileStacks {
		consumedParameter := make(map[string]struct{})
		for _, p := range helmfileStack.consumedParameters {
			consumedParameter[p] = struct{}{}
		}

		parameters := make(StackParameters)
		for name, value := range helmfileStack.parameters {
			_, consumed := consumedParameter[name]
			parameters[name] = StackParameter{DefaultValue: value, Consumed: consumed}
		}
		helmfileParameters[stackName] = parameters
	}

	// helmfileParameters will not contain Go Validator or Provider functions. We therefore need to
	// create a map of stack name to parameters with parameters only containing. DefaultValue and
	// Consumed as we cannot ignore fields in the assertions we use.
	stacks := make(map[string]StackParameters, len(All))
	for _, s := range All {
		stacks[s.Name] = s.Parameters
	}
	stackDefinitions := make(map[string]StackParameters)
	for stackName, stackParameters := range stacks {
		stackDefinitionParameters := make(map[string]StackParameter, len(stackParameters))
		for parameterName, parameter := range stackParameters {
			// DefaultValue is nil since it's no longer in the helmfile and therefor we don't have anything to compare it to
			stackDefinitionParameters[parameterName] = StackParameter{DefaultValue: nil, Consumed: parameter.Consumed}
		}
		stackDefinitions[stackName] = stackDefinitionParameters
	}

	for name, parameters := range helmfileParameters {
		stackDefinition, ok := stackDefinitions[name]
		require.Truef(t, ok, "stack %q has a helmfile but no static stack definition in Go", name)
		assert.Equalf(t, parameters, stackDefinition, "parameters defined in Go for stack %q don't match its helmfile", name)
		delete(stackDefinitions, name)
	}

	assert.Empty(t, stackDefinitions, "all stack definitions should have a helmfile, these don't")
}

// A destroy uninstalls the release by the name ReleaseName gives it, so every helmfile has to install
// exactly that one release.
func TestStackReleaseNamesAreInSyncWithHelmfile(t *testing.T) {
	releaseName := regexp.MustCompile(`(?m)^  - name: "?\{\{ requiredEnv "INSTANCE_NAME" \}\}([^"\s]*)"?\s*$`)

	for _, s := range All {
		file, err := os.ReadFile(fmt.Sprintf("../../stacks/%s/helmfile.yaml.gotmpl", s.Name)) // #nosec
		require.NoError(t, err)

		matches := releaseName.FindAllStringSubmatch(string(file), -1)
		require.Lenf(t, matches, 1, "stack %q should install exactly one release named after the instance", s.Name)
		assert.Equalf(t, s.ReleaseName("instance"), "instance"+matches[0][1], "stack %q", s.Name)
	}
}

// A parameter a helmfile reads with requiredEnv cannot render blank, so IM has to refuse a blank one
// before helm does, and every other parameter may be left blank.
func TestRequiredParametersAreInSyncWithHelmfile(t *testing.T) {
	for _, s := range All {
		file, err := os.ReadFile(fmt.Sprintf("../../stacks/%s/helmfile.yaml.gotmpl", s.Name)) // #nosec
		require.NoError(t, err)
		readWithRequiredEnv := extractRequiredParameters(file, nil)

		for name, parameter := range s.Parameters {
			if parameter.Consumed {
				continue
			}
			assert.Equalf(t, inSlice(name, readWithRequiredEnv), parameter.Required, "stack %q parameter %q: Required has to match whether its helmfile reads it with requiredEnv", s.Name, name)
		}
	}
}

func TestIsSystemParameterPositive(t *testing.T) {
	const instanceId = "INSTANCE_ID"

	parameter := isSystemParameter(instanceId)

	assert.True(t, parameter)
}

func TestIsSystemParameterNegative(t *testing.T) {
	const instanceId = "some-random-parameter-name"

	parameter := isSystemParameter(instanceId)

	assert.False(t, parameter)
}

const (
	stackParametersIdentifier    = "# stackParameters: "
	consumedParametersIdentifier = "# consumedParameters: "
)

type stack struct {
	consumedParameters []string
	parameters         map[string]*string
}

func parseStacks(dir string) (map[string]*stack, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	stacks := make(map[string]*stack)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stackName := entry.Name()
		stack, err := parseStack(dir, stackName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stack %q: %v", stackName, err)
		}

		stacks[stackName] = stack
	}

	return stacks, nil
}

func parseStack(dir, name string) (*stack, error) {
	path := fmt.Sprintf("%s/%s/helmfile.yaml.gotmpl", dir, name)
	file, err := os.ReadFile(path) // #nosec
	if err != nil {
		return nil, fmt.Errorf("error reading stack %q: %v", name, err)
	}

	consumedParameters := extractMetadataParameters(file, consumedParametersIdentifier)
	stackParameters := extractMetadataParameters(file, stackParametersIdentifier)
	requiredParams := extractRequiredParameters(file, stackParameters)

	parameters := make(map[string]*string, len(stackParameters)+len(requiredParams))
	for _, parameter := range stackParameters {
		parameters[parameter] = nil
	}
	for _, parameter := range requiredParams {
		parameters[parameter] = nil
	}

	return &stack{
		consumedParameters: consumedParameters,
		parameters:         parameters,
	}, nil
}

func extractMetadataParameters(file []byte, identifier string) []string {
	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, identifier) {
			trimmed := strings.TrimPrefix(line, identifier)
			trimmed = strings.ReplaceAll(trimmed, " ", "")
			parameters := strings.Split(trimmed, ",")
			sort.Strings(parameters)
			return parameters
		}
	}
	return []string{}
}

func extractRequiredParameters(file []byte, stackParameters []string) []string {
	regexStr := `requiredEnv "([\w_]+)"`
	return extractParameters(file, regexStr, stackParameters)
}

func extractParameters(file []byte, regexStr string, stackParameters []string) []string {
	fileData := string(file)
	parameterSet := make(map[string]bool)
	re := regexp.MustCompile(regexStr)
	matches := re.FindAllStringSubmatch(fileData, -1)
	for _, match := range matches {
		if !isSystemParameter(match[1]) && !isStackParameter(match[1], stackParameters) {
			parameterSet[match[1]] = true
		}
	}
	return getKeys(parameterSet)
}

func getKeys(parameterSet map[string]bool) []string {
	keys := make([]string, len(parameterSet))
	i := 0
	for k := range parameterSet {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func isStackParameter(parameter string, stackParameters []string) bool {
	return inSlice(parameter, stackParameters)
}

func isSystemParameter(parameter string) bool {
	systemParameters := getSystemParameters()
	return inSlice(parameter, systemParameters)
}

func inSlice(str string, strings []string) bool {
	index := sort.SearchStrings(strings, str)
	return index < len(strings) && strings[index] == str
}

func getSystemParameters() []string {
	parameters := []string{"HOSTNAME", "DEPLOYMENT_ID", "INSTANCE_ID", "INSTANCE_TTL", "INSTANCE_NAME", "INSTANCE_PATH_NAME", "INSTANCE_HOSTNAME", "INSTANCE_NAMESPACE", "IM_ACCESS_TOKEN", "INSTANCE_CREATION_TIMESTAMP"}
	sort.Strings(parameters)
	return parameters
}

// assert group declarations are consistent: unique group names with titles, every parameter's
// group referencing a declared group, and every group condition referencing a stack parameter.
func TestParameterGroupsAreConsistent(t *testing.T) {
	for _, s := range All {
		groups := make(map[string]bool, len(s.ParameterGroups))
		for _, group := range s.ParameterGroups {
			assert.NotEmptyf(t, group.Name, "stack %q declares a group without a name", s.Name)
			assert.NotEmptyf(t, group.Title, "stack %q group %q has no title", s.Name, group.Name)
			assert.Falsef(t, groups[group.Name], "stack %q declares group %q twice", s.Name, group.Name)
			groups[group.Name] = true

			if group.When != nil {
				_, ok := s.Parameters[group.When.Parameter]
				assert.Truef(t, ok, "stack %q group %q condition references unknown parameter %q", s.Name, group.Name, group.When.Parameter)
			}
		}

		for name, parameter := range s.Parameters {
			if parameter.Group != "" {
				assert.Truef(t, groups[parameter.Group], "stack %q parameter %q references undeclared group %q", s.Name, name, parameter.Group)
			}
		}

		if len(s.ParameterGroups) > 0 {
			for name, parameter := range s.Parameters {
				assert.NotEmptyf(t, parameter.Group, "stack %q parameter %q has no group but the stack declares groups", s.Name, name)
			}
		}
	}
}

// assert every parameter offered as an add-on is a 'true'/'false' switch that says what it adds,
// since a client shows it as something to add with nothing but its name and description to go by.
func TestAddOnParametersAreDescribedSwitches(t *testing.T) {
	for _, s := range All {
		for name, parameter := range s.Parameters {
			if !parameter.AddOn {
				continue
			}
			assert.NotEmptyf(t, parameter.Description, "stack %q add-on %q has no description", s.Name, name)
			if assert.NotNilf(t, parameter.DefaultValue, "stack %q add-on %q has no default", s.Name, name) {
				assert.Containsf(t, []string{"true", "false"}, *parameter.DefaultValue, "stack %q add-on %q is not a 'true'/'false' switch", s.Name, name)
			}
		}
	}
}

// assert the stacks API serves what a client needs to offer an add-on, and serves neither for a
// parameter that is an ordinary setting.
func TestResponseServesAddOnAndDescription(t *testing.T) {
	parameters := make(map[string]StackParameterResponse)
	for _, parameter := range toResponseStack(DHIS2V2).Parameters {
		parameters[parameter.ParameterName] = parameter
	}

	for _, name := range []string{"DEPLOY_CHAP", "ENABLE_DORIS", "ENABLE_PGADMIN", "DEPLOY_GLOWROOT"} {
		assert.Truef(t, parameters[name].AddOn, "%q is not served as an add-on", name)
		assert.Equalf(t, DHIS2V2.Parameters[name].Description, parameters[name].Description, "%q", name)
		assert.NotEmptyf(t, parameters[name].Description, "%q is served without a description", name)
	}

	assert.False(t, parameters["ALLOW_SUSPEND"].AddOn)
	assert.Empty(t, parameters["ALLOW_SUSPEND"].Description)
}

// ingressPath matches the path a helmfile gives an ingress, capturing what follows the instance's
// own path in it. Probe and context paths are spelled the same way, so they only ever repeat a
// suffix an ingress already has.
var ingressPath = regexp.MustCompile(`(?m)\bpath: /\{\{ requiredEnv "INSTANCE_PATH_NAME" \}\}(\S*)\s*$`)

// assert the endpoints a stack declares are exactly the paths its helmfile serves, so the address a
// client builds is one the chart answers on and no served address goes undeclared.
func TestEndpointsAreInSyncWithHelmfile(t *testing.T) {
	for _, s := range All {
		helmfile, err := os.ReadFile(fmt.Sprintf("../../stacks/%s/helmfile.yaml.gotmpl", s.Name))
		require.NoError(t, err)

		served := map[string]bool{}
		for _, match := range ingressPath.FindAllStringSubmatch(string(helmfile), -1) {
			served[match[1]] = true
		}

		declared := map[string]bool{}
		for _, endpoint := range s.Endpoints {
			assert.Falsef(t, declared[endpoint.PathSuffix], "stack %q declares the endpoint %q twice", s.Name, endpoint.PathSuffix)
			declared[endpoint.PathSuffix] = true

			if endpoint.When != nil {
				_, ok := s.Parameters[endpoint.When.Parameter]
				assert.Truef(t, ok, "stack %q endpoint %q condition references unknown parameter %q", s.Name, endpoint.PathSuffix, endpoint.When.Parameter)
			}
		}

		assert.Equalf(t, served, declared, "stack %q endpoints differ from the paths its helmfile serves", s.Name)
	}
}

// assert the stacks API serves a stack's endpoints with their conditions, and none for a stack that
// is reached only through another.
func TestResponseServesEndpoints(t *testing.T) {
	assert.Equal(t, []Endpoint{{PathSuffix: ""}, {PathSuffix: "-glowroot", When: whenGlowrootIsDeployed}}, toResponseStack(DHIS2V2).Endpoints)
	assert.Equal(t, []Endpoint{{PathSuffix: "-pgadmin"}}, toResponseStack(PgAdmin).Endpoints)
	assert.Empty(t, toResponseStack(Chap).Endpoints)
}

// assert the layout a stack declares for a form is one a client can follow: sections with unique
// names and titles, every parameter's section declared by its own group and named only by an
// advanced parameter, no section left empty, and every parameter that cannot be tucked away under
// the advanced settings declared primary.
func TestParameterLayoutIsConsistent(t *testing.T) {
	for _, s := range All {
		used := make(map[string]map[string]bool, len(s.ParameterGroups))
		for _, group := range s.ParameterGroups {
			used[group.Name] = make(map[string]bool, len(group.Sections))
			titles := make(map[string]bool, len(group.Sections))
			for _, section := range group.Sections {
				assert.NotEmptyf(t, section.Name, "stack %q group %q declares a section without a name", s.Name, group.Name)
				assert.NotEmptyf(t, section.Title, "stack %q group %q section %q has no title", s.Name, group.Name, section.Name)
				_, declared := used[group.Name][section.Name]
				assert.Falsef(t, declared, "stack %q group %q declares section %q twice", s.Name, group.Name, section.Name)
				assert.Falsef(t, titles[section.Title], "stack %q group %q declares two sections titled %q", s.Name, group.Name, section.Title)
				used[group.Name][section.Name] = false
				titles[section.Title] = true
			}
		}

		for name, parameter := range s.Parameters {
			if parameter.Section != "" {
				assert.Falsef(t, parameter.Primary, "stack %q parameter %q is primary and in section %q, which only lists advanced parameters", s.Name, name, parameter.Section)
				_, declared := used[parameter.Group][parameter.Section]
				assert.Truef(t, declared, "stack %q parameter %q names section %q, which its group %q does not declare", s.Name, name, parameter.Section, parameter.Group)
				used[parameter.Group][parameter.Section] = true
			}
			if parameter.Required && parameter.DefaultValue == nil && !parameter.Consumed {
				assert.Truef(t, parameter.Primary, "stack %q parameter %q is required with no default, so it has to be filled in and has to be primary", s.Name, name)
			}
		}

		// A conditional group is shown beside the choice it depends on when that choice lives in
		// another group, so the choice and the group's own parameters are up front. An add-on's
		// switch is offered as something to add instead, and the groups it gates make its own card.
		for _, group := range s.ParameterGroups {
			if group.When == nil {
				continue
			}
			choice := s.Parameters[group.When.Parameter]
			if choice.Group == group.Name || choice.AddOn {
				continue
			}
			assert.Truef(t, choice.Primary, "stack %q parameter %q, which group %q depends on, is not primary", s.Name, group.When.Parameter, group.Name)
			for name, parameter := range group.Parameters {
				assert.Truef(t, parameter.Primary, "stack %q parameter %q is not primary, though its group %q is shown beside the choice it depends on", s.Name, name, group.Name)
			}
		}

		for group, sections := range used {
			for section, hasParameters := range sections {
				assert.Truef(t, hasParameters, "stack %q group %q section %q has no parameters", s.Name, group, section)
			}
		}
	}
}

// sectionLayout is a titled list of a group's advanced parameters. An empty title is the list
// after the group's sections.
type sectionLayout struct {
	Title      string
	Parameters []string
}

// groupLayout is where a client following the served layout puts a group's parameters.
type groupLayout struct {
	Primary  []string
	Advanced []sectionLayout
}

// layoutOf lays out a stack's served parameters the way a client does: primary parameters up front,
// advanced ones under the group's sections in order and then the rest, consumed ones nowhere since
// the server fills them. Names are sorted, since within a list a client orders by priority.
func layoutOf(response StackResponse) map[string]groupLayout {
	byGroup := make(map[string][]StackParameterResponse)
	for _, parameter := range response.Parameters {
		if !parameter.Consumed {
			byGroup[parameter.Group] = append(byGroup[parameter.Group], parameter)
		}
	}

	names := func(parameters []StackParameterResponse, keep func(StackParameterResponse) bool) []string {
		var result []string
		for _, parameter := range parameters {
			if keep(parameter) {
				result = append(result, parameter.ParameterName)
			}
		}
		sort.Strings(result)
		return result
	}

	layout := make(map[string]groupLayout, len(response.ParameterGroups))
	for _, group := range response.ParameterGroups {
		parameters := byGroup[group.Name]
		var advanced []sectionLayout
		for _, section := range group.Sections {
			advanced = append(advanced, sectionLayout{Title: section.Title, Parameters: names(parameters, func(p StackParameterResponse) bool {
				return !p.Primary && p.Section == section.Name
			})})
		}
		if rest := names(parameters, func(p StackParameterResponse) bool { return !p.Primary && p.Section == "" }); len(rest) > 0 {
			advanced = append(advanced, sectionLayout{Parameters: rest})
		}
		layout[group.Name] = groupLayout{
			Primary:  names(parameters, func(p StackParameterResponse) bool { return p.Primary }),
			Advanced: advanced,
		}
	}
	return layout
}

// assert the stacks API serves the form layout the web client used to keep as its own tables of
// parameter names, so moving those tables here changes nothing a user sees. Parameters of a group
// shown beside the choice it depends on, such as the file store's, were always all shown.
func TestResponseServesFormLayout(t *testing.T) {
	addOns := sectionLayout{Parameters: []string{"DEPLOY_CHAP", "DEPLOY_GLOWROOT", "ENABLE_DORIS", "ENABLE_PGADMIN"}}

	assert.Equal(t, map[string]groupLayout{
		"dhis2": {
			Primary: []string{"IMAGE_REPOSITORY", "IMAGE_TAG", "STORAGE_TYPE"},
			Advanced: []sectionLayout{
				{"Image and resources", []string{"CHART_VERSION", "CORE_RESOURCES_REQUESTS_CPU", "CORE_RESOURCES_REQUESTS_MEMORY", "DHIS2_HOME", "IMAGE_PULL_POLICY", "JAVA_OPTS"}},
				{"Health checks", []string{"LIVENESS_PROBE_TIMEOUT_SECONDS", "MIN_READY_SECONDS", "READINESS_PROBE_TIMEOUT_SECONDS", "STARTUP_PROBE_FAILURE_THRESHOLD", "STARTUP_PROBE_PERIOD_SECONDS"}},
				{"Configuration", []string{"ALLOW_SUSPEND", "CUSTOM_DHIS2_CONFIG", "SAME_SITE_COOKIES"}},
				{"Google sign-in", []string{"GOOGLE_AUTH_CLIENT_EMAIL", "GOOGLE_AUTH_CLIENT_ID", "GOOGLE_AUTH_PRIVATE_KEY", "GOOGLE_AUTH_PRIVATE_KEY_ID", "GOOGLE_AUTH_PROJECT_ID"}},
				{"Database migrations", []string{"ENABLE_QUERY_LOGGING", "FLYWAY_MIGRATE_OUT_OF_ORDER", "FLYWAY_REPAIR_BEFORE_MIGRATION"}},
				// The add-on switches, which a client offers as tiles rather than as settings.
				addOns,
			},
		},
		"db": {
			Primary: []string{"DATABASE_ID", "DATABASE_SIZE"},
			Advanced: []sectionLayout{
				{"Connection", []string{"DATABASE_NAME", "DATABASE_PASSWORD", "DATABASE_USERNAME"}},
				{"Resources", []string{"DB_RESOURCES_REQUESTS_CPU", "DB_RESOURCES_REQUESTS_MEMORY"}},
			},
		},
		"minio":      {Primary: []string{"MINIO_STORAGE_SIZE"}},
		"s3":         {Primary: []string{"S3_BUCKET", "S3_IDENTITY", "S3_REGION", "S3_SECRET"}},
		"filesystem": {Primary: []string{"FILESYSTEM_VOLUME_SIZE"}},
		"doris-fe": {
			Primary: []string{"DORIS_FRONTEND_VERSION"},
			Advanced: []sectionLayout{{Parameters: []string{
				"DORIS_DATABASE", "DORIS_FRONTEND_REPLICAS",
				"DORIS_FRONTEND_RESOURCES_LIMITS_CPU", "DORIS_FRONTEND_RESOURCES_LIMITS_MEMORY",
				"DORIS_FRONTEND_RESOURCES_REQUESTS_CPU", "DORIS_FRONTEND_RESOURCES_REQUESTS_MEMORY",
				"DORIS_PASSWORD", "DORIS_USERNAME",
			}}},
		},
		"doris-be": {
			Primary: []string{"DORIS_BACKEND_VERSION"},
			Advanced: []sectionLayout{{Parameters: []string{
				"DORIS_BACKEND_REPLICAS",
				"DORIS_BACKEND_RESOURCES_LIMITS_CPU", "DORIS_BACKEND_RESOURCES_LIMITS_MEMORY",
				"DORIS_BACKEND_RESOURCES_REQUESTS_CPU", "DORIS_BACKEND_RESOURCES_REQUESTS_MEMORY",
			}}},
		},
	}, layoutOf(toResponseStack(DHIS2V2)))

	assert.Equal(t, map[string]groupLayout{
		"api":      {Primary: []string{"DHIS2_REGISTER", "IMAGE_TAG"}, Advanced: []sectionLayout{{Parameters: []string{"CHART_VERSION", "IMAGE_PULL_POLICY"}}}},
		"worker":   {Primary: []string{"WORKER_IMAGE_TAG"}, Advanced: []sectionLayout{{Parameters: []string{"WORKER_IMAGE_PULL_POLICY"}}}},
		"register": {Primary: []string{"DHIS2_PASSWORD", "DHIS2_USERNAME"}},
		"db":       {Primary: []string{"DATABASE_SIZE"}, Advanced: []sectionLayout{{Parameters: []string{"DATABASE_NAME", "DATABASE_PASSWORD", "DATABASE_VERSION"}}}},
		"valkey":   {Advanced: []sectionLayout{{Parameters: []string{"REDIS_PASSWORD", "REDIS_STORAGE_SIZE"}}}},
	}, layoutOf(toResponseStack(Chap)))

	assert.Equal(t, map[string]groupLayout{
		"pgadmin": {Primary: []string{"PGADMIN_PASSWORD", "PGADMIN_USERNAME"}, Advanced: []sectionLayout{{Parameters: []string{"CHART_VERSION"}}}},
	}, layoutOf(toResponseStack(PgAdmin)))
}
