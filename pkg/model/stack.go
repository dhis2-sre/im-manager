package model

// swagger:model StackDetail
type Stack struct {
	Name             string          `json:"name"`
	Parameters       StackParameters `json:"parameters"`
	Instances        []Instance      `json:"instances"`
	HostnamePattern  string          `json:"hostnamePattern"`
	HostnameVariable string          `json:"hostnameVariable"`
	// ParameterProviders provide parameters to other stacks.
	ParameterProviders ParameterProviders `json:"-"`
	// Requires these stacks to deploy an instance of this stack.
	Requires []Stack `json:"requires"`
}

// swagger:model StackDetailParameters
type StackParameters map[string]StackParameter

// swagger:model StackDetailParameter
type StackParameter struct {
	// Name is the user friendly name of the parameter.
	Name         string  `json:"name"`
	DefaultValue *string `json:"defaultValue,omitempty"`
	// Consumed signals that this parameter is provided by another stack i.e. one of the stacks required stacks.
	Consumed bool `json:"consumed"`
	// Validator ensures that the actual stack parameters are valid according to its rules.
	Validator func(value string) error `json:"-"`
	// Priority determines the order in which the parameter is shown
	Priority uint
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
