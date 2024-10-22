package auth

import (
	"net/http"
	"tax-bro/view/constants"

	"github.com/labstack/echo/v4"
)

func HandleSignIn(c echo.Context) error {
	ac, err := authenticate(c)
	if err != nil {
		return redirectErr(c, constants.ClientErrInternal)
	}
	if ac.AccountId != 0 {
		return c.Redirect(http.StatusPermanentRedirect, "/wallets")
	}

	provider := c.Param("provider")
	// if provider == "discord" {
	// 	return redirectErr(c, constants.ClientErrDiscordAuthNotWorking)
	// }

	oauthConfig := oauthConfigs[provider]
	if oauthConfig == nil {
		return redirectErr(c, constants.ClientErrInvalidProvider)

	}
	authUrl := oauthConfig.AuthCodeURL(constants.OAUTH_STATE)

	return c.Redirect(http.StatusPermanentRedirect, authUrl)
}
