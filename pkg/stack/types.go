package stack

import (
	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// swagger:model StackDetail
type Stack struct {
	Name string `json:"name"`
	// ReleaseSuffix follows the instance name in the name of the one helm release the stack's helmfile
	// installs, so the release can be removed without rendering the helmfile.
	ReleaseSuffix string          `json:"-"`
	Parameters    StackParameters `json:"parameters"`
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
	Companions []Companion `json:"companions"`
	// Endpoints are the addresses an instance of this stack is reached at, each the path the stack's
	// helmfile gives an ingress. A stack reached only through another, as chap is through the route
	// it registers in DHIS2, declares none.
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	// Components are the addressable parts a deployed instance of this stack consists of. Their
	// names equal the im-type label values the stack's helmfile applies.
	Components []kube.Component `json:"-"`
}

// ReleaseName is the name of the helm release the stack's helmfile installs for an instance.
func (s Stack) ReleaseName(instanceName string) string {
	return instanceName + s.ReleaseSuffix
}

// Companion is a stack that can be deployed alongside the stack declaring it, optionally only when
// a condition over the declaring stack's parameters holds. The condition is data rather than code,
// so a deploy form decides whether to offer the companion from the same declaration the backend
// reads, and the two cannot disagree.
type Companion struct {
	Stack Stack           `json:"stack"`
	When  *kube.Condition `json:"when,omitempty"`
}

// Endpoint is an address an instance is reached at: its hostname and path, followed by PathSuffix.
// An empty suffix is the instance's own address. A condition means the address exists only while it
// holds over the stack's parameters, as Glowroot's does while DEPLOY_GLOWROOT is on. The suffix is
// data so a client builds and offers the address from the same declaration the helmfile is checked
// against, rather than keeping a copy of the template's path.
// swagger:model StackEndpoint
type Endpoint struct {
	PathSuffix string          `json:"pathSuffix"`
	When       *kube.Condition `json:"when,omitempty"`
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
	// Sections are the titled sub-sections a client sorts the group's advanced parameters into, in
	// the order it shows them. A parameter names its section, and one naming none is listed after
	// them, so a new parameter shows up without anyone having to place it. Empty means the group's
	// advanced parameters are one list.
	Sections []ParameterSection `json:"sections,omitempty"`
}

// ParameterSection is a titled sub-section of a group's advanced parameters, such as the health
// checks of DHIS 2 core. Like the group it is presentation only, and nothing here reads it.
// swagger:model StackParameterSection
type ParameterSection struct {
	Name  string `json:"name"`
	Title string `json:"title"`
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
	// Validator ensures that the actual stack parameters are valid according to its rules. A
	// Choice or the boolean validator also decides the type and allowed values the stacks API
	// serves, so a form offers exactly the values accepted here.
	Validator Validator `json:"-"`
	// Priority determines the order in which the parameter is shown.
	Priority  uint `json:"priority"`
	Sensitive bool `json:"sensitive"`
	// Required says the stack's helmfile cannot render without a value for this parameter, so an
	// instance is rejected while it is blank rather than failing in helmfile after it was accepted.
	Required bool `json:"required"`
	// ImmutableReason says why the parameter cannot be changed once the instance has been deployed;
	// empty means it can. The update path rejects a change to such a parameter and a client renders
	// it read only with this as the explanation, so the two cannot disagree about what is editable.
	ImmutableReason string `json:"immutableReason,omitempty"`
	// Description says what the parameter is for in a sentence a client can show beside it; empty
	// means the display name is all there is to say.
	Description string `json:"description,omitempty"`
	// AddOn says this 'true'/'false' parameter switches on an optional part of the instance, so a
	// client can offer it as something to add rather than as one more setting. It is presentation
	// only and nothing here reads it: what the parameter switches on is whatever the stack already
	// gates on it, a companion, parameter groups, an endpoint, or nothing a client can show.
	AddOn bool `json:"addOn,omitempty"`
	// Primary says a client shows this parameter up front rather than among its group's advanced
	// settings. Most parameters have a default nobody needs to look at, so advanced is the zero
	// value and a new parameter lands there until someone decides it belongs up front. A parameter
	// that has to be filled in, being required with no default, is primary. So is the choice a
	// conditional group of another group depends on, along with that group's own parameters, since
	// the group is shown beside the choice rather than tucked away. It is presentation only and
	// nothing here reads it.
	Primary bool `json:"primary,omitempty"`
	// Section names the section of the parameter's group it is listed under among the advanced
	// settings; empty means after the group's sections. A primary parameter has none.
	Section string `json:"section,omitempty"`
	// NotRendered says the stack's own helmfile never reads this parameter, so changing it changes
	// nothing about what the stack renders and an edit that touches only it leaves the instance
	// alone. It is for the parameters whose only job is to decide whether a companion belongs to the
	// deployment: ENABLE_PGADMIN appears nowhere in dhis2-v2's template, while DEPLOY_CHAP gates chap
	// and is read by the template, so only the first is declared here. A parameter that starts being
	// read by its stack's template has to lose this, or the cluster is left stale.
	NotRendered bool `json:"-"`
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
