// Package stack contains stacks that can be deployed with the instance manager. Stacks have
// parameters as their input which are used to render helmfile templates. Stacks might depend
// on other stacks to provide a parameter (consumed parameter). Stack parameters declared here are
// kept in sync with the helmfile template. No cycle is allowed within our stacks as this would lead
// to undeployable stacks. No two stacks are allowed to provide the same parameter for another stack
// as this is an ambiguity that cannot be automatically resolved.
package stack

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dominikbraun/graph"
	"golang.org/x/exp/slices"
	k8s "k8s.io/api/core/v1"
)

// Stacks represents all deployable stacks.
type Stacks map[string]Stack

// All lists every stack the service serves. main.go registers exactly this list and the tests
// derive their expectations from it, so a new stack is added here and nowhere else.
var All = []Stack{
	DHIS2DB,
	MINIO,
	DHIS2Core,
	DHIS2,
	DHIS2V2,
	PgAdmin,
	WhoamiGo,
	IMJobRunner,
	Chap,
}

// New creates stacks ensuring consumed parameters are provided by required stacks.
func New(stacks ...Stack) (Stacks, error) {
	_, err := ValidateNoCycles(stacks)
	if err != nil {
		return nil, err
	}

	err = ValidateConsumedParameters(stacks)
	if err != nil {
		return nil, err
	}

	err = ValidateCompanionConditions(stacks)
	if err != nil {
		return nil, err
	}

	result := make(Stacks, len(stacks))
	for _, s := range stacks {
		result[s.Name] = s
	}
	return result, nil
}

// ValidateNoCycles validates that the stacks graph does not contain a cycle. The stacks form a
// graph via the required stacks forming a directed edge from the stack to the required stack.
// Stacks with cycles would lead to undeployable instances. There would not be an order (no solution
// to topological sort) in which we could deploy instances in.
func ValidateNoCycles(stacks []Stack) (graph.Graph[string, Stack], error) {
	g := graph.New(func(stack Stack) string {
		return stack.Name
	}, graph.Directed(), graph.PreventCycles())

	for _, stack := range stacks {
		err := g.AddVertex(stack)
		if err != nil {
			return nil, fmt.Errorf("failed adding vertex for stack %q: %v", stack.Name, err)
		}
	}

	for _, src := range stacks {
		for _, dest := range src.Requires {
			err := g.AddEdge(src.Name, dest.Name)
			if err != nil {
				if errors.Is(err, graph.ErrEdgeAlreadyExists) {
					return nil, fmt.Errorf("stack %q requires %q more than once", src.Name, dest.Name)
				} else if errors.Is(err, graph.ErrEdgeCreatesCycle) {
					return nil, fmt.Errorf("edge from stack %q to stack %q creates a cycle", src.Name, dest.Name)
				}
				return nil, fmt.Errorf("failed adding edge from stack %q to stack %q: %v", src.Name, dest.Name, err)
			}
		}
	}

	return g, nil
}

// ValidateConsumedParameters validates all consumed parameters are provided by exactly one of the
// required stacks. Required stacks need to provide at least one consumed parameter.
func ValidateConsumedParameters(stacks []Stack) error {
	var errs []error
	for _, stack := range stacks { // validate each stacks consumed parameters are provided by its required stacks
		requiredStacks := make(map[string]int)
		for _, requiredStack := range stack.Requires {
			requiredStacks[requiredStack.Name] = 0
		}

		// collect all consumed parameters
		consumedParameterProviders := make(map[string]int)
		for name, parameter := range stack.Parameters {
			if !parameter.Consumed {
				continue
			}
			consumedParameterProviders[name] = 0
		}

		// generate frequency map of provided parameters
		for _, requiredStack := range stack.Requires {
			for parameterName, parameter := range requiredStack.Parameters {
				if parameter.Consumed { // consumed parameters cannot be provided
					continue
				}
				_, ok := consumedParameterProviders[parameterName]
				if ok {
					consumedParameterProviders[parameterName]++
					requiredStacks[requiredStack.Name]++
				}
			}
			for parameterName := range requiredStack.ParameterProviders {
				_, ok := consumedParameterProviders[parameterName]
				if ok {
					consumedParameterProviders[parameterName]++
					requiredStacks[requiredStack.Name]++
				}
			}
		}

		for parameter, providerCount := range consumedParameterProviders {
			if providerCount == 0 {
				errs = append(errs, fmt.Errorf("no provider for stack %q parameter %q", stack.Name, parameter))
			}
			if providerCount > 1 {
				errs = append(errs, fmt.Errorf("every consumed parameter must have exactly one provider. %d provider(s) for stack %q parameter %q", providerCount, stack.Name, parameter))
			}
		}

		for requiredStackName, providedCount := range requiredStacks {
			if providedCount == 0 {
				errs = append(errs, fmt.Errorf("stack %q requires %q but does not consume from %q", stack.Name, requiredStackName, requiredStackName))
			}
		}
	}

	return errors.Join(errs...)
}

// ValidateCompanionConditions validates that every companion condition names a parameter the
// declaring stack actually has. A condition over a parameter that does not exist never holds, so a
// deploy form would silently never offer the companion.
func ValidateCompanionConditions(stacks []Stack) error {
	var errs []error
	for _, stack := range stacks {
		for _, companion := range stack.Companions {
			if companion.When == nil {
				continue
			}
			if _, ok := stack.Parameters[companion.When.Parameter]; !ok {
				errs = append(errs, fmt.Errorf("stack %q offers companion %q conditional on parameter %q which it does not have", stack.Name, companion.Stack.Name, companion.When.Parameter))
			}
		}
	}

	return errors.Join(errs...)
}

const ifNotPresent = "IfNotPresent"
const always = "Always"

// withGroupedParameters flattens the parameters declared inside a stack's groups into the stack's
// parameter map, stamping each parameter's Group with the declaring group's name, so membership
// follows from where a parameter is declared instead of a string reference. Panics on a duplicate
// parameter name since that is a definition error caught by any test run.
func withGroupedParameters(s Stack) Stack {
	if s.Parameters == nil {
		s.Parameters = StackParameters{}
	}
	for _, group := range s.ParameterGroups {
		for name, parameter := range group.Parameters {
			if _, ok := s.Parameters[name]; ok {
				panic(fmt.Sprintf("stack %q: parameter %q declared more than once", s.Name, name))
			}
			parameter.Group = group.Name
			s.Parameters[name] = parameter
		}
	}
	return s
}

var imagePullPolicy = OneOf(string(k8s.PullAlways), string(k8s.PullNever), string(k8s.PullIfNotPresent))

const (
	filesystemStorage = "filesystem"
	minIOStorage      = "minio"
	s3Storage         = "s3"
)

// storage validates the value is one of our storage types.
var storage = OneOf(minIOStorage, s3Storage, filesystemStorage)

const (
	strict = "strict"
	lax    = "lax"
	none   = "none"
)

// sameSiteCookies validates the value is one of our same site cookie types.
var sameSiteCookies = OneOf(strict, lax, none)

// OneOf creates a function returning an error when called with a value that is not any of the given
// validValues.
func OneOf(validValues ...string) func(value string) error {
	fmtErrorArg := quoteStrings(validValues)

	return func(value string) error {
		if slices.Contains(validValues, value) {
			return nil
		}

		return fmt.Errorf("%q is not valid, only %s are allowed", value, fmtErrorArg)
	}
}

// quoteStrings quotes values and comma separates them into a joint string.
func quoteStrings(values []string) string {
	var result strings.Builder
	for i, validValue := range values {
		result.WriteString(strconv.Quote(validValue))
		if i+1 < len(values) {
			result.WriteString(", ")
		}
	}
	return result.String()
}
