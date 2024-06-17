package user

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
)

// swagger:parameters signUp
type _ struct {
	// SignUp request body parameter
	// in: body
	// required: true
	Body signUpRequest
}

// swagger:parameters requestPasswordReset
type _ struct {
	// Request password reset request body parameter
	// in: body
	// required: true
	Body RequestPasswordResetRequest
}

// swagger:parameters resetPassword
type _ struct {
	// Reset password request body parameter
	// in: body
	// required: true
	Body ResetPasswordRequest
}

// swagger:parameters refreshToken
type _ struct {
	// Refresh token request body parameter. Note that this is optional and the refresh token can also be supplied using a cookie named "refreshToken"
	// in: body
	// required: false
	Body RefreshTokenRequest
}

// swagger:parameters findUserById deleteUser updateUser
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:parameters validateEmail
type _ struct {
	// Email validation token request body parameter
	// in: body
	// required: true
	Body validateEmailRequest
}

// swagger:response Tokens
type _ struct {
	//in: body
	_ token.Tokens
}

// swagger:response UsersResponse
type _ struct {
	// Users list response
	//in: body
	_ *[]model.User
}

// swagger:parameters updateUser
type _ struct {
	// Update user request
	// in: body
	// required: true
	Body updateUserRequest
}
