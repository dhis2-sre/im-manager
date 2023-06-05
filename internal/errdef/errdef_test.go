package errdef_test

import (
	"errors"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/stretchr/testify/assert"
)

func TestIsForbidden(t *testing.T) {
	assert.False(t, errdef.IsForbidden(errors.New("some error")))
	assert.True(t, errdef.IsForbidden(errdef.NewForbidden("some error")))
}

func TestIsBadRequest(t *testing.T) {
	assert.False(t, errdef.IsBadRequest(errors.New("some error")))
	assert.True(t, errdef.IsBadRequest(errdef.NewBadRequest("some error")))
}

func TestIsDuplicate(t *testing.T) {
	assert.False(t, errdef.IsDuplicated(errors.New("some error")))
	assert.True(t, errdef.IsDuplicated(errdef.NewDuplicated("some error")))
}

func TestIsUnauthorized(t *testing.T) {
	assert.False(t, errdef.IsUnauthorized(errors.New("some error")))
	assert.True(t, errdef.IsUnauthorized(errdef.NewUnauthorized("some error")))
}

func TestIsNotFound(t *testing.T) {
	assert.False(t, errdef.IsNotFound(errors.New("some error")))
	assert.True(t, errdef.IsNotFound(errdef.NewNotFound("some error")))
}
