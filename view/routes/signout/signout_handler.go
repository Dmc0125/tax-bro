package signout

import (
	"net/http"
	"tax-bro/view/middlewares"

	"github.com/labstack/echo/v4"
)

var DeletedAuthCookie = &http.Cookie{
	Name:     "auth",
	Value:    "",
	MaxAge:   -1,
	HttpOnly: true,
	Path:     "/",
}

func SignoutHandler(c echo.Context) error {
	existingCookie, err := c.Cookie("auth")
	if err != nil {
		return c.Redirect(http.StatusPermanentRedirect, "/signin")
	}

	sessionId := existingCookie.Value

	db := c.(*middlewares.DbContext).Db
	if _, err = db.Exec("DELETE FROM \"session\" WHERE id = $1", sessionId); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	c.SetCookie(DeletedAuthCookie)
	return c.Redirect(http.StatusPermanentRedirect, "/signin")
}
