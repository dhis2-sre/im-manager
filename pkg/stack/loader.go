package stack

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	stackParametersIdentifier    = "# stackParameters: "
	consumedParametersIdentifier = "# consumedParameters: "
)

// TODO: This is not thread safe
// Deleting the stack on each boot isn't ideal since instance parameters are linked to stack parameters
// Perhaps upsert using... https://gorm.io/docs/advanced_query.html#FirstOrCreate

type stack struct {
	consumedParameters []string
	stackParameters    []string
	parameters         map[string]*string
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
