package apperror

import (
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"log"
	"net/http"
)

type Type string

const (
	BadRequest           Type = "BAD_REQUEST"
	Internal             Type = "INTERNAL"
	UnsupportedMediaType Type = "UNSUPPORTED_MEDIA_TYPE"
	Unauthorized         Type = "AUTHORIZATION"
	NotFound             Type = "NOT_FOUND"
	Conflict             Type = "CONFLICT"
)

type Error struct {
	Type    Type   `json:"type"`
	Message string `json:"message"`
}

// Implement errors interface
func (e *Error) Error() string {
	return e.Message
}

func (e *Error) HttpStatusCode() int {
	switch e.Type {
	case BadRequest:
		return http.StatusBadRequest
	case Internal:
		return http.StatusInternalServerError
	case UnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case Unauthorized:
		return http.StatusUnauthorized
	case NotFound:
		return http.StatusNotFound
	case Conflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func ToHttpStatusCode(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.HttpStatusCode()
	}
	return http.StatusInternalServerError
}

func NewUnsupportedMediaType(message string) *Error {
	return &Error{
		Type:    UnsupportedMediaType,
		Message: message,
	}
}

func NewInternal(message string) *Error {
	logId, _ := uuid.NewV4()
	log.Printf("%s\nLog Id: %s", message, logId)
	return &Error{
		Type:    Internal,
		Message: fmt.Sprintf("Internal server error! Logged with id: %s", logId),
	}
}

func NewBadRequest(message string) *Error {
	return &Error{
		Type:    BadRequest,
		Message: message,
	}
}

func NewUnauthorized(message string) *Error {
	return &Error{
		Type:    Unauthorized,
		Message: message,
	}
}

func NewNotFound(name string, value string) *Error {
	return &Error{
		Type:    NotFound,
		Message: fmt.Sprintf("Resource: %s with value: %s not found", name, value),
	}
}

func NewConflict(message string) *Error {
	return &Error{
		Type:    Conflict,
		Message: message,
	}
}
