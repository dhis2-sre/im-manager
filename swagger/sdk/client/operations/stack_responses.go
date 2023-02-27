// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// StackReader is a Reader for the Stack structure.
type StackReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StackReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewStackOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewStackUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewStackForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewStackNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewStackUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewStackOK creates a StackOK with default headers values
func NewStackOK() *StackOK {
	return &StackOK{}
}

/*
StackOK describes a response with status code 200, with default header values.

StackOK stack o k
*/
type StackOK struct {
}

// IsSuccess returns true when this stack o k response has a 2xx status code
func (o *StackOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this stack o k response has a 3xx status code
func (o *StackOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this stack o k response has a 4xx status code
func (o *StackOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this stack o k response has a 5xx status code
func (o *StackOK) IsServerError() bool {
	return false
}

// IsCode returns true when this stack o k response a status code equal to that given
func (o *StackOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the stack o k response
func (o *StackOK) Code() int {
	return 200
}

func (o *StackOK) Error() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackOK ", 200)
}

func (o *StackOK) String() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackOK ", 200)
}

func (o *StackOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewStackUnauthorized creates a StackUnauthorized with default headers values
func NewStackUnauthorized() *StackUnauthorized {
	return &StackUnauthorized{}
}

/*
StackUnauthorized describes a response with status code 401, with default header values.

StackUnauthorized stack unauthorized
*/
type StackUnauthorized struct {
}

// IsSuccess returns true when this stack unauthorized response has a 2xx status code
func (o *StackUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this stack unauthorized response has a 3xx status code
func (o *StackUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this stack unauthorized response has a 4xx status code
func (o *StackUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this stack unauthorized response has a 5xx status code
func (o *StackUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this stack unauthorized response a status code equal to that given
func (o *StackUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the stack unauthorized response
func (o *StackUnauthorized) Code() int {
	return 401
}

func (o *StackUnauthorized) Error() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackUnauthorized ", 401)
}

func (o *StackUnauthorized) String() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackUnauthorized ", 401)
}

func (o *StackUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewStackForbidden creates a StackForbidden with default headers values
func NewStackForbidden() *StackForbidden {
	return &StackForbidden{}
}

/*
StackForbidden describes a response with status code 403, with default header values.

StackForbidden stack forbidden
*/
type StackForbidden struct {
}

// IsSuccess returns true when this stack forbidden response has a 2xx status code
func (o *StackForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this stack forbidden response has a 3xx status code
func (o *StackForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this stack forbidden response has a 4xx status code
func (o *StackForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this stack forbidden response has a 5xx status code
func (o *StackForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this stack forbidden response a status code equal to that given
func (o *StackForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the stack forbidden response
func (o *StackForbidden) Code() int {
	return 403
}

func (o *StackForbidden) Error() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackForbidden ", 403)
}

func (o *StackForbidden) String() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackForbidden ", 403)
}

func (o *StackForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewStackNotFound creates a StackNotFound with default headers values
func NewStackNotFound() *StackNotFound {
	return &StackNotFound{}
}

/*
StackNotFound describes a response with status code 404, with default header values.

StackNotFound stack not found
*/
type StackNotFound struct {
}

// IsSuccess returns true when this stack not found response has a 2xx status code
func (o *StackNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this stack not found response has a 3xx status code
func (o *StackNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this stack not found response has a 4xx status code
func (o *StackNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this stack not found response has a 5xx status code
func (o *StackNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this stack not found response a status code equal to that given
func (o *StackNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the stack not found response
func (o *StackNotFound) Code() int {
	return 404
}

func (o *StackNotFound) Error() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackNotFound ", 404)
}

func (o *StackNotFound) String() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackNotFound ", 404)
}

func (o *StackNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewStackUnsupportedMediaType creates a StackUnsupportedMediaType with default headers values
func NewStackUnsupportedMediaType() *StackUnsupportedMediaType {
	return &StackUnsupportedMediaType{}
}

/*
StackUnsupportedMediaType describes a response with status code 415, with default header values.

StackUnsupportedMediaType stack unsupported media type
*/
type StackUnsupportedMediaType struct {
}

// IsSuccess returns true when this stack unsupported media type response has a 2xx status code
func (o *StackUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this stack unsupported media type response has a 3xx status code
func (o *StackUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this stack unsupported media type response has a 4xx status code
func (o *StackUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this stack unsupported media type response has a 5xx status code
func (o *StackUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this stack unsupported media type response a status code equal to that given
func (o *StackUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the stack unsupported media type response
func (o *StackUnsupportedMediaType) Code() int {
	return 415
}

func (o *StackUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackUnsupportedMediaType ", 415)
}

func (o *StackUnsupportedMediaType) String() string {
	return fmt.Sprintf("[GET /stacks/{name}][%d] stackUnsupportedMediaType ", 415)
}

func (o *StackUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
