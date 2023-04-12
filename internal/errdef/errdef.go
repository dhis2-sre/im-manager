package errdef

import (
	"errors"
)

func NewDuplicated(err error) error {
	return duplicated{err}
}

type duplicated struct{ error }

func IsDuplicated(err error) bool {
	var e duplicated
	return errors.As(err, &e)
}

func NewUnauthorized(err error) error {
	return unauthorized{err}
}

type unauthorized struct{ error }

func IsUnauthorized(err error) bool {
	var e unauthorized
	return errors.As(err, &e)
}

// NewNotFound creates an error representing a resource that could not be found.
func NewNotFound(err error) error {
	return notFound{err}
}

type notFound struct{ error }

// IsNotFound returns true if err is an error representing a resource that could not be found and false otherwise.
func IsNotFound(err error) bool {
	var e notFound
	return errors.As(err, &e)
}
