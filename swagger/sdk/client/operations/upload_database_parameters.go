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

// NewUploadDatabaseParams creates a new UploadDatabaseParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewUploadDatabaseParams() *UploadDatabaseParams {
	return &UploadDatabaseParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewUploadDatabaseParamsWithTimeout creates a new UploadDatabaseParams object
// with the ability to set a timeout on a request.
func NewUploadDatabaseParamsWithTimeout(timeout time.Duration) *UploadDatabaseParams {
	return &UploadDatabaseParams{
		timeout: timeout,
	}
}

// NewUploadDatabaseParamsWithContext creates a new UploadDatabaseParams object
// with the ability to set a context for a request.
func NewUploadDatabaseParamsWithContext(ctx context.Context) *UploadDatabaseParams {
	return &UploadDatabaseParams{
		Context: ctx,
	}
}

// NewUploadDatabaseParamsWithHTTPClient creates a new UploadDatabaseParams object
// with the ability to set a custom HTTPClient for a request.
func NewUploadDatabaseParamsWithHTTPClient(client *http.Client) *UploadDatabaseParams {
	return &UploadDatabaseParams{
		HTTPClient: client,
	}
}

/*
UploadDatabaseParams contains all the parameters to send to the API endpoint

	for the upload database operation.

	Typically these are written to a http.Request.
*/
type UploadDatabaseParams struct {

	/* File.

	   Upload database request body parameter
	*/
	File runtime.NamedReadCloser

	/* Group.

	   Upload database request body parameter
	*/
	Group string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the upload database params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UploadDatabaseParams) WithDefaults() *UploadDatabaseParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the upload database params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UploadDatabaseParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the upload database params
func (o *UploadDatabaseParams) WithTimeout(timeout time.Duration) *UploadDatabaseParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the upload database params
func (o *UploadDatabaseParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the upload database params
func (o *UploadDatabaseParams) WithContext(ctx context.Context) *UploadDatabaseParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the upload database params
func (o *UploadDatabaseParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the upload database params
func (o *UploadDatabaseParams) WithHTTPClient(client *http.Client) *UploadDatabaseParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the upload database params
func (o *UploadDatabaseParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithFile adds the file to the upload database params
func (o *UploadDatabaseParams) WithFile(file runtime.NamedReadCloser) *UploadDatabaseParams {
	o.SetFile(file)
	return o
}

// SetFile adds the file to the upload database params
func (o *UploadDatabaseParams) SetFile(file runtime.NamedReadCloser) {
	o.File = file
}

// WithGroup adds the group to the upload database params
func (o *UploadDatabaseParams) WithGroup(group string) *UploadDatabaseParams {
	o.SetGroup(group)
	return o
}

// SetGroup adds the group to the upload database params
func (o *UploadDatabaseParams) SetGroup(group string) {
	o.Group = group
}

// WriteToRequest writes these params to a swagger request
func (o *UploadDatabaseParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error
	// form file param File
	if err := r.SetFileParam("File", o.File); err != nil {
		return err
	}

	// form param Group
	frGroup := o.Group
	fGroup := frGroup
	if fGroup != "" {
		if err := r.SetFormParam("Group", fGroup); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}