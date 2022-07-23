package stack

import (
	"errors"
	"io"
	"strings"
	"text/template"
)

// TODO use a new func with all functions being methods?
// func newTemplate() (*tmpl, error) {
// 	template := &tmpl{}
//
// 	return template, nil
// }

type tmpl struct {
	requiredEnvs []string
	envs         map[string]any
}

// TODO pass in name of stack? for better error message
func parse(in string) (*tmpl, error) {
	result := &tmpl{
		envs: make(map[string]any),
	}

	t, err := template.New("").Funcs(template.FuncMap{
		"requiredEnv": requiredEnv(result),
		"env":         env(result),
		"default":     dfault(result),
	}).Parse(in)
	if err != nil {
		return nil, err
	}

	err = t.Execute(io.Discard, "")
	if err != nil {
		return nil, err
	}

	return result, err
}

// requiredEnv replaces the helmfile requiredEnv template function. This implementation ensures stack templates
// calling requiredEnv provide one arg of type string which is not blank.
// https://github.com/helmfile/helmfile/blob/70d2dd653b5fd7a64d834aa99e07d727d3f4d10d/pkg/tmpl/context_funcs.go#L313
func requiredEnv(result *tmpl) func(string) (string, error) {
	return func(name string) (string, error) {
		if strings.TrimSpace(name) == "" {
			return "", errors.New("must provide name")
		}
		result.requiredEnvs = append(result.requiredEnvs, name)

		return name, nil
	}
}

// env replaces the Sprig env template function which is actually os.Getenv. This implementation ensures stack templates
// calling env provide one arg of type string which is not blank.
// https://github.com/Masterminds/sprig/blob/5a09ebddb9c54ae8c3531154a6e16b93b87c47a8/functions.go#L267
func env(result *tmpl) func(string) (string, error) {
	return func(name string) (string, error) {
		if strings.TrimSpace(name) == "" {
			return "", errors.New("must provide name")
		}
		result.envs[name] = ""

		return name, nil
	}
}

// dfault replaces the Sprig default template function. This implementation ensures stack templates
// calling default provide two args. It expects a string as a second argument as we only use it to
// default environment variables retrieved via function env which are not empty or not set.
// https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/defaults.go#L26
func dfault(result *tmpl) func(any, string) (string, error) {
	return func(d any, name string) (string, error) {
		if strings.TrimSpace(name) == "" {
			return "", errors.New("must provide name")
		}
		// TODO should we check the default value in any way? It will be provided otherwise the
		// text/template Execute will fail
		result.envs[name] = d

		return name, nil
	}
}
