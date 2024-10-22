package auth

import (
	"net/http"
	"tax-bro/pkg/logger"
	"tax-bro/view/constants"
	"tax-bro/view/middlewares"

	"github.com/labstack/echo/v4"
)

func HandleSignOut(c echo.Context) error {
	cookie, err := c.Cookie("auth")

	if err != nil {
		return c.Redirect(http.StatusPermanentRedirect, "/signin")
	}

	sessionId := cookie.Value
	db := c.(*middlewares.DbContext).Db
	if _, err = db.Exec("DELETE FROM session WHERE id = $1", sessionId); err != nil {
		logger.Log(err)
		return redirectErr(c, constants.ClientErrInternal)
	}
	c.SetCookie(createAuthCookie("", -1))

	return c.Redirect(http.StatusPermanentRedirect, "/signin")
}
