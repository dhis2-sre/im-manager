package model

type Stack struct {
	Name             string
	Parameters       StackParameters
	Instances        []Instance
	HostnamePattern  string
	HostnameVariable string
	// ParameterProviders provide parameters to other stacks.
	ParameterProviders ParameterProviders
	// Requires these stacks to deploy an instance of this stack.
	Requires []Stack
}

type StackParameters map[string]StackParameter

type StackParameter struct {
	DefaultValue *string
	// Consumed signals that this parameter is provided by another stack i.e. one of the stacks required stacks.
	Consumed bool
	// Validator ensures that the actual stack parameters are valid according to its rules.
	Validator func(value string) error
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
