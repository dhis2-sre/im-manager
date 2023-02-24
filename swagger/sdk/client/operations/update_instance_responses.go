// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/dhis2-sre/im-manager/swagger/sdk/models"
)

// UpdateInstanceReader is a Reader for the UpdateInstance structure.
type UpdateInstanceReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *UpdateInstanceReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 204:
		result := NewUpdateInstanceNoContent()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewUpdateInstanceUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewUpdateInstanceForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewUpdateInstanceNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewUpdateInstanceUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewUpdateInstanceNoContent creates a UpdateInstanceNoContent with default headers values
func NewUpdateInstanceNoContent() *UpdateInstanceNoContent {
	return &UpdateInstanceNoContent{}
}

/*
UpdateInstanceNoContent describes a response with status code 204, with default header values.

Instance
*/
type UpdateInstanceNoContent struct {
	Payload *models.Instance
}

// IsSuccess returns true when this update instance no content response has a 2xx status code
func (o *UpdateInstanceNoContent) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this update instance no content response has a 3xx status code
func (o *UpdateInstanceNoContent) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update instance no content response has a 4xx status code
func (o *UpdateInstanceNoContent) IsClientError() bool {
	return false
}

// IsServerError returns true when this update instance no content response has a 5xx status code
func (o *UpdateInstanceNoContent) IsServerError() bool {
	return false
}

// IsCode returns true when this update instance no content response a status code equal to that given
func (o *UpdateInstanceNoContent) IsCode(code int) bool {
	return code == 204
}

// Code gets the status code for the update instance no content response
func (o *UpdateInstanceNoContent) Code() int {
	return 204
}

func (o *UpdateInstanceNoContent) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceNoContent  %+v", 204, o.Payload)
}

func (o *UpdateInstanceNoContent) String() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceNoContent  %+v", 204, o.Payload)
}

func (o *UpdateInstanceNoContent) GetPayload() *models.Instance {
	return o.Payload
}

func (o *UpdateInstanceNoContent) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Instance)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewUpdateInstanceUnauthorized creates a UpdateInstanceUnauthorized with default headers values
func NewUpdateInstanceUnauthorized() *UpdateInstanceUnauthorized {
	return &UpdateInstanceUnauthorized{}
}

/*
UpdateInstanceUnauthorized describes a response with status code 401, with default header values.

UpdateInstanceUnauthorized update instance unauthorized
*/
type UpdateInstanceUnauthorized struct {
}

// IsSuccess returns true when this update instance unauthorized response has a 2xx status code
func (o *UpdateInstanceUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this update instance unauthorized response has a 3xx status code
func (o *UpdateInstanceUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update instance unauthorized response has a 4xx status code
func (o *UpdateInstanceUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this update instance unauthorized response has a 5xx status code
func (o *UpdateInstanceUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this update instance unauthorized response a status code equal to that given
func (o *UpdateInstanceUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the update instance unauthorized response
func (o *UpdateInstanceUnauthorized) Code() int {
	return 401
}

func (o *UpdateInstanceUnauthorized) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceUnauthorized ", 401)
}

func (o *UpdateInstanceUnauthorized) String() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceUnauthorized ", 401)
}

func (o *UpdateInstanceUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUpdateInstanceForbidden creates a UpdateInstanceForbidden with default headers values
func NewUpdateInstanceForbidden() *UpdateInstanceForbidden {
	return &UpdateInstanceForbidden{}
}

/*
UpdateInstanceForbidden describes a response with status code 403, with default header values.

UpdateInstanceForbidden update instance forbidden
*/
type UpdateInstanceForbidden struct {
}

// IsSuccess returns true when this update instance forbidden response has a 2xx status code
func (o *UpdateInstanceForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this update instance forbidden response has a 3xx status code
func (o *UpdateInstanceForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update instance forbidden response has a 4xx status code
func (o *UpdateInstanceForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this update instance forbidden response has a 5xx status code
func (o *UpdateInstanceForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this update instance forbidden response a status code equal to that given
func (o *UpdateInstanceForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the update instance forbidden response
func (o *UpdateInstanceForbidden) Code() int {
	return 403
}

func (o *UpdateInstanceForbidden) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceForbidden ", 403)
}

func (o *UpdateInstanceForbidden) String() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceForbidden ", 403)
}

func (o *UpdateInstanceForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUpdateInstanceNotFound creates a UpdateInstanceNotFound with default headers values
func NewUpdateInstanceNotFound() *UpdateInstanceNotFound {
	return &UpdateInstanceNotFound{}
}

/*
UpdateInstanceNotFound describes a response with status code 404, with default header values.

UpdateInstanceNotFound update instance not found
*/
type UpdateInstanceNotFound struct {
}

// IsSuccess returns true when this update instance not found response has a 2xx status code
func (o *UpdateInstanceNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this update instance not found response has a 3xx status code
func (o *UpdateInstanceNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update instance not found response has a 4xx status code
func (o *UpdateInstanceNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this update instance not found response has a 5xx status code
func (o *UpdateInstanceNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this update instance not found response a status code equal to that given
func (o *UpdateInstanceNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the update instance not found response
func (o *UpdateInstanceNotFound) Code() int {
	return 404
}

func (o *UpdateInstanceNotFound) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceNotFound ", 404)
}

func (o *UpdateInstanceNotFound) String() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceNotFound ", 404)
}

func (o *UpdateInstanceNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUpdateInstanceUnsupportedMediaType creates a UpdateInstanceUnsupportedMediaType with default headers values
func NewUpdateInstanceUnsupportedMediaType() *UpdateInstanceUnsupportedMediaType {
	return &UpdateInstanceUnsupportedMediaType{}
}

/*
UpdateInstanceUnsupportedMediaType describes a response with status code 415, with default header values.

UpdateInstanceUnsupportedMediaType update instance unsupported media type
*/
type UpdateInstanceUnsupportedMediaType struct {
}

// IsSuccess returns true when this update instance unsupported media type response has a 2xx status code
func (o *UpdateInstanceUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this update instance unsupported media type response has a 3xx status code
func (o *UpdateInstanceUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update instance unsupported media type response has a 4xx status code
func (o *UpdateInstanceUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this update instance unsupported media type response has a 5xx status code
func (o *UpdateInstanceUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this update instance unsupported media type response a status code equal to that given
func (o *UpdateInstanceUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the update instance unsupported media type response
func (o *UpdateInstanceUnsupportedMediaType) Code() int {
	return 415
}

func (o *UpdateInstanceUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceUnsupportedMediaType ", 415)
}

func (o *UpdateInstanceUnsupportedMediaType) String() string {
	return fmt.Sprintf("[PUT /instances/{id}][%d] updateInstanceUnsupportedMediaType ", 415)
}

func (o *UpdateInstanceUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
