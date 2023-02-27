// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// New creates a new operations API client.
func New(transport runtime.ClientTransport, formats strfmt.Registry) ClientService {
	return &Client{transport: transport, formats: formats}
}

/*
Client for operations API
*/
type Client struct {
	transport runtime.ClientTransport
	formats   strfmt.Registry
}

// ClientOption is the option for Client methods
type ClientOption func(*runtime.ClientOperation)

// ClientService is the interface for Client methods
type ClientService interface {
	DeleteInstance(params *DeleteInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*DeleteInstanceAccepted, error)

	DeployInstance(params *DeployInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*DeployInstanceCreated, error)

	FindByID(params *FindByIDParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*FindByIDOK, error)

	FindByIDDecrypted(params *FindByIDDecryptedParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*FindByIDDecryptedOK, error)

	Health(params *HealthParams, opts ...ClientOption) (*HealthOK, error)

	InstanceLogs(params *InstanceLogsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*InstanceLogsOK, error)

	InstanceNameToID(params *InstanceNameToIDParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*InstanceNameToIDOK, error)

	ListInstances(params *ListInstancesParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListInstancesOK, error)

	ListPresets(params *ListPresetsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListPresetsOK, error)

	PauseInstance(params *PauseInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*PauseInstanceAccepted, error)

	PostIntegration(params *PostIntegrationParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*PostIntegrationOK, error)

	RestartInstance(params *RestartInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*RestartInstanceAccepted, error)

	Stack(params *StackParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*StackOK, error)

	Stacks(params *StacksParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*StacksOK, error)

	UpdateInstance(params *UpdateInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*UpdateInstanceNoContent, error)

	SetTransport(transport runtime.ClientTransport)
}

/*
DeleteInstance deletes instance

Delete an instance by id
*/
func (a *Client) DeleteInstance(params *DeleteInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*DeleteInstanceAccepted, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDeleteInstanceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "deleteInstance",
		Method:             "DELETE",
		PathPattern:        "/instances/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DeleteInstanceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DeleteInstanceAccepted)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for deleteInstance: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
DeployInstance deploys instance

Deploy an instance...
*/
func (a *Client) DeployInstance(params *DeployInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*DeployInstanceCreated, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDeployInstanceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "deployInstance",
		Method:             "POST",
		PathPattern:        "/instances",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DeployInstanceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DeployInstanceCreated)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for deployInstance: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
FindByID finds instance

Find an instance by id
*/
func (a *Client) FindByID(params *FindByIDParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*FindByIDOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewFindByIDParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "findById",
		Method:             "GET",
		PathPattern:        "/instances/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &FindByIDReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*FindByIDOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for findById: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
FindByIDDecrypted finds decrypted instance

Find instance by id with decrypted parameters
*/
func (a *Client) FindByIDDecrypted(params *FindByIDDecryptedParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*FindByIDDecryptedOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewFindByIDDecryptedParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "findByIdDecrypted",
		Method:             "GET",
		PathPattern:        "/instances/{id}/parameters",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &FindByIDDecryptedReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*FindByIDDecryptedOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for findByIdDecrypted: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
Health healths status

Show service health status
*/
func (a *Client) Health(params *HealthParams, opts ...ClientOption) (*HealthOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewHealthParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "health",
		Method:             "GET",
		PathPattern:        "/health",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &HealthReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*HealthOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for health: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
InstanceLogs streams logs

Stream instance logs in real time
*/
func (a *Client) InstanceLogs(params *InstanceLogsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*InstanceLogsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewInstanceLogsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "instanceLogs",
		Method:             "GET",
		PathPattern:        "/instances/{id}/logs",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &InstanceLogsReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*InstanceLogsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for instanceLogs: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
InstanceNameToID finds an instance

Find instance id by name and group name
*/
func (a *Client) InstanceNameToID(params *InstanceNameToIDParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*InstanceNameToIDOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewInstanceNameToIDParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "instanceNameToId",
		Method:             "GET",
		PathPattern:        "/instances-name-to-id/{groupName}/{instanceName}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &InstanceNameToIDReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*InstanceNameToIDOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for instanceNameToId: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
ListInstances lists instances

List all instances accessible by the user
*/
func (a *Client) ListInstances(params *ListInstancesParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListInstancesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListInstancesParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "listInstances",
		Method:             "GET",
		PathPattern:        "/instances",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListInstancesReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListInstancesOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listInstances: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
ListPresets lists presets

List all presets accessible by the user
*/
func (a *Client) ListPresets(params *ListPresetsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListPresetsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListPresetsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "listPresets",
		Method:             "GET",
		PathPattern:        "/presets",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListPresetsReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListPresetsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listPresets: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
PauseInstance pauses instance

Pause an instance...
*/
func (a *Client) PauseInstance(params *PauseInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*PauseInstanceAccepted, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPauseInstanceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "pauseInstance",
		Method:             "PUT",
		PathPattern:        "/instances/{id}/pause",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PauseInstanceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PauseInstanceAccepted)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for pauseInstance: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
PostIntegration integrations

Return integration for a given key
*/
func (a *Client) PostIntegration(params *PostIntegrationParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*PostIntegrationOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPostIntegrationParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "postIntegration",
		Method:             "POST",
		PathPattern:        "/integrations",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PostIntegrationReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PostIntegrationOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for postIntegration: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
RestartInstance restarts instance

Restart an instance...
*/
func (a *Client) RestartInstance(params *RestartInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*RestartInstanceAccepted, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewRestartInstanceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "restartInstance",
		Method:             "PUT",
		PathPattern:        "/instances/{id}/restart",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &RestartInstanceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*RestartInstanceAccepted)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for restartInstance: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
Stack finds stack

Find stack by name
*/
func (a *Client) Stack(params *StackParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*StackOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewStackParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "stack",
		Method:             "GET",
		PathPattern:        "/stacks/{name}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &StackReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*StackOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for stack: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
Stacks finds all stacks

Find all stacks...
*/
func (a *Client) Stacks(params *StacksParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*StacksOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewStacksParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "stacks",
		Method:             "GET",
		PathPattern:        "/stacks",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &StacksReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*StacksOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for stacks: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
UpdateInstance updates instance

Update an instance...
*/
func (a *Client) UpdateInstance(params *UpdateInstanceParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*UpdateInstanceNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewUpdateInstanceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "updateInstance",
		Method:             "PUT",
		PathPattern:        "/instances/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &UpdateInstanceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*UpdateInstanceNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for updateInstance: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

// SetTransport changes the transport on the client
func (a *Client) SetTransport(transport runtime.ClientTransport) {
	a.transport = transport
}
