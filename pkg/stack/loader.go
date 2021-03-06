package stack

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	FOLDER                       = "./stacks"
	stackParametersIdentifier    = "# stackParameters: "
	consumedParametersIdentifier = "# consumedParameters: "
	hostnamePatternIdentifier    = "# hostnamePattern: "
	hostnameVariableIdentifier   = "# hostnameVariable: "
)

// TODO: This is not thread safe
// Deleting the stack on each boot isn't ideal since instance parameters are linked to stack parameters
// Perhaps upsert using... https://gorm.io/docs/advanced_query.html#FirstOrCreate

func LoadStacks(stackService Service) error {
	stacksFolder := FOLDER

	entries, err := os.ReadDir(stacksFolder)
	if err != nil {
		return fmt.Errorf("failed to read stack folder: %s", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		log.Printf("Parsing stack: %s\n", name)
		existingStack, err := stackService.Find(name)
		if err != nil {
			if err.Error() != "record not found" {
				return fmt.Errorf("error searching existing stack %q: %v", name, err)
			}
		}
		if err == nil {
			log.Printf("Stack exists: %s\n", existingStack.Name)
			// TODO: For now just bail if the stack exists. This should probably be done differently so we can reload the stack if it has changed
			// If we have running instances we can't just change parameters etc. though
			continue
		}

		stack, err := stackService.Create(name)
		log.Printf("Stack created: %+v\n", stack)
		if err != nil {
			return fmt.Errorf("error creating stack %q: %v", name, err)
		}

		path := fmt.Sprintf("%s/%s/helmfile.yaml", stacksFolder, name)
		log.Printf("Reading: %s", path)
		file, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading stack %q: %v", name, err)
		}

		hostnamePattern := extractMetadataParameters(file, hostnamePatternIdentifier)
		if len(hostnamePattern) > 1 {
			return fmt.Errorf("error parsing stack %q: %q defined more than once", name, hostnamePatternIdentifier)
		}
		if len(hostnamePattern) == 1 {
			stack.HostnamePattern = hostnamePattern[0]
		}

		hostnameVariable := extractMetadataParameters(file, hostnameVariableIdentifier)
		if len(hostnameVariable) > 1 {
			return fmt.Errorf("error parsing stack %q: %q defined more than once", name, hostnameVariableIdentifier)
		}
		if len(hostnameVariable) == 1 {
			stack.HostnameVariable = hostnameVariable[0]
		}

		err = stackService.Save(stack)
		if err != nil {
			return fmt.Errorf("error updating stack %q: %v", name, err)
		}

		consumedParameters := extractMetadataParameters(file, consumedParametersIdentifier)

		stackParameters := extractMetadataParameters(file, stackParametersIdentifier)

		requiredParameterSet := extractRequiredParameters(file, stackParameters)
		fmt.Printf("Required parameters: %+v\n", requiredParameterSet)
		for _, name := range requiredParameterSet {
			isConsumed := isConsumedParameter(name, consumedParameters)
			_, err := stackService.CreateRequiredParameter(stack, name, isConsumed)
			if err != nil {
				return fmt.Errorf("error creating required parameter %q for stack %q: %v", name, stack.Name, err)
			}
		}

		optionalParameterMap := extractOptionalParameters(file, stackParameters)
		fmt.Printf("Optional parameters: %+v\n", optionalParameterMap)
		for name, v := range optionalParameterMap {
			isConsumed := isConsumedParameter(name, consumedParameters)
			_, err := stackService.CreateOptionalParameter(stack, name, v, isConsumed)
			if err != nil {
				return fmt.Errorf("error creating optional parameter %q for stack %q: %v", name, stack.Name, err)
			}
		}
	}

	return nil
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

func extractOptionalParameters(file []byte, stackParameters []string) map[string]string {
	regexStr := `{{[ ]env[ ]"(\w+)"[ ]?(\|[ ]?default[ ]["]?(.*)\s+}})?`
	fileData := string(file)
	parameterMap := make(map[string]string)
	re := regexp.MustCompile(regexStr)
	matches := re.FindAllStringSubmatch(fileData, -1)
	for _, match := range matches {
		if !isSystemParameter(match[1]) && !isStackParameter(match[1], stackParameters) {
			// TODO: Update the regular expression so there's no need to trim
			parameterMap[match[1]] = strings.TrimSuffix(strings.TrimSpace(match[3]), "\"")
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
	parameters := []string{"INSTANCE_ID", "INSTANCE_NAME", "INSTANCE_HOSTNAME", "INSTANCE_NAMESPACE", "IM_ACCESS_TOKEN"}
	sort.Strings(parameters)
	return parameters
}
