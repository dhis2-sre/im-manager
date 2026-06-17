package storage

import (
	"errors"
	"testing"

	smithy "github.com/aws/smithy-go"
	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAPIError struct {
	code    string
	message string
}

func (e *fakeAPIError) ErrorCode() string             { return e.code }
func (e *fakeAPIError) ErrorMessage() string          { return e.message }
func (e *fakeAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }
func (e *fakeAPIError) Error() string                 { return e.code + ": " + e.message }

func TestS3AuthErr(t *testing.T) {
	authCodes := []string{
		"AuthorizationHeaderMalformed",
		"InvalidAccessKeyId",
		"SignatureDoesNotMatch",
		"AccessDenied",
		"RequestTimeTooSkewed",
	}

	for _, code := range authCodes {
		t.Run(code, func(t *testing.T) {
			err := s3AuthErr(&fakeAPIError{code: code, message: "some detail"})
			require.Error(t, err)
			assert.True(t, errdef.IsServiceUnavailable(err), "expected ServiceUnavailable for %s", code)
		})
	}

	t.Run("non-auth error returns nil", func(t *testing.T) {
		err := s3AuthErr(&fakeAPIError{code: "NoSuchKey", message: "key not found"})
		assert.NoError(t, err)
	})

	t.Run("non-API error returns nil", func(t *testing.T) {
		err := s3AuthErr(errors.New("some network error"))
		assert.NoError(t, err)
	})
}
