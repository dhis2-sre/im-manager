// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// PauseInstanceReader is a Reader for the PauseInstance structure.
type PauseInstanceReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *PauseInstanceReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 202:
		result := NewPauseInstanceAccepted()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewPauseInstanceUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewPauseInstanceForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewPauseInstanceNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewPauseInstanceUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewPauseInstanceAccepted creates a PauseInstanceAccepted with default headers values
func NewPauseInstanceAccepted() *PauseInstanceAccepted {
	return &PauseInstanceAccepted{}
}

/* PauseInstanceAccepted describes a response with status code 202, with default header values.

PauseInstanceAccepted pause instance accepted
*/
type PauseInstanceAccepted struct {
}

// IsSuccess returns true when this pause instance accepted response has a 2xx status code
func (o *PauseInstanceAccepted) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this pause instance accepted response has a 3xx status code
func (o *PauseInstanceAccepted) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pause instance accepted response has a 4xx status code
func (o *PauseInstanceAccepted) IsClientError() bool {
	return false
}

// IsServerError returns true when this pause instance accepted response has a 5xx status code
func (o *PauseInstanceAccepted) IsServerError() bool {
	return false
}

// IsCode returns true when this pause instance accepted response a status code equal to that given
func (o *PauseInstanceAccepted) IsCode(code int) bool {
	return code == 202
}

// Code gets the status code for the pause instance accepted response
func (o *PauseInstanceAccepted) Code() int {
	return 202
}

func (o *PauseInstanceAccepted) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceAccepted ", 202)
}

func (o *PauseInstanceAccepted) String() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceAccepted ", 202)
}

func (o *PauseInstanceAccepted) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPauseInstanceUnauthorized creates a PauseInstanceUnauthorized with default headers values
func NewPauseInstanceUnauthorized() *PauseInstanceUnauthorized {
	return &PauseInstanceUnauthorized{}
}

/* PauseInstanceUnauthorized describes a response with status code 401, with default header values.

PauseInstanceUnauthorized pause instance unauthorized
*/
type PauseInstanceUnauthorized struct {
}

// IsSuccess returns true when this pause instance unauthorized response has a 2xx status code
func (o *PauseInstanceUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pause instance unauthorized response has a 3xx status code
func (o *PauseInstanceUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pause instance unauthorized response has a 4xx status code
func (o *PauseInstanceUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this pause instance unauthorized response has a 5xx status code
func (o *PauseInstanceUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this pause instance unauthorized response a status code equal to that given
func (o *PauseInstanceUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the pause instance unauthorized response
func (o *PauseInstanceUnauthorized) Code() int {
	return 401
}

func (o *PauseInstanceUnauthorized) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceUnauthorized ", 401)
}

func (o *PauseInstanceUnauthorized) String() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceUnauthorized ", 401)
}

func (o *PauseInstanceUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPauseInstanceForbidden creates a PauseInstanceForbidden with default headers values
func NewPauseInstanceForbidden() *PauseInstanceForbidden {
	return &PauseInstanceForbidden{}
}

/* PauseInstanceForbidden describes a response with status code 403, with default header values.

PauseInstanceForbidden pause instance forbidden
*/
type PauseInstanceForbidden struct {
}

// IsSuccess returns true when this pause instance forbidden response has a 2xx status code
func (o *PauseInstanceForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pause instance forbidden response has a 3xx status code
func (o *PauseInstanceForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pause instance forbidden response has a 4xx status code
func (o *PauseInstanceForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this pause instance forbidden response has a 5xx status code
func (o *PauseInstanceForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this pause instance forbidden response a status code equal to that given
func (o *PauseInstanceForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the pause instance forbidden response
func (o *PauseInstanceForbidden) Code() int {
	return 403
}

func (o *PauseInstanceForbidden) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceForbidden ", 403)
}

func (o *PauseInstanceForbidden) String() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceForbidden ", 403)
}

func (o *PauseInstanceForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPauseInstanceNotFound creates a PauseInstanceNotFound with default headers values
func NewPauseInstanceNotFound() *PauseInstanceNotFound {
	return &PauseInstanceNotFound{}
}

/* PauseInstanceNotFound describes a response with status code 404, with default header values.

PauseInstanceNotFound pause instance not found
*/
type PauseInstanceNotFound struct {
}

// IsSuccess returns true when this pause instance not found response has a 2xx status code
func (o *PauseInstanceNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pause instance not found response has a 3xx status code
func (o *PauseInstanceNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pause instance not found response has a 4xx status code
func (o *PauseInstanceNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this pause instance not found response has a 5xx status code
func (o *PauseInstanceNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this pause instance not found response a status code equal to that given
func (o *PauseInstanceNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the pause instance not found response
func (o *PauseInstanceNotFound) Code() int {
	return 404
}

func (o *PauseInstanceNotFound) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceNotFound ", 404)
}

func (o *PauseInstanceNotFound) String() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceNotFound ", 404)
}

func (o *PauseInstanceNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPauseInstanceUnsupportedMediaType creates a PauseInstanceUnsupportedMediaType with default headers values
func NewPauseInstanceUnsupportedMediaType() *PauseInstanceUnsupportedMediaType {
	return &PauseInstanceUnsupportedMediaType{}
}

/* PauseInstanceUnsupportedMediaType describes a response with status code 415, with default header values.

PauseInstanceUnsupportedMediaType pause instance unsupported media type
*/
type PauseInstanceUnsupportedMediaType struct {
}

// IsSuccess returns true when this pause instance unsupported media type response has a 2xx status code
func (o *PauseInstanceUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pause instance unsupported media type response has a 3xx status code
func (o *PauseInstanceUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pause instance unsupported media type response has a 4xx status code
func (o *PauseInstanceUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this pause instance unsupported media type response has a 5xx status code
func (o *PauseInstanceUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this pause instance unsupported media type response a status code equal to that given
func (o *PauseInstanceUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the pause instance unsupported media type response
func (o *PauseInstanceUnsupportedMediaType) Code() int {
	return 415
}

func (o *PauseInstanceUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceUnsupportedMediaType ", 415)
}

func (o *PauseInstanceUnsupportedMediaType) String() string {
	return fmt.Sprintf("[PUT /instances/{id}/pause][%d] pauseInstanceUnsupportedMediaType ", 415)
}

func (o *PauseInstanceUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
