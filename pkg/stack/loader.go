package stack

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

const FOLDER = "./stacks"
const STACK_PARAMETERS_IDENTIFIER = "# stackParameters: "

// TODO: This is not thread safe
// Deleting the stack on each boot isn't ideal since instance parameters are linked to stack parameters
// Perhaps upsert using... https://gorm.io/docs/advanced_query.html#FirstOrCreate

func LoadStacks(stackService Service) {
	stacksFolder := FOLDER

	entries, err := os.ReadDir(stacksFolder)
	if err != nil {
		log.Fatalf("Failed to read stack folder: %s", err)
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
				log.Fatalf("Error search for existing stack: %s\n%+v", name, err)
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
			log.Fatalf("Error creating stack: %s\n%+v", name, err)
		}

		path := fmt.Sprintf("%s/%s/helmfile.yaml", stacksFolder, name)
		log.Printf("Reading: %s", path)
		file, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("Failed to read stack: %s", err)
		}

		stackParameters := extractStackParameters(file)

		requiredParameterSet := extractRequiredParameters(file, stackParameters)
		fmt.Printf("Required parameters: %+v\n", requiredParameterSet)
		for _, parameter := range requiredParameterSet {
			_, err := stackService.CreateRequiredParameter(stack, parameter)
			if err != nil {
				log.Fatalf("Failed to create parameter: %s", err)
			}
		}

		optionalParameterMap := extractOptionalParameters(file, stackParameters)
		fmt.Printf("Optional parameters: %+v\n", optionalParameterMap)
		for k, v := range optionalParameterMap {
			_, err := stackService.CreateOptionalParameter(stack, k, v)
			if err != nil {
				log.Fatalf("Failed to create parameter: %s", err)
			}
		}

	}
}

func extractStackParameters(file []byte) []string {
	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, STACK_PARAMETERS_IDENTIFIER) {
			trimmed := strings.TrimPrefix(line, STACK_PARAMETERS_IDENTIFIER)
			trimmed = strings.ReplaceAll(trimmed, " ", "")
			parameters := strings.Split(trimmed, ",")
			sort.Strings(parameters)
			return parameters
		}
	}
	return []string{}
}

func extractRequiredParameters(file []byte, stackParameters []string) []string {
	regexStr := "\\{\\{[ ]?requiredEnv[ ]?\"(.*?)\".*\\}\\}"
	return extractParameters(file, regexStr, stackParameters)
}

func extractOptionalParameters(file []byte, stackParameters []string) map[string]string {
	regexStr := "\\{\\{[ ]?env[ ]?\"(.*?)\"[ ]?|[ ]?default[ ]? \"(.*?)\"\\}\\}"
	fileData := string(file)
	parameterMap := make(map[string]string)
	re := regexp.MustCompile(regexStr)
	matches := re.FindAllStringSubmatch(fileData, 100) // TODO: Better way than just passing 100?
	log.Println("Matches: ")
	log.Printf("%+v", matches)
	for _, match := range matches {
		if !isSystemParameter(match[1]) && !isStackParameter(match[1], stackParameters) {
			parameterMap[match[1]] = match[2]
		}
	}
	return parameterMap
}

func extractParameters(file []byte, regexStr string, stackParameters []string) []string {
	fileData := string(file)
	parameterSet := make(map[string]bool)
	re := regexp.MustCompile(regexStr)
	matches := re.FindAllStringSubmatch(fileData, 100) // TODO: Better way than just passing 100?
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
	parameters := []string{"INSTANCE_ID", "INSTANCE_NAME", "INSTANCE_HOSTNAME", "INSTANCE_NAMESPACE", "IM_ACCESS_TOKEN"}
	sort.Strings(parameters)
	return parameters
}
