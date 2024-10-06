package middlewares

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type AuthenticatedContext struct {
	*DbContext
	SessionId string
	UserId    int32
}

func Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		p := c.Request().URL.Path

		if !strings.HasPrefix(p, "/dashboard") && !strings.HasPrefix(p, "dashboard") {
			return next(c)
		}

		cookie, err := c.Cookie("auth")
		if err != nil {
			return c.Redirect(http.StatusPermanentRedirect, "/signin")
		}

		sessionId := cookie.Value
		dbContext := c.(*DbContext)
		db := dbContext.Db

		savedSession := struct {
			UserId int32 `db:"user_id"`
		}{}
		err = db.Get(&savedSession, "SELECT user_id FROM \"session\" WHERE id = $1", sessionId)
		if errors.Is(err, sql.ErrNoRows) {
			return c.Redirect(http.StatusPermanentRedirect, "/signin")
		} else if err != nil {
			log.Print(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		ac := &AuthenticatedContext{
			DbContext: dbContext,
			SessionId: sessionId,
			UserId:    savedSession.UserId,
		}
		return next(ac)
	}
}
