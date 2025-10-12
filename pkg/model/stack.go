package model

type KubernetesResource string

const (
	DeploymentResource  = KubernetesResource("deployment")
	StatefulSetResource = KubernetesResource("statefulSet")
)

// swagger:model StackDetail
type Stack struct {
	Name             string          `json:"name"`
	Parameters       StackParameters `json:"parameters"`
	HostnamePattern  string          `json:"hostnamePattern"`
	HostnameVariable string          `json:"hostnameVariable"`
	// ParameterProviders provide parameters to other stacks.
	ParameterProviders ParameterProviders `json:"-"`
	// Requires these stacks to deploy an instance of this stack.
	Requires []Stack `json:"requires"`
	// Companions are optional stacks that can be deployed alongside this stack. Certain parameters can require a companion stack.
	Companions         []Stack `json:"companions"`
	KubernetesResource KubernetesResource
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
	Priority         uint   `json:"priority"`
	Sensitive        bool   `json:"sensitive"`
	RequireCompanion RequireCompanionFunc `json:"-"`
}

type ParameterProviders map[string]ParameterProvider

// ParameterProvider provides a value that can be consumed by a stack as a stack parameter.
type ParameterProvider interface {
	Provide(instance DeploymentInstance) (value string, err error)
}

type ParameterProviderFunc func(instance DeploymentInstance) (string, error)

func (p ParameterProviderFunc) Provide(instance DeploymentInstance) (string, error) {
	return p(instance)
}

type RequireCompanionFunc func(instance DeploymentInstanceParameter) (*Stack, error)

func (r RequireCompanionFunc) Require(parameter DeploymentInstanceParameter) (*Stack, error) {
	return r(parameter)
}
