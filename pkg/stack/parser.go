package stack

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
)

type tmpl struct {
	requiredEnvs []string
}

// TODO pass in name of stack? for better error message
func parse(in string) (*tmpl, error) {
	result := &tmpl{}

	t, err := template.New("").Funcs(template.FuncMap{
		// TODO add comment
		// https://github.com/roboll/helmfile/blob/9dca4fea5926c2ae4266e4ac1cad3b72ef9afed9/pkg/tmpl/context_funcs.go#L249
		"requiredEnv": func(name string) (string, error) {
			if strings.TrimSpace(name) == "" {
				return "", errors.New("must provide name")
			}
			fmt.Println(name)
			result.requiredEnvs = append(result.requiredEnvs, name)

			return name, nil
		},
	}).Parse(in)
	if err != nil {
		return nil, err
	}
	err = t.Execute(io.Discard, "")

	return result, err
}
