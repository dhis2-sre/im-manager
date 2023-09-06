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

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// assert every stack defined in Go has a helmfile
// assert every stack helmfile has a stack definition in Go
// assert that the parameters their default value and whether they are consumed are in sync
func TestStackDefinitionsAreInSyncWithHelmfile(t *testing.T) {
	helmfileStacks, err := parseStacks("../../stacks")
	require.NoError(t, err)

	helmfileParameters := make(map[string]model.StackParameters, len(helmfileStacks))
	for stackName, helmfileStack := range helmfileStacks {
		consumedParameter := make(map[string]struct{})
		for _, p := range helmfileStack.consumedParameters {
			consumedParameter[p] = struct{}{}
		}

		t.Logf("helmfile stack %q: %#v", stackName, helmfileStack)
		parameters := make(model.StackParameters)
		for name, value := range helmfileStack.parameters {
			_, consumed := consumedParameter[name]
			parameters[name] = model.StackParameter{DefaultValue: value, Consumed: consumed}
		}
		helmfileParameters[stackName] = parameters
	}

	// helmfileParameters will not contain Go Validator or Provider functions. We therefore need to
	// create map of stack name to parameters with parameters only containing. DefaultValue and
	// Consumed as we cannot ignore fields in the assertions we use.
	stacks := map[string]model.StackParameters{
		"dhis2-db":      DHIS2DB.Parameters,
		"dhis2-core":    DHIS2Core.Parameters,
		"dhis2":         DHIS2.Parameters,
		"pgadmin":       PgAdmin.Parameters,
		"whoami-go":     WhoamiGo.Parameters,
		"im-job-runner": IMJobRunner.Parameters,
	}
	stackDefinitions := make(map[string]model.StackParameters)
	for stackName, stackParameters := range stacks {
		stackDefinitionParameters := make(map[string]model.StackParameter, len(stackParameters))
		for parameterName, parameter := range stackParameters {
			stackDefinitionParameters[parameterName] = model.StackParameter{DefaultValue: parameter.DefaultValue, Consumed: parameter.Consumed}
		}
		stackDefinitions[stackName] = stackDefinitionParameters
	}
	require.NoError(t, err)

	for name, parameters := range helmfileParameters {
		stackDefinition, ok := stackDefinitions[name]
		require.Truef(t, ok, "stack %q has a helmfile but no static stack definition in Go", name)
		assert.Equalf(t, parameters, stackDefinition, "parameters defined in Go for stack %q don't match its helmfile", name)
		delete(stackDefinitions, name)
	}

	assert.Empty(t, stackDefinitions, "all stack definitions should have a helmfile, these don't")
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
	stackParameters    []string
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
	path := fmt.Sprintf("%s/%s/helmfile.yaml", dir, name)
	file, err := os.ReadFile(path) // #nosec
	if err != nil {
		return nil, fmt.Errorf("error reading stack %q: %v", name, err)
	}

	consumedParameters := extractMetadataParameters(file, consumedParametersIdentifier)
	stackParameters := extractMetadataParameters(file, stackParametersIdentifier)
	requiredParams := extractRequiredParameters(file, stackParameters)
	optionalParams := extractOptionalParameters(file, stackParameters)

	for _, parameter := range requiredParams {
		optionalParams[parameter] = nil
	}

	return &stack{
		consumedParameters: consumedParameters,
		stackParameters:    stackParameters,
		parameters:         optionalParams,
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
	regexStr := `{{[ ]requiredEnv[ ]"(.*?)".*?}}`
	return extractParameters(file, regexStr, stackParameters)
}

func extractOptionalParameters(file []byte, stackParameters []string) map[string]*string {
	regexStr := `{{[ ]env[ ]"(\w+)"[ ]?(\|[ ]?default[ ]["]?(.*)\s+}})?`
	fileData := string(file)
	parameterMap := make(map[string]*string)
	re := regexp.MustCompile(regexStr)
	matches := re.FindAllStringSubmatch(fileData, -1)
	for _, match := range matches {
		if !isSystemParameter(match[1]) && !isStackParameter(match[1], stackParameters) {
			// TODO: Update the regular expression so there's no need to trim
			value := strings.TrimSuffix(strings.TrimSpace(match[3]), "\"")
			parameterMap[match[1]] = &value
		}
	}
	return parameterMap
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
	parameters := []string{"HOSTNAME", "INSTANCE_ID", "INSTANCE_TTL", "INSTANCE_NAME", "INSTANCE_HOSTNAME", "INSTANCE_NAMESPACE", "IM_ACCESS_TOKEN", "INSTANCE_CREATION_TIMESTAMP"}
	sort.Strings(parameters)
	return parameters
}
