package user

import "github.com/dhis2-sre/im-manager/pkg/token"

// swagger:parameters signUp
type _ struct {
	// SignUp request body parameter
	// in: body
	// required: true
	Body SignUpRequest
}

// swagger:parameters refreshToken
type _ struct {
	// Refresh token request body parameter
	// in: body
	// required: true
	Body RefreshTokenRequest
}

// swagger:parameters findUserById deleteUser updateUser
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:response Tokens
type _ struct {
	//in: body
	_ token.Tokens
}
