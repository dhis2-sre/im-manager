// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewInstanceNameToIDParams creates a new InstanceNameToIDParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewInstanceNameToIDParams() *InstanceNameToIDParams {
	return &InstanceNameToIDParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewInstanceNameToIDParamsWithTimeout creates a new InstanceNameToIDParams object
// with the ability to set a timeout on a request.
func NewInstanceNameToIDParamsWithTimeout(timeout time.Duration) *InstanceNameToIDParams {
	return &InstanceNameToIDParams{
		timeout: timeout,
	}
}

// NewInstanceNameToIDParamsWithContext creates a new InstanceNameToIDParams object
// with the ability to set a context for a request.
func NewInstanceNameToIDParamsWithContext(ctx context.Context) *InstanceNameToIDParams {
	return &InstanceNameToIDParams{
		Context: ctx,
	}
}

// NewInstanceNameToIDParamsWithHTTPClient creates a new InstanceNameToIDParams object
// with the ability to set a custom HTTPClient for a request.
func NewInstanceNameToIDParamsWithHTTPClient(client *http.Client) *InstanceNameToIDParams {
	return &InstanceNameToIDParams{
		HTTPClient: client,
	}
}

/*
InstanceNameToIDParams contains all the parameters to send to the API endpoint

	for the instance name to Id operation.

	Typically these are written to a http.Request.
*/
type InstanceNameToIDParams struct {

	// GroupName.
	GroupName string

	// InstanceName.
	InstanceName string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the instance name to Id params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *InstanceNameToIDParams) WithDefaults() *InstanceNameToIDParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the instance name to Id params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *InstanceNameToIDParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the instance name to Id params
func (o *InstanceNameToIDParams) WithTimeout(timeout time.Duration) *InstanceNameToIDParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the instance name to Id params
func (o *InstanceNameToIDParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the instance name to Id params
func (o *InstanceNameToIDParams) WithContext(ctx context.Context) *InstanceNameToIDParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the instance name to Id params
func (o *InstanceNameToIDParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the instance name to Id params
func (o *InstanceNameToIDParams) WithHTTPClient(client *http.Client) *InstanceNameToIDParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the instance name to Id params
func (o *InstanceNameToIDParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithGroupName adds the groupName to the instance name to Id params
func (o *InstanceNameToIDParams) WithGroupName(groupName string) *InstanceNameToIDParams {
	o.SetGroupName(groupName)
	return o
}

// SetGroupName adds the groupName to the instance name to Id params
func (o *InstanceNameToIDParams) SetGroupName(groupName string) {
	o.GroupName = groupName
}

// WithInstanceName adds the instanceName to the instance name to Id params
func (o *InstanceNameToIDParams) WithInstanceName(instanceName string) *InstanceNameToIDParams {
	o.SetInstanceName(instanceName)
	return o
}

// SetInstanceName adds the instanceName to the instance name to Id params
func (o *InstanceNameToIDParams) SetInstanceName(instanceName string) {
	o.InstanceName = instanceName
}

// WriteToRequest writes these params to a swagger request
func (o *InstanceNameToIDParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param groupName
	if err := r.SetPathParam("groupName", o.GroupName); err != nil {
		return err
	}

	// path param instanceName
	if err := r.SetPathParam("instanceName", o.InstanceName); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
