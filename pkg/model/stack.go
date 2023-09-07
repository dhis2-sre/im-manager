package model

// swagger:model Stack
type Stack struct {
	Name             string          `json:"name"`
	Parameters       StackParameters `json:"parameters"`
	Instances        []Instance      `json:"instances"`
	HostnamePattern  string          `json:"hostnamePattern"`
	HostnameVariable string          `json:"hostnameVariable"`
	// ParameterProviders provide parameters to other stacks.
	ParameterProviders ParameterProviders `json:"-"`
	// Requires these stacks to deploy an instance of this stack.
	Requires []Stack `json:"-"`
}

type StackParameters map[string]StackParameter

type StackParameter struct {
	DefaultValue *string `json:"defaultValue,omitempty"`
	// Consumed signals that this parameter is provided by another stack i.e. one of the stacks required stacks.
	Consumed bool `json:"consumed"`
	// Validator ensures that the actual stack parameters are valid according to its rules.
	Validator func(value string) error `json:"-"`
}

type ParameterProviders map[string]ParameterProvider

// ParameterProvider provides a value that can be consumed by a stack as a stack parameter.
type ParameterProvider interface {
	Provide(instance Instance) (value string, err error)
}

type ParameterProviderFunc func(instance Instance) (string, error)

func (p ParameterProviderFunc) Provide(instance Instance) (string, error) {
	return p(instance)
}
