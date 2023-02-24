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

// ListInstancesReader is a Reader for the ListInstances structure.
type ListInstancesReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ListInstancesReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewListInstancesOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewListInstancesUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewListInstancesForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewListInstancesUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewListInstancesOK creates a ListInstancesOK with default headers values
func NewListInstancesOK() *ListInstancesOK {
	return &ListInstancesOK{}
}

/*
ListInstancesOK describes a response with status code 200, with default header values.

ListInstancesOK list instances o k
*/
type ListInstancesOK struct {
	Payload []*models.GroupWithInstances
}

// IsSuccess returns true when this list instances o k response has a 2xx status code
func (o *ListInstancesOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this list instances o k response has a 3xx status code
func (o *ListInstancesOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this list instances o k response has a 4xx status code
func (o *ListInstancesOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this list instances o k response has a 5xx status code
func (o *ListInstancesOK) IsServerError() bool {
	return false
}

// IsCode returns true when this list instances o k response a status code equal to that given
func (o *ListInstancesOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the list instances o k response
func (o *ListInstancesOK) Code() int {
	return 200
}

func (o *ListInstancesOK) Error() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesOK  %+v", 200, o.Payload)
}

func (o *ListInstancesOK) String() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesOK  %+v", 200, o.Payload)
}

func (o *ListInstancesOK) GetPayload() []*models.GroupWithInstances {
	return o.Payload
}

func (o *ListInstancesOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewListInstancesUnauthorized creates a ListInstancesUnauthorized with default headers values
func NewListInstancesUnauthorized() *ListInstancesUnauthorized {
	return &ListInstancesUnauthorized{}
}

/*
ListInstancesUnauthorized describes a response with status code 401, with default header values.

ListInstancesUnauthorized list instances unauthorized
*/
type ListInstancesUnauthorized struct {
}

// IsSuccess returns true when this list instances unauthorized response has a 2xx status code
func (o *ListInstancesUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this list instances unauthorized response has a 3xx status code
func (o *ListInstancesUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this list instances unauthorized response has a 4xx status code
func (o *ListInstancesUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this list instances unauthorized response has a 5xx status code
func (o *ListInstancesUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this list instances unauthorized response a status code equal to that given
func (o *ListInstancesUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the list instances unauthorized response
func (o *ListInstancesUnauthorized) Code() int {
	return 401
}

func (o *ListInstancesUnauthorized) Error() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesUnauthorized ", 401)
}

func (o *ListInstancesUnauthorized) String() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesUnauthorized ", 401)
}

func (o *ListInstancesUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewListInstancesForbidden creates a ListInstancesForbidden with default headers values
func NewListInstancesForbidden() *ListInstancesForbidden {
	return &ListInstancesForbidden{}
}

/*
ListInstancesForbidden describes a response with status code 403, with default header values.

ListInstancesForbidden list instances forbidden
*/
type ListInstancesForbidden struct {
}

// IsSuccess returns true when this list instances forbidden response has a 2xx status code
func (o *ListInstancesForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this list instances forbidden response has a 3xx status code
func (o *ListInstancesForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this list instances forbidden response has a 4xx status code
func (o *ListInstancesForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this list instances forbidden response has a 5xx status code
func (o *ListInstancesForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this list instances forbidden response a status code equal to that given
func (o *ListInstancesForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the list instances forbidden response
func (o *ListInstancesForbidden) Code() int {
	return 403
}

func (o *ListInstancesForbidden) Error() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesForbidden ", 403)
}

func (o *ListInstancesForbidden) String() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesForbidden ", 403)
}

func (o *ListInstancesForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewListInstancesUnsupportedMediaType creates a ListInstancesUnsupportedMediaType with default headers values
func NewListInstancesUnsupportedMediaType() *ListInstancesUnsupportedMediaType {
	return &ListInstancesUnsupportedMediaType{}
}

/*
ListInstancesUnsupportedMediaType describes a response with status code 415, with default header values.

ListInstancesUnsupportedMediaType list instances unsupported media type
*/
type ListInstancesUnsupportedMediaType struct {
}

// IsSuccess returns true when this list instances unsupported media type response has a 2xx status code
func (o *ListInstancesUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this list instances unsupported media type response has a 3xx status code
func (o *ListInstancesUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this list instances unsupported media type response has a 4xx status code
func (o *ListInstancesUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this list instances unsupported media type response has a 5xx status code
func (o *ListInstancesUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this list instances unsupported media type response a status code equal to that given
func (o *ListInstancesUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the list instances unsupported media type response
func (o *ListInstancesUnsupportedMediaType) Code() int {
	return 415
}

func (o *ListInstancesUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesUnsupportedMediaType ", 415)
}

func (o *ListInstancesUnsupportedMediaType) String() string {
	return fmt.Sprintf("[GET /instances][%d] listInstancesUnsupportedMediaType ", 415)
}

func (o *ListInstancesUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
