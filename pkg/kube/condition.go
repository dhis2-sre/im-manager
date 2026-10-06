package kube

import "github.com/dhis2-sre/im-manager/pkg/model"

// Condition is a declarative predicate over an instance's parameters: it holds when the named
// parameter has the given value. Being data rather than code, the same condition both gates a
// component's presence on the backend and is served to clients, so forms can react to parameter
// edits without a round trip and cannot disagree with the backend about when something exists.
type Condition struct {
	Parameter string `json:"parameter"`
	Equals    string `json:"equals"`
}

// Matches reports whether the condition holds for the given decrypted parameters. A nil condition
// always holds.
func (c *Condition) Matches(params model.DeploymentInstanceParameters) bool {
	if c == nil {
		return true
	}
	return params[c.Parameter].Value == c.Equals
}
