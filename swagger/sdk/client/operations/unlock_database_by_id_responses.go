// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// UnlockDatabaseByIDReader is a Reader for the UnlockDatabaseByID structure.
type UnlockDatabaseByIDReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *UnlockDatabaseByIDReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 202:
		result := NewUnlockDatabaseByIDAccepted()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 401:
		result := NewUnlockDatabaseByIDUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewUnlockDatabaseByIDForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewUnlockDatabaseByIDNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 415:
		result := NewUnlockDatabaseByIDUnsupportedMediaType()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewUnlockDatabaseByIDAccepted creates a UnlockDatabaseByIDAccepted with default headers values
func NewUnlockDatabaseByIDAccepted() *UnlockDatabaseByIDAccepted {
	return &UnlockDatabaseByIDAccepted{}
}

/*
UnlockDatabaseByIDAccepted describes a response with status code 202, with default header values.

UnlockDatabaseByIDAccepted unlock database by Id accepted
*/
type UnlockDatabaseByIDAccepted struct {
}

// IsSuccess returns true when this unlock database by Id accepted response has a 2xx status code
func (o *UnlockDatabaseByIDAccepted) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this unlock database by Id accepted response has a 3xx status code
func (o *UnlockDatabaseByIDAccepted) IsRedirect() bool {
	return false
}

// IsClientError returns true when this unlock database by Id accepted response has a 4xx status code
func (o *UnlockDatabaseByIDAccepted) IsClientError() bool {
	return false
}

// IsServerError returns true when this unlock database by Id accepted response has a 5xx status code
func (o *UnlockDatabaseByIDAccepted) IsServerError() bool {
	return false
}

// IsCode returns true when this unlock database by Id accepted response a status code equal to that given
func (o *UnlockDatabaseByIDAccepted) IsCode(code int) bool {
	return code == 202
}

// Code gets the status code for the unlock database by Id accepted response
func (o *UnlockDatabaseByIDAccepted) Code() int {
	return 202
}

func (o *UnlockDatabaseByIDAccepted) Error() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdAccepted ", 202)
}

func (o *UnlockDatabaseByIDAccepted) String() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdAccepted ", 202)
}

func (o *UnlockDatabaseByIDAccepted) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUnlockDatabaseByIDUnauthorized creates a UnlockDatabaseByIDUnauthorized with default headers values
func NewUnlockDatabaseByIDUnauthorized() *UnlockDatabaseByIDUnauthorized {
	return &UnlockDatabaseByIDUnauthorized{}
}

/*
UnlockDatabaseByIDUnauthorized describes a response with status code 401, with default header values.

UnlockDatabaseByIDUnauthorized unlock database by Id unauthorized
*/
type UnlockDatabaseByIDUnauthorized struct {
}

// IsSuccess returns true when this unlock database by Id unauthorized response has a 2xx status code
func (o *UnlockDatabaseByIDUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this unlock database by Id unauthorized response has a 3xx status code
func (o *UnlockDatabaseByIDUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this unlock database by Id unauthorized response has a 4xx status code
func (o *UnlockDatabaseByIDUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this unlock database by Id unauthorized response has a 5xx status code
func (o *UnlockDatabaseByIDUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this unlock database by Id unauthorized response a status code equal to that given
func (o *UnlockDatabaseByIDUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the unlock database by Id unauthorized response
func (o *UnlockDatabaseByIDUnauthorized) Code() int {
	return 401
}

func (o *UnlockDatabaseByIDUnauthorized) Error() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdUnauthorized ", 401)
}

func (o *UnlockDatabaseByIDUnauthorized) String() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdUnauthorized ", 401)
}

func (o *UnlockDatabaseByIDUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUnlockDatabaseByIDForbidden creates a UnlockDatabaseByIDForbidden with default headers values
func NewUnlockDatabaseByIDForbidden() *UnlockDatabaseByIDForbidden {
	return &UnlockDatabaseByIDForbidden{}
}

/*
UnlockDatabaseByIDForbidden describes a response with status code 403, with default header values.

UnlockDatabaseByIDForbidden unlock database by Id forbidden
*/
type UnlockDatabaseByIDForbidden struct {
}

// IsSuccess returns true when this unlock database by Id forbidden response has a 2xx status code
func (o *UnlockDatabaseByIDForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this unlock database by Id forbidden response has a 3xx status code
func (o *UnlockDatabaseByIDForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this unlock database by Id forbidden response has a 4xx status code
func (o *UnlockDatabaseByIDForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this unlock database by Id forbidden response has a 5xx status code
func (o *UnlockDatabaseByIDForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this unlock database by Id forbidden response a status code equal to that given
func (o *UnlockDatabaseByIDForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the unlock database by Id forbidden response
func (o *UnlockDatabaseByIDForbidden) Code() int {
	return 403
}

func (o *UnlockDatabaseByIDForbidden) Error() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdForbidden ", 403)
}

func (o *UnlockDatabaseByIDForbidden) String() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdForbidden ", 403)
}

func (o *UnlockDatabaseByIDForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUnlockDatabaseByIDNotFound creates a UnlockDatabaseByIDNotFound with default headers values
func NewUnlockDatabaseByIDNotFound() *UnlockDatabaseByIDNotFound {
	return &UnlockDatabaseByIDNotFound{}
}

/*
UnlockDatabaseByIDNotFound describes a response with status code 404, with default header values.

UnlockDatabaseByIDNotFound unlock database by Id not found
*/
type UnlockDatabaseByIDNotFound struct {
}

// IsSuccess returns true when this unlock database by Id not found response has a 2xx status code
func (o *UnlockDatabaseByIDNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this unlock database by Id not found response has a 3xx status code
func (o *UnlockDatabaseByIDNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this unlock database by Id not found response has a 4xx status code
func (o *UnlockDatabaseByIDNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this unlock database by Id not found response has a 5xx status code
func (o *UnlockDatabaseByIDNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this unlock database by Id not found response a status code equal to that given
func (o *UnlockDatabaseByIDNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the unlock database by Id not found response
func (o *UnlockDatabaseByIDNotFound) Code() int {
	return 404
}

func (o *UnlockDatabaseByIDNotFound) Error() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdNotFound ", 404)
}

func (o *UnlockDatabaseByIDNotFound) String() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdNotFound ", 404)
}

func (o *UnlockDatabaseByIDNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewUnlockDatabaseByIDUnsupportedMediaType creates a UnlockDatabaseByIDUnsupportedMediaType with default headers values
func NewUnlockDatabaseByIDUnsupportedMediaType() *UnlockDatabaseByIDUnsupportedMediaType {
	return &UnlockDatabaseByIDUnsupportedMediaType{}
}

/*
UnlockDatabaseByIDUnsupportedMediaType describes a response with status code 415, with default header values.

UnlockDatabaseByIDUnsupportedMediaType unlock database by Id unsupported media type
*/
type UnlockDatabaseByIDUnsupportedMediaType struct {
}

// IsSuccess returns true when this unlock database by Id unsupported media type response has a 2xx status code
func (o *UnlockDatabaseByIDUnsupportedMediaType) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this unlock database by Id unsupported media type response has a 3xx status code
func (o *UnlockDatabaseByIDUnsupportedMediaType) IsRedirect() bool {
	return false
}

// IsClientError returns true when this unlock database by Id unsupported media type response has a 4xx status code
func (o *UnlockDatabaseByIDUnsupportedMediaType) IsClientError() bool {
	return true
}

// IsServerError returns true when this unlock database by Id unsupported media type response has a 5xx status code
func (o *UnlockDatabaseByIDUnsupportedMediaType) IsServerError() bool {
	return false
}

// IsCode returns true when this unlock database by Id unsupported media type response a status code equal to that given
func (o *UnlockDatabaseByIDUnsupportedMediaType) IsCode(code int) bool {
	return code == 415
}

// Code gets the status code for the unlock database by Id unsupported media type response
func (o *UnlockDatabaseByIDUnsupportedMediaType) Code() int {
	return 415
}

func (o *UnlockDatabaseByIDUnsupportedMediaType) Error() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdUnsupportedMediaType ", 415)
}

func (o *UnlockDatabaseByIDUnsupportedMediaType) String() string {
	return fmt.Sprintf("[DELETE /databases/{id}/lock][%d] unlockDatabaseByIdUnsupportedMediaType ", 415)
}

func (o *UnlockDatabaseByIDUnsupportedMediaType) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
