// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// DeployInstanceRequest deploy instance request
//
// swagger:model DeployInstanceRequest
type DeployInstanceRequest struct {

	// group
	Group string `json:"groupName,omitempty"`

	// name
	Name string `json:"name,omitempty"`

	// parameters
	Parameters map[string]Parameter `json:"parameters,omitempty"`

	// preset instance
	PresetInstance uint64 `json:"presetInstance,omitempty"`

	// source instance
	SourceInstance uint64 `json:"sourceInstance,omitempty"`

	// stack
	Stack string `json:"stackName,omitempty"`
}

// Validate validates this deploy instance request
func (m *DeployInstanceRequest) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateParameters(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *DeployInstanceRequest) validateParameters(formats strfmt.Registry) error {
	if swag.IsZero(m.Parameters) { // not required
		return nil
	}

	for k := range m.Parameters {

		if err := validate.Required("parameters"+"."+k, "body", m.Parameters[k]); err != nil {
			return err
		}
		if val, ok := m.Parameters[k]; ok {
			if err := val.Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("parameters" + "." + k)
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("parameters" + "." + k)
				}
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this deploy instance request based on the context it is used
func (m *DeployInstanceRequest) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateParameters(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *DeployInstanceRequest) contextValidateParameters(ctx context.Context, formats strfmt.Registry) error {

	for k := range m.Parameters {

		if val, ok := m.Parameters[k]; ok {
			if err := val.ContextValidate(ctx, formats); err != nil {
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *DeployInstanceRequest) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *DeployInstanceRequest) UnmarshalBinary(b []byte) error {
	var res DeployInstanceRequest
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
