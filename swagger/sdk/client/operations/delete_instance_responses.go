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

// IsSuccess returns true when this delete instance accepted response has a 2xx status code
func (o *DeleteInstanceAccepted) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this delete instance accepted response has a 3xx status code
func (o *DeleteInstanceAccepted) IsRedirect() bool {
	return false
}

// IsClientError returns true when this delete instance accepted response has a 4xx status code
func (o *DeleteInstanceAccepted) IsClientError() bool {
	return false
}

// IsServerError returns true when this delete instance accepted response has a 5xx status code
func (o *DeleteInstanceAccepted) IsServerError() bool {
	return false
}

// IsCode returns true when this delete instance accepted response a status code equal to that given
func (o *DeleteInstanceAccepted) IsCode(code int) bool {
	return code == 202
}

// Code gets the status code for the delete instance accepted response
func (o *DeleteInstanceAccepted) Code() int {
	return 202
}

func (o *DeleteInstanceAccepted) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceAccepted ", 202)
}

func (o *DeleteInstanceAccepted) String() string {
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

// IsSuccess returns true when this delete instance unauthorized response has a 2xx status code
func (o *DeleteInstanceUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this delete instance unauthorized response has a 3xx status code
func (o *DeleteInstanceUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this delete instance unauthorized response has a 4xx status code
func (o *DeleteInstanceUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this delete instance unauthorized response has a 5xx status code
func (o *DeleteInstanceUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this delete instance unauthorized response a status code equal to that given
func (o *DeleteInstanceUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the delete instance unauthorized response
func (o *DeleteInstanceUnauthorized) Code() int {
	return 401
}

func (o *DeleteInstanceUnauthorized) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceUnauthorized ", 401)
}

func (o *DeleteInstanceUnauthorized) String() string {
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

// IsSuccess returns true when this delete instance forbidden response has a 2xx status code
func (o *DeleteInstanceForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this delete instance forbidden response has a 3xx status code
func (o *DeleteInstanceForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this delete instance forbidden response has a 4xx status code
func (o *DeleteInstanceForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this delete instance forbidden response has a 5xx status code
func (o *DeleteInstanceForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this delete instance forbidden response a status code equal to that given
func (o *DeleteInstanceForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the delete instance forbidden response
func (o *DeleteInstanceForbidden) Code() int {
	return 403
}

func (o *DeleteInstanceForbidden) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceForbidden ", 403)
}

func (o *DeleteInstanceForbidden) String() string {
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

// IsSuccess returns true when this delete instance not found response has a 2xx status code
func (o *DeleteInstanceNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this delete instance not found response has a 3xx status code
func (o *DeleteInstanceNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this delete instance not found response has a 4xx status code
func (o *DeleteInstanceNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this delete instance not found response has a 5xx status code
func (o *DeleteInstanceNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this delete instance not found response a status code equal to that given
func (o *DeleteInstanceNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the delete instance not found response
func (o *DeleteInstanceNotFound) Code() int {
	return 404
}

func (o *DeleteInstanceNotFound) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceNotFound ", 404)
}

func (o *DeleteInstanceNotFound) String() string {
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

// IsSuccess returns true when this delete instance unsupported media type response has a 2xx status code
func (o *DeleteInstanceUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this delete instance unsupported media type response has a 3xx status code
func (o *DeleteInstanceUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this delete instance unsupported media type response has a 4xx status code
func (o *DeleteInstanceUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this delete instance unsupported media type response has a 5xx status code
func (o *DeleteInstanceUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this delete instance unsupported media type response a status code equal to that given
func (o *DeleteInstanceUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the delete instance unsupported media type response
func (o *DeleteInstanceUnsupportedMediaType) Code() int {
	return 415
}

func (o *DeleteInstanceUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceUnsupportedMediaType ", 415)
}

func (o *DeleteInstanceUnsupportedMediaType) String() string {
	return fmt.Sprintf("[DELETE /instances/{id}][%d] deleteInstanceUnsupportedMediaType ", 415)
}

func (o *DeleteInstanceUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
