// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// RestartInstanceReader is a Reader for the RestartInstance structure.
type RestartInstanceReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *RestartInstanceReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 202:
		result := NewRestartInstanceAccepted()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewRestartInstanceUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewRestartInstanceForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewRestartInstanceNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewRestartInstanceUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewRestartInstanceAccepted creates a RestartInstanceAccepted with default headers values
func NewRestartInstanceAccepted() *RestartInstanceAccepted {
	return &RestartInstanceAccepted{}
}

/* RestartInstanceAccepted describes a response with status code 202, with default header values.

RestartInstanceAccepted restart instance accepted
*/
type RestartInstanceAccepted struct {
}

func (o *RestartInstanceAccepted) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/restart][%d] restartInstanceAccepted ", 202)
}

func (o *RestartInstanceAccepted) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewRestartInstanceUnauthorized creates a RestartInstanceUnauthorized with default headers values
func NewRestartInstanceUnauthorized() *RestartInstanceUnauthorized {
	return &RestartInstanceUnauthorized{}
}

/* RestartInstanceUnauthorized describes a response with status code 401, with default header values.

RestartInstanceUnauthorized restart instance unauthorized
*/
type RestartInstanceUnauthorized struct {
}

func (o *RestartInstanceUnauthorized) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/restart][%d] restartInstanceUnauthorized ", 401)
}

func (o *RestartInstanceUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewRestartInstanceForbidden creates a RestartInstanceForbidden with default headers values
func NewRestartInstanceForbidden() *RestartInstanceForbidden {
	return &RestartInstanceForbidden{}
}

/* RestartInstanceForbidden describes a response with status code 403, with default header values.

RestartInstanceForbidden restart instance forbidden
*/
type RestartInstanceForbidden struct {
}

func (o *RestartInstanceForbidden) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/restart][%d] restartInstanceForbidden ", 403)
}

func (o *RestartInstanceForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewRestartInstanceNotFound creates a RestartInstanceNotFound with default headers values
func NewRestartInstanceNotFound() *RestartInstanceNotFound {
	return &RestartInstanceNotFound{}
}

/* RestartInstanceNotFound describes a response with status code 404, with default header values.

RestartInstanceNotFound restart instance not found
*/
type RestartInstanceNotFound struct {
}

func (o *RestartInstanceNotFound) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/restart][%d] restartInstanceNotFound ", 404)
}

func (o *RestartInstanceNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewRestartInstanceUnsupportedMediaType creates a RestartInstanceUnsupportedMediaType with default headers values
func NewRestartInstanceUnsupportedMediaType() *RestartInstanceUnsupportedMediaType {
	return &RestartInstanceUnsupportedMediaType{}
}

/* RestartInstanceUnsupportedMediaType describes a response with status code 415, with default header values.

RestartInstanceUnsupportedMediaType restart instance unsupported media type
*/
type RestartInstanceUnsupportedMediaType struct {
}

func (o *RestartInstanceUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/restart][%d] restartInstanceUnsupportedMediaType ", 415)
}

func (o *RestartInstanceUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
