// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// PostIntegrationReader is a Reader for the PostIntegration structure.
type PostIntegrationReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *PostIntegrationReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewPostIntegrationOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewPostIntegrationUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewPostIntegrationForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewPostIntegrationUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewPostIntegrationOK creates a PostIntegrationOK with default headers values
func NewPostIntegrationOK() *PostIntegrationOK {
	return &PostIntegrationOK{}
}

/* PostIntegrationOK describes a response with status code 200, with default header values.

PostIntegrationOK post integration o k
*/
type PostIntegrationOK struct {
	Payload interface{}
}

// IsSuccess returns true when this post integration o k response has a 2xx status code
func (o *PostIntegrationOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this post integration o k response has a 3xx status code
func (o *PostIntegrationOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this post integration o k response has a 4xx status code
func (o *PostIntegrationOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this post integration o k response has a 5xx status code
func (o *PostIntegrationOK) IsServerError() bool {
	return false
}

// IsCode returns true when this post integration o k response a status code equal to that given
func (o *PostIntegrationOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the post integration o k response
func (o *PostIntegrationOK) Code() int {
	return 200
}

func (o *PostIntegrationOK) Error() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationOK  %+v", 200, o.Payload)
}

func (o *PostIntegrationOK) String() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationOK  %+v", 200, o.Payload)
}

func (o *PostIntegrationOK) GetPayload() interface{} {
	return o.Payload
}

func (o *PostIntegrationOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPostIntegrationUnauthorized creates a PostIntegrationUnauthorized with default headers values
func NewPostIntegrationUnauthorized() *PostIntegrationUnauthorized {
	return &PostIntegrationUnauthorized{}
}

/* PostIntegrationUnauthorized describes a response with status code 401, with default header values.

PostIntegrationUnauthorized post integration unauthorized
*/
type PostIntegrationUnauthorized struct {
}

// IsSuccess returns true when this post integration unauthorized response has a 2xx status code
func (o *PostIntegrationUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this post integration unauthorized response has a 3xx status code
func (o *PostIntegrationUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this post integration unauthorized response has a 4xx status code
func (o *PostIntegrationUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this post integration unauthorized response has a 5xx status code
func (o *PostIntegrationUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this post integration unauthorized response a status code equal to that given
func (o *PostIntegrationUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the post integration unauthorized response
func (o *PostIntegrationUnauthorized) Code() int {
	return 401
}

func (o *PostIntegrationUnauthorized) Error() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationUnauthorized ", 401)
}

func (o *PostIntegrationUnauthorized) String() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationUnauthorized ", 401)
}

func (o *PostIntegrationUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPostIntegrationForbidden creates a PostIntegrationForbidden with default headers values
func NewPostIntegrationForbidden() *PostIntegrationForbidden {
	return &PostIntegrationForbidden{}
}

/* PostIntegrationForbidden describes a response with status code 403, with default header values.

PostIntegrationForbidden post integration forbidden
*/
type PostIntegrationForbidden struct {
}

// IsSuccess returns true when this post integration forbidden response has a 2xx status code
func (o *PostIntegrationForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this post integration forbidden response has a 3xx status code
func (o *PostIntegrationForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this post integration forbidden response has a 4xx status code
func (o *PostIntegrationForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this post integration forbidden response has a 5xx status code
func (o *PostIntegrationForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this post integration forbidden response a status code equal to that given
func (o *PostIntegrationForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the post integration forbidden response
func (o *PostIntegrationForbidden) Code() int {
	return 403
}

func (o *PostIntegrationForbidden) Error() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationForbidden ", 403)
}

func (o *PostIntegrationForbidden) String() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationForbidden ", 403)
}

func (o *PostIntegrationForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPostIntegrationUnsupportedMediaType creates a PostIntegrationUnsupportedMediaType with default headers values
func NewPostIntegrationUnsupportedMediaType() *PostIntegrationUnsupportedMediaType {
	return &PostIntegrationUnsupportedMediaType{}
}

/* PostIntegrationUnsupportedMediaType describes a response with status code 415, with default header values.

PostIntegrationUnsupportedMediaType post integration unsupported media type
*/
type PostIntegrationUnsupportedMediaType struct {
}

// IsSuccess returns true when this post integration unsupported media type response has a 2xx status code
func (o *PostIntegrationUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this post integration unsupported media type response has a 3xx status code
func (o *PostIntegrationUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this post integration unsupported media type response has a 4xx status code
func (o *PostIntegrationUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this post integration unsupported media type response has a 5xx status code
func (o *PostIntegrationUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this post integration unsupported media type response a status code equal to that given
func (o *PostIntegrationUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the post integration unsupported media type response
func (o *PostIntegrationUnsupportedMediaType) Code() int {
	return 415
}

func (o *PostIntegrationUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationUnsupportedMediaType ", 415)
}

func (o *PostIntegrationUnsupportedMediaType) String() string {
	return fmt.Sprintf("[POST /integrations][%d] postIntegrationUnsupportedMediaType ", 415)
}

func (o *PostIntegrationUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
