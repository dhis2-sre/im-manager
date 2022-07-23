package stack

import (
	"bytes"
	"errors"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

// TODO convert helmfiles to new format

// TODO pass in dir?
// func newTmpl(name string) *tmpl {
// 	return &tmpl{
// 		name: name,
// 	}
// }

// tmpl represents a stack template used to create the environment for running a stacks helmfile
// commands.
type tmpl struct {
	name               string
	requiredEnvs       []string
	envs               map[string]any
	hostnameVariable   string
	hostnamePattern    string
	consumedParameters []string
	stackParameters    []string
}

// helmfile represents a helmfile with added instance manager metadata.
type helmfile struct {
	Metadata struct {
		HostnameVariable   string   `yaml:"hostnameVariable,omitempty"`
		HostnamePattern    string   `yaml:"hostnamePattern,omitempty"`
		ConsumedParameters []string `yaml:"consumedParameters,omitempty"`
		StackParameters    []string `yaml:"stackParameters,omitempty"`
	} `yaml:"instanceManager,omitempty"`
	Releases     []map[any]any `yaml:"releases,omitempty"`
	Repositories []map[any]any `yaml:"repositories,omitempty"`
}

func (t *tmpl) parse(in string) error {
	te, err := template.New(t.name).Funcs(template.FuncMap{
		"default":     t.dfault,
		"env":         t.env,
		"indent":      indent,
		"quote":       quote,
		"readFile":    readFile,
		"replace":     replace,
		"requiredEnv": t.requiredEnv,
	}).Parse(in)
	if err != nil {
		return err
	}

	var yl bytes.Buffer
	err = te.Execute(&yl, "")
	if err != nil {
		return err
	}

	// fmt.Println(yl.String())

	var helm helmfile
	err = yaml.Unmarshal(yl.Bytes(), &helm)
	if err != nil {
		return err
	}

	t.hostnameVariable = helm.Metadata.HostnameVariable
	t.hostnamePattern = helm.Metadata.HostnamePattern
	t.consumedParameters = helm.Metadata.ConsumedParameters
	t.stackParameters = helm.Metadata.StackParameters

	return err
}

// TODO sort functions in call order

// requiredEnv replaces the helmfile requiredEnv template function. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/helmfile/helmfile/blob/70d2dd653b5fd7a64d834aa99e07d727d3f4d10d/pkg/tmpl/context_funcs.go#L313
func (t *tmpl) requiredEnv(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("must provide name")
	}
	t.requiredEnvs = append(t.requiredEnvs, name)

	return name, nil
}

// env replaces the Sprig env template function which is actually os.Getenv. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/Masterminds/sprig/blob/5a09ebddb9c54ae8c3531154a6e16b93b87c47a8/functions.go#L267
func (t *tmpl) env(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("must provide name")
	}
	if t.envs == nil {
		t.envs = make(map[string]any)
	}
	t.envs[name] = ""

	return name, nil
}

// TODO should we check the default value in any way? It will be provided otherwise the
// text/template Execute will fail

// dfault replaces the Sprig default template function. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/defaults.go#L26
func (t *tmpl) dfault(d any, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("must provide name")
	}
	t.envs[name] = d

	return name, nil
}

// requiredEnv replaces the helmfile readFile template function. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/helmfile/helmfile/blob/70d2dd653b5fd7a64d834aa99e07d727d3f4d10d/pkg/tmpl/context_funcs.go#L200
func readFile(filename string) (string, error) {
	if strings.TrimSpace(filename) == "" {
		return "", errors.New("must provide filename")
	}

	// f, err := os.ReadFile(filename)
	// if err != nil {
	// 	return "", fmt.Errorf("error reading %q: %v", filename, err)
	// }

	return string(""), nil
}

// replace replaces the Sprig replace template function. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/strings.go#L118
func replace(old, new, src string) (string, error) {
	if strings.TrimSpace(old) == "" {
		return "", errors.New("must provide old")
	}
	if strings.TrimSpace(new) == "" {
		return "", errors.New("must provide new")
	}
	if strings.TrimSpace(src) == "" {
		return "", errors.New("must provide src")
	}

	return old, nil
}

// replace replaces the Sprig indent template function. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/strings.go#L109
func indent(_ int, v string) (string, error) {
	// TODO this related to readFile if I uncomment this then indent fails if readFile returns ""
	// if readFile returns something other than "" the yaml seems to be incorrect

	// if strings.TrimSpace(v) == "" {
	// 	return "", errors.New("must provide v")
	// }

	return v, nil
}

// quote replaces the Sprig indent template function. This implementation ensures stack templates
// calling it provide the correct amount/type of args. String args must be non-blank as this is most
// likely an unintentional mistake.
// https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/strings.go#L83
func quote(str ...interface{}) (string, error) {
	return "", nil
}
