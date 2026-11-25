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
