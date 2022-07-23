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
}

// TODO pass in name of stack? for better error message
func parse(in string) (*tmpl, error) {
	result := &tmpl{}

	t, err := template.New("").Funcs(template.FuncMap{
		"requiredEnv": requiredEnv(result),
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

// requiredEnv replaces the helmfile requiredEnv template function. It ensures stack templates
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
