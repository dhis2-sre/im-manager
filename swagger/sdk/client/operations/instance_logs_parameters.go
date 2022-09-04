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
	"github.com/go-openapi/swag"
)

// NewInstanceLogsParams creates a new InstanceLogsParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewInstanceLogsParams() *InstanceLogsParams {
	return &InstanceLogsParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewInstanceLogsParamsWithTimeout creates a new InstanceLogsParams object
// with the ability to set a timeout on a request.
func NewInstanceLogsParamsWithTimeout(timeout time.Duration) *InstanceLogsParams {
	return &InstanceLogsParams{
		timeout: timeout,
	}
}

// NewInstanceLogsParamsWithContext creates a new InstanceLogsParams object
// with the ability to set a context for a request.
func NewInstanceLogsParamsWithContext(ctx context.Context) *InstanceLogsParams {
	return &InstanceLogsParams{
		Context: ctx,
	}
}

// NewInstanceLogsParamsWithHTTPClient creates a new InstanceLogsParams object
// with the ability to set a custom HTTPClient for a request.
func NewInstanceLogsParamsWithHTTPClient(client *http.Client) *InstanceLogsParams {
	return &InstanceLogsParams{
		HTTPClient: client,
	}
}

/* InstanceLogsParams contains all the parameters to send to the API endpoint
   for the instance logs operation.

   Typically these are written to a http.Request.
*/
type InstanceLogsParams struct {

	// ID.
	//
	// Format: uint64
	ID uint64

	/* Selector.

	   selector
	*/
	Selector *string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the instance logs params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *InstanceLogsParams) WithDefaults() *InstanceLogsParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the instance logs params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *InstanceLogsParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the instance logs params
func (o *InstanceLogsParams) WithTimeout(timeout time.Duration) *InstanceLogsParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the instance logs params
func (o *InstanceLogsParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the instance logs params
func (o *InstanceLogsParams) WithContext(ctx context.Context) *InstanceLogsParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the instance logs params
func (o *InstanceLogsParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the instance logs params
func (o *InstanceLogsParams) WithHTTPClient(client *http.Client) *InstanceLogsParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the instance logs params
func (o *InstanceLogsParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithID adds the id to the instance logs params
func (o *InstanceLogsParams) WithID(id uint64) *InstanceLogsParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the instance logs params
func (o *InstanceLogsParams) SetID(id uint64) {
	o.ID = id
}

// WithSelector adds the selector to the instance logs params
func (o *InstanceLogsParams) WithSelector(selector *string) *InstanceLogsParams {
	o.SetSelector(selector)
	return o
}

// SetSelector adds the selector to the instance logs params
func (o *InstanceLogsParams) SetSelector(selector *string) {
	o.Selector = selector
}

// WriteToRequest writes these params to a swagger request
func (o *InstanceLogsParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param id
	if err := r.SetPathParam("id", swag.FormatUint64(o.ID)); err != nil {
		return err
	}

	if o.Selector != nil {

		// query param selector
		var qrSelector string

		if o.Selector != nil {
			qrSelector = *o.Selector
		}
		qSelector := qrSelector
		if qSelector != "" {

			if err := r.SetQueryParam("selector", qSelector); err != nil {
				return err
			}
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
