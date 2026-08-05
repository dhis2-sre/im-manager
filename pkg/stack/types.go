package stack

import (
	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// swagger:model StackDetail
type Stack struct {
	Name       string          `json:"name"`
	Parameters StackParameters `json:"parameters"`
	// ParameterGroups are the sections a client renders the parameters in, in order. A group's
	// condition decides its visibility as the user edits the enabling parameter; component
	// presence predicates reference the same conditions so form and backend cannot disagree.
	ParameterGroups  []ParameterGroup `json:"parameterGroups,omitempty"`
	HostnamePattern  string           `json:"hostnamePattern"`
	HostnameVariable string           `json:"hostnameVariable"`
	// ParameterProviders provide parameters to other stacks.
	ParameterProviders ParameterProviders `json:"-"`
	// Requires these stacks to deploy an instance of this stack.
	Requires []Stack `json:"requires"`
	// Companions are optional stacks that can be deployed alongside this stack. Certain parameters can require a companion stack.
	Companions []Stack `json:"companions"`
	// Components are the addressable parts a deployed instance of this stack consists of. Their
	// names equal the im-type label values the stack's helmfile applies.
	Components []kube.Component `json:"-"`
}

// ParameterGroup is a named section of a stack's parameters, naturally the component the
// parameters belong to, with a human-readable title.
// swagger:model StackParameterGroup
type ParameterGroup struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	// When gates the group's visibility on the value of another parameter; nil means the group is
	// always shown.
	When *kube.Condition `json:"when,omitempty"`
	// Parameters declared inside the group. withGroupedParameters flattens them into the stack's
	// parameter map, stamping each parameter's Group, so membership follows from where a parameter
	// is declared. Not serialized: the API serves the flat list where each parameter names its group.
	Parameters StackParameters `json:"-"`
}

// swagger:model StackDetailParameters
type StackParameters map[string]StackParameter

// swagger:model StackDetailParameter
type StackParameter struct {
	// DisplayName is the user-friendly name of the parameter.
	DisplayName  string  `json:"displayName"`
	DefaultValue *string `json:"defaultValue,omitempty"`
	// Consumed signals that this parameter is provided by another stack i.e. one of the stacks required stacks.
	Consumed bool `json:"consumed"`
	// Validator ensures that the actual stack parameters are valid according to its rules.
	Validator func(value string) error `json:"-"`
	// Priority determines the order in which the parameter is shown.
	Priority  uint `json:"priority"`
	Sensitive bool `json:"sensitive"`
	// Group names the ParameterGroup this parameter belongs to; empty means ungrouped, which
	// clients render in a single flat section. Populated by withGroupedParameters from the
	// declaring group, never set by hand.
	Group            string               `json:"group,omitempty"`
	RequireCompanion RequireCompanionFunc `json:"-"`
}

type ParameterProviders map[string]ParameterProvider

// ParameterProvider provides a value that can be consumed by a stack as a stack parameter.
type ParameterProvider interface {
	Provide(instance model.DeploymentInstance) (value string, err error)
}

type ParameterProviderFunc func(instance model.DeploymentInstance) (string, error)

func (p ParameterProviderFunc) Provide(instance model.DeploymentInstance) (string, error) {
	return p(instance)
}

type RequireCompanionFunc func(instance model.DeploymentInstanceParameter) (*Stack, error)

func (r RequireCompanionFunc) Require(parameter model.DeploymentInstanceParameter) (*Stack, error) {
	return r(parameter)
}
