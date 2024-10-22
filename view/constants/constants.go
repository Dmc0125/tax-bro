package constants

import (
	"tax-bro/view/utils"
)

var BASE_URL = utils.GetEnvVar("BASE_URL")
var OAUTH_STATE = utils.GetEnvVar("OAUTH_STATE")

const (
	ClientErrInternal              = "internal"
	ClientErrInvalidProvider       = "invalid_provider"
	ClientErrEmailNotVerified      = "email_not_verified"
	ClientErrDiscordAuthNotWorking = "discord_auth_not_working"
)
