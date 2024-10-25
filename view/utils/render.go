package utils

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

func RenderOk(c echo.Context, t templ.Component) error {
	return RenderWithStatus(c, t, http.StatusOK)
}

func RenderWithStatus(c echo.Context, t templ.Component, status int) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(c.Request().Context(), buf); err != nil {
		return err
	}

	return c.HTML(status, buf.String())
}
