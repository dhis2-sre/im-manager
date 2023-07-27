package stack

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

const (
	stackParametersIdentifier    = "# stackParameters: "
	consumedParametersIdentifier = "# consumedParameters: "
	hostnamePatternIdentifier    = "# hostnamePattern: "
	hostnameVariableIdentifier   = "# hostnameVariable: "
)

// TODO: This is not thread safe
// Deleting the stack on each boot isn't ideal since instance parameters are linked to stack parameters
// Perhaps upsert using... https://gorm.io/docs/advanced_query.html#FirstOrCreate

func LoadStacks(dir string, stackService Service) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read stack folder: %s", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		log.Printf("Parsing stack: %q\n", name)
		stackTemplate, err := parseStack(dir, name)
		if err != nil {
			return fmt.Errorf("error parsing stack %q: %v", name, err)
		}

		existingStack, err := stackService.Find(name)
		if err != nil {
			if !errdef.IsNotFound(err) {
				return fmt.Errorf("error searching existing stack %q: %w", name, err)
			}
		}
		if err == nil {
			log.Printf("Stack exists: %s\n", existingStack.Name)
			// TODO: For now just bail if the stack exists. This should probably be done differently so we can reload the stack if it has changed
			// If we have running instances we can't just change parameters etc. though
			continue
		}

		stack := &model.Stack{
			Name:             name,
			HostnamePattern:  stackTemplate.hostnamePattern,
			HostnameVariable: stackTemplate.hostnameVariable,
		}
		for name, v := range stackTemplate.parameters {
			isConsumed := isConsumedParameter(name, stackTemplate.consumedParameters)
			parameter := &model.StackParameter{Name: name, StackName: stack.Name, Consumed: isConsumed, DefaultValue: v}
			stack.Parameters = append(stack.Parameters, *parameter)
		}

		err = stackService.Create(stack)
		log.Printf("Stack created: %+v\n", stack)
		if err != nil {
			return fmt.Errorf("error creating stack %q: %v", name, err)
		}
	}

	return nil
}

type stack struct {
	hostnamePattern    string
	hostnameVariable   string
	consumedParameters []string
	stackParameters    []string
	requiredParameters []string
	optionalParameters map[string]*string
	parameters         map[string]*string
}

func parseStack(dir, name string) (*stack, error) {
	path := fmt.Sprintf("%s/%s/helmfile.yaml", dir, name)
	file, err := os.ReadFile(path) // #nosec
	if err != nil {
		return nil, fmt.Errorf("error reading stack %q: %v", name, err)
	}

	hostnamePatterns := extractMetadataParameters(file, hostnamePatternIdentifier)
	if len(hostnamePatterns) > 1 {
		return nil, fmt.Errorf("error parsing stack %q: %q defined more than once", name, hostnamePatternIdentifier)
	}
	var hostnamePattern string
	if len(hostnamePatterns) == 1 {
		hostnamePattern = hostnamePatterns[0]
	}

	hostnameVariables := extractMetadataParameters(file, hostnameVariableIdentifier)
	if len(hostnameVariables) > 1 {
		return nil, fmt.Errorf("error parsing stack %q: %q defined more than once", name, hostnameVariableIdentifier)
	}
	var hostnameVariable string
	if len(hostnameVariables) == 1 {
		hostnameVariable = hostnameVariables[0]
	}

	consumedParameters := extractMetadataParameters(file, consumedParametersIdentifier)
	stackParameters := extractMetadataParameters(file, stackParametersIdentifier)
	requiredParams := extractRequiredParameters(file, stackParameters)
	optionalParams := extractOptionalParameters(file, stackParameters)

	for _, parameter := range requiredParams {
		optionalParams[parameter] = nil
	}

	return &stack{
		hostnamePattern:    hostnamePattern,
		hostnameVariable:   hostnameVariable,
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

func isConsumedParameter(parameter string, consumedParameters []string) bool {
	return inSlice(parameter, consumedParameters)
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
