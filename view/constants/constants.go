package constants

import (
	"tax-bro/view/utils"
)

var BASE_URL = utils.GetEnvVar("BASE_URL")
var OAUTH_STATE = utils.GetEnvVar("OAUTH_STATE")

const (
	ClientErrInternal                = "Internal server error. Please try again later"
	ClientErrInvalidProvider         = "Invalid OAuth provider"
	ClientErrEmailNotVerified        = "Email is not verified"
	ClientErrSigninMethodUnavailable = "Sign in method is currently not available"
	ClientErrUnprocessableContent    = "Request content is unprocessable"
)

const PostgresDuplicateUnique = "pq: duplicate key value violates unique constraint"
