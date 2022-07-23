package stack

import (
	"io"
	"text/template"
)

type tmpl struct {
	requiredEnvs []string
}

// TODO pass in name of stack? for better error message
func parse(in string) (*tmpl, error) {
	result := &tmpl{}

	t, err := template.New("").Funcs(template.FuncMap{
		"requiredEnv": func(name string) (string, error) {
			// if strings.TrimSpace(name) == "" {
			// 	return "", errors.New("requiredEnv() must provide name")
			// }
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
