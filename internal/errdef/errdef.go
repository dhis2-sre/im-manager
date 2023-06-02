package errdef

import (
	"errors"
	"fmt"
)

func NewForbidden(format string, a ...any) error {
	return forbidden{fmt.Errorf(format, a...)}
}

type forbidden struct{ error }

func IsForbidden(err error) bool {
	var e forbidden
	return errors.As(err, &e)
}

func NewBadRequest(format string, a ...any) error {
	return badRequest{fmt.Errorf(format, a...)}
}

type badRequest struct{ error }

func IsBadRequest(err error) bool {
	var e badRequest
	return errors.As(err, &e)
}

func NewUnsupportedMediaType(format string, a ...any) error {
	return unsupportedMediaType{fmt.Errorf(format, a...)}
}

type unsupportedMediaType struct{ error }

func IsUnsupportedMediaType(err error) bool {
	var e unsupportedMediaType
	return errors.As(err, &e)
}

func NewDuplicated(format string, a ...any) error {
	return duplicated{fmt.Errorf(format, a...)}
}

type duplicated struct{ error }

func IsDuplicated(err error) bool {
	var e duplicated
	return errors.As(err, &e)
}

func NewUnauthorized(format string, a ...any) error {
	return unauthorized{fmt.Errorf(format, a...)}
}

type unauthorized struct{ error }

func IsUnauthorized(err error) bool {
	var e unauthorized
	return errors.As(err, &e)
}

// NewNotFound creates an error representing a resource that could not be found.
func NewNotFound(format string, a ...any) error {
	return notFound{fmt.Errorf(format, a...)}
}

type notFound struct{ error }

// IsNotFound returns true if err is an error representing a resource that could not be found and false otherwise.
func IsNotFound(err error) bool {
	var e notFound
	return errors.As(err, &e)
}
