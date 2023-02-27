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

// FindByIDReader is a Reader for the FindByID structure.
type FindByIDReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *FindByIDReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewFindByIDOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewFindByIDUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewFindByIDForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewFindByIDNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewFindByIDUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewFindByIDOK creates a FindByIDOK with default headers values
func NewFindByIDOK() *FindByIDOK {
	return &FindByIDOK{}
}

/*
FindByIDOK describes a response with status code 200, with default header values.

Instance
*/
type FindByIDOK struct {
	Payload *models.Instance
}

// IsSuccess returns true when this find by Id o k response has a 2xx status code
func (o *FindByIDOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this find by Id o k response has a 3xx status code
func (o *FindByIDOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this find by Id o k response has a 4xx status code
func (o *FindByIDOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this find by Id o k response has a 5xx status code
func (o *FindByIDOK) IsServerError() bool {
	return false
}

// IsCode returns true when this find by Id o k response a status code equal to that given
func (o *FindByIDOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the find by Id o k response
func (o *FindByIDOK) Code() int {
	return 200
}

func (o *FindByIDOK) Error() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdOK  %+v", 200, o.Payload)
}

func (o *FindByIDOK) String() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdOK  %+v", 200, o.Payload)
}

func (o *FindByIDOK) GetPayload() *models.Instance {
	return o.Payload
}

func (o *FindByIDOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Instance)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewFindByIDUnauthorized creates a FindByIDUnauthorized with default headers values
func NewFindByIDUnauthorized() *FindByIDUnauthorized {
	return &FindByIDUnauthorized{}
}

/*
FindByIDUnauthorized describes a response with status code 401, with default header values.

FindByIDUnauthorized find by Id unauthorized
*/
type FindByIDUnauthorized struct {
}

// IsSuccess returns true when this find by Id unauthorized response has a 2xx status code
func (o *FindByIDUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this find by Id unauthorized response has a 3xx status code
func (o *FindByIDUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this find by Id unauthorized response has a 4xx status code
func (o *FindByIDUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this find by Id unauthorized response has a 5xx status code
func (o *FindByIDUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this find by Id unauthorized response a status code equal to that given
func (o *FindByIDUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the find by Id unauthorized response
func (o *FindByIDUnauthorized) Code() int {
	return 401
}

func (o *FindByIDUnauthorized) Error() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdUnauthorized ", 401)
}

func (o *FindByIDUnauthorized) String() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdUnauthorized ", 401)
}

func (o *FindByIDUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewFindByIDForbidden creates a FindByIDForbidden with default headers values
func NewFindByIDForbidden() *FindByIDForbidden {
	return &FindByIDForbidden{}
}

/*
FindByIDForbidden describes a response with status code 403, with default header values.

FindByIDForbidden find by Id forbidden
*/
type FindByIDForbidden struct {
}

// IsSuccess returns true when this find by Id forbidden response has a 2xx status code
func (o *FindByIDForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this find by Id forbidden response has a 3xx status code
func (o *FindByIDForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this find by Id forbidden response has a 4xx status code
func (o *FindByIDForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this find by Id forbidden response has a 5xx status code
func (o *FindByIDForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this find by Id forbidden response a status code equal to that given
func (o *FindByIDForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the find by Id forbidden response
func (o *FindByIDForbidden) Code() int {
	return 403
}

func (o *FindByIDForbidden) Error() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdForbidden ", 403)
}

func (o *FindByIDForbidden) String() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdForbidden ", 403)
}

func (o *FindByIDForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewFindByIDNotFound creates a FindByIDNotFound with default headers values
func NewFindByIDNotFound() *FindByIDNotFound {
	return &FindByIDNotFound{}
}

/*
FindByIDNotFound describes a response with status code 404, with default header values.

FindByIDNotFound find by Id not found
*/
type FindByIDNotFound struct {
}

// IsSuccess returns true when this find by Id not found response has a 2xx status code
func (o *FindByIDNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this find by Id not found response has a 3xx status code
func (o *FindByIDNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this find by Id not found response has a 4xx status code
func (o *FindByIDNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this find by Id not found response has a 5xx status code
func (o *FindByIDNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this find by Id not found response a status code equal to that given
func (o *FindByIDNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the find by Id not found response
func (o *FindByIDNotFound) Code() int {
	return 404
}

func (o *FindByIDNotFound) Error() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdNotFound ", 404)
}

func (o *FindByIDNotFound) String() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdNotFound ", 404)
}

func (o *FindByIDNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewFindByIDUnsupportedMediaType creates a FindByIDUnsupportedMediaType with default headers values
func NewFindByIDUnsupportedMediaType() *FindByIDUnsupportedMediaType {
	return &FindByIDUnsupportedMediaType{}
}

/*
FindByIDUnsupportedMediaType describes a response with status code 415, with default header values.

FindByIDUnsupportedMediaType find by Id unsupported media type
*/
type FindByIDUnsupportedMediaType struct {
}

// IsSuccess returns true when this find by Id unsupported media type response has a 2xx status code
func (o *FindByIDUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this find by Id unsupported media type response has a 3xx status code
func (o *FindByIDUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this find by Id unsupported media type response has a 4xx status code
func (o *FindByIDUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this find by Id unsupported media type response has a 5xx status code
func (o *FindByIDUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this find by Id unsupported media type response a status code equal to that given
func (o *FindByIDUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the find by Id unsupported media type response
func (o *FindByIDUnsupportedMediaType) Code() int {
	return 415
}

func (o *FindByIDUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdUnsupportedMediaType ", 415)
}

func (o *FindByIDUnsupportedMediaType) String() string {
	return fmt.Sprintf("[GET /instances/{id}][%d] findByIdUnsupportedMediaType ", 415)
}

func (o *FindByIDUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
