package stack

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"gopkg.in/yaml.v3"
)

const (
	stackParametersIdentifier    = "# stackParameters: "
	consumedParametersIdentifier = "# consumedParameters: "
	hostnamePatternIdentifier    = "# hostnamePattern: "
	hostnameVariableIdentifier   = "# hostnameVariable: "
)

type metadata struct {
	hostnameVariable   string
	hostnamePattern    string
	consumedParameters map[string]struct{}
	stackParameters    map[string]struct{}
}

type metadataYaml struct {
	HostnameVariable   string   `yaml:"hostnameVariable,omitempty"`
	HostnamePattern    string   `yaml:"hostnamePattern,omitempty"`
	ConsumedParameters []string `yaml:"consumedParameters,omitempty"`
	StackParameters    []string `yaml:"stackParameters,omitempty"`
}

// TODO: This is not thread safe
// Deleting the stack on each boot isn't ideal since instance parameters are linked to stack parameters
// Perhaps upsert using... https://gorm.io/docs/advanced_query.html#FirstOrCreate

func LoadStacks(dir string, stackService Service) error {
	stacks, err := parseStacks(dir)
	if err != nil {
		return err
	}

	for _, stack := range stacks {
		existingStack, err := stackService.Find(stack.Name)
		if err != nil {
			if err.Error() != "record not found" {
				return fmt.Errorf("error searching existing stack %q: %v", stack.Name, err)
			}
		}
		if err == nil {
			log.Printf("Stack exists: %s\n", existingStack.Name)
			// TODO: For now just bail if the stack exists. This should probably be done differently so we can reload the stack if it has changed
			// If we have running instances we can't just change parameters etc. though
			continue
		}

		createdStack, err := stackService.Create(stack)
		if err != nil {
			return fmt.Errorf("error creating stack %q: %v", stack.Name, err)
		}
		log.Printf("Stack created: %+v\n", createdStack)
	}

	return nil
}

func parseStacks(dir string) ([]*model.Stack, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error reading stack directory %q: %v", dir, err)
	}

	var stacks []*model.Stack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		log.Printf("Parsing stack: %q\n", name)

		meta, err := parseMetadata(dir, name)
		if err != nil {
			return nil, err
		}

		tmpl, err := parseTemplate(dir, name, meta.stackParameters)
		if err != nil {
			return nil, err
		}

		stack := &model.Stack{
			Name:             name,
			HostnamePattern:  meta.hostnamePattern,
			HostnameVariable: meta.hostnameVariable,
		}
		for requiredEnv := range tmpl.requiredEnvs {
			_, isConsumed := meta.consumedParameters[requiredEnv]
			parameter := &model.StackRequiredParameter{Name: requiredEnv, StackName: stack.Name, Consumed: isConsumed}
			stack.RequiredParameters = append(stack.RequiredParameters, *parameter)
		}
		for env, value := range tmpl.envs {
			_, isConsumed := meta.consumedParameters[env]
			parameter := &model.StackOptionalParameter{Name: env, StackName: stack.Name, Consumed: isConsumed, DefaultValue: fmt.Sprintf("%v", value)}
			stack.OptionalParameters = append(stack.OptionalParameters, *parameter)
		}
		stacks = append(stacks, stack)
	}

	return stacks, nil
}

func parseMetadata(dir, name string) (metadata, error) {
	var meta metadata

	path := path.Join(dir, name, "im-metadata.yaml")
	file, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return meta, nil
	}
	if err != nil {
		return meta, fmt.Errorf("error reading stack metadata %q: %v", name, err)
	}

	var metaYaml metadataYaml
	err = yaml.Unmarshal(file, &metaYaml)
	if err != nil {
		return meta, fmt.Errorf("error parsing stack metadata %q: %v", name, err)
	}

	cps := make(map[string]struct{})
	for _, cp := range metaYaml.ConsumedParameters {
		cps[strings.TrimSpace(cp)] = struct{}{}
	}
	sps := make(map[string]struct{})
	for _, sp := range metaYaml.StackParameters {
		sps[strings.TrimSpace(sp)] = struct{}{}
	}
	meta = metadata{
		hostnameVariable:   metaYaml.HostnameVariable,
		hostnamePattern:    metaYaml.HostnamePattern,
		consumedParameters: cps,
		stackParameters:    sps,
	}

	return meta, err
}

func parseTemplate(dir, name string, stackParams map[string]struct{}) (*tmpl, error) {
	path := path.Join(dir, name, "helmfile.yaml")
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading stack template %q: %v", name, err)
	}

	template := newTmpl(name, stackParams)
	err = template.parse(string(file))
	if err != nil {
		return nil, fmt.Errorf("error parsing stack template %q: %v", name, err)
	}

	return template, nil
}

func parseStacksOld(dir string) ([]*model.Stack, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack folder: %s", err)
	}

	var stacks []*model.Stack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		log.Printf("Parsing stack: %q\n", name)
		stack, err := parseStackOld(dir, name)
		if err != nil {
			return nil, fmt.Errorf("error parsing stack %q: %v", name, err)
		}

		stacks = append(stacks, stack)
	}

	return stacks, nil
}

func parseStackOld(dir, name string) (*model.Stack, error) {
	path := fmt.Sprintf("%s/%s/helmfile.yaml", dir, name)
	file, err := os.ReadFile(path)
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

	stack := &model.Stack{
		Name:             name,
		HostnamePattern:  hostnamePattern,
		HostnameVariable: hostnameVariable,
	}
	for _, name := range requiredParams {
		isConsumed := isConsumedParameter(name, consumedParameters)
		parameter := &model.StackRequiredParameter{Name: name, StackName: stack.Name, Consumed: isConsumed}
		stack.RequiredParameters = append(stack.RequiredParameters, *parameter)
	}
	for name, v := range optionalParams {
		isConsumed := isConsumedParameter(name, consumedParameters)
		parameter := &model.StackOptionalParameter{Name: name, StackName: stack.Name, Consumed: isConsumed, DefaultValue: v}
		stack.OptionalParameters = append(stack.OptionalParameters, *parameter)
	}

	return stack, nil
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
