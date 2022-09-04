// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// DeleteInstanceReader is a Reader for the DeleteInstance structure.
type DeleteInstanceReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *DeleteInstanceReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 202:
		result := NewDeleteInstanceAccepted()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewDeleteInstanceUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewDeleteInstanceForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewDeleteInstanceNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewDeleteInstanceUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewDeleteInstanceAccepted creates a DeleteInstanceAccepted with default headers values
func NewDeleteInstanceAccepted() *DeleteInstanceAccepted {
	return &DeleteInstanceAccepted{}
}

/* DeleteInstanceAccepted describes a response with status code 202, with default header values.

DeleteInstanceAccepted delete instance accepted
*/
type DeleteInstanceAccepted struct {
}

func (o *DeleteInstanceAccepted) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceAccepted ", 202)
}

func (o *DeleteInstanceAccepted) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewDeleteInstanceUnauthorized creates a DeleteInstanceUnauthorized with default headers values
func NewDeleteInstanceUnauthorized() *DeleteInstanceUnauthorized {
	return &DeleteInstanceUnauthorized{}
}

/* DeleteInstanceUnauthorized describes a response with status code 401, with default header values.

DeleteInstanceUnauthorized delete instance unauthorized
*/
type DeleteInstanceUnauthorized struct {
}

func (o *DeleteInstanceUnauthorized) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceUnauthorized ", 401)
}

func (o *DeleteInstanceUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewDeleteInstanceForbidden creates a DeleteInstanceForbidden with default headers values
func NewDeleteInstanceForbidden() *DeleteInstanceForbidden {
	return &DeleteInstanceForbidden{}
}

/* DeleteInstanceForbidden describes a response with status code 403, with default header values.

DeleteInstanceForbidden delete instance forbidden
*/
type DeleteInstanceForbidden struct {
}

func (o *DeleteInstanceForbidden) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceForbidden ", 403)
}

func (o *DeleteInstanceForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewDeleteInstanceNotFound creates a DeleteInstanceNotFound with default headers values
func NewDeleteInstanceNotFound() *DeleteInstanceNotFound {
	return &DeleteInstanceNotFound{}
}

/* DeleteInstanceNotFound describes a response with status code 404, with default header values.

DeleteInstanceNotFound delete instance not found
*/
type DeleteInstanceNotFound struct {
}

func (o *DeleteInstanceNotFound) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceNotFound ", 404)
}

func (o *DeleteInstanceNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewDeleteInstanceUnsupportedMediaType creates a DeleteInstanceUnsupportedMediaType with default headers values
func NewDeleteInstanceUnsupportedMediaType() *DeleteInstanceUnsupportedMediaType {
	return &DeleteInstanceUnsupportedMediaType{}
}

/* DeleteInstanceUnsupportedMediaType describes a response with status code 415, with default header values.

DeleteInstanceUnsupportedMediaType delete instance unsupported media type
*/
type DeleteInstanceUnsupportedMediaType struct {
}

func (o *DeleteInstanceUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceUnsupportedMediaType ", 415)
}

func (o *DeleteInstanceUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
