package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"tax-bro/pkg/logger"
	"tax-bro/view/constants"
	"tax-bro/view/middlewares"
	"time"

	"github.com/labstack/echo/v4"
)

type AuthenticatedContext struct {
	*middlewares.DbContext
	SessionId string
	AccountId int32
}

var authRoutesPrefixes = []string{
	"/wallets",
	"wallets",
}

func authenticate(c echo.Context) (ac AuthenticatedContext, e error) {
	ac.AccountId = 0

	cookie, err := c.Cookie("auth")
	if err != nil {
		return
	}

	sessionId := cookie.Value
	dbContext := c.(*middlewares.DbContext)
	db := dbContext.Db

	savedSession := struct {
		AccountId int32     `db:"account_id"`
		ExpiresAt time.Time `db:"expires_at"`
	}{}
	err = db.Get(&savedSession, "SELECT account_id, expires_at FROM \"session\" WHERE id = $1", sessionId)
	if errors.Is(err, sql.ErrNoRows) {
		c.SetCookie(createAuthCookie("", -1))
		return
	} else if err != nil {
		logger.Log(err)
		e = err
		return
	}

	nowSecs := time.Now().Unix()
	if nowSecs > savedSession.ExpiresAt.Unix() {
		if _, err = db.Exec("DELETE FROM session WHERE id = $1", sessionId); err != nil {
			logger.Log(err)
			e = err
			return
		}
		c.SetCookie(createAuthCookie("", -1))
		return
	}

	ac = AuthenticatedContext{
		DbContext: dbContext,
		SessionId: sessionId,
		AccountId: savedSession.AccountId,
	}
	return
}

func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		p := c.Request().URL.Path
		hasAuth := slices.IndexFunc(authRoutesPrefixes, func(prefix string) bool {
			return strings.HasPrefix(p, prefix)
		}) > -1
		if !hasAuth {
			return next(c)
		}

		ac, err := authenticate(c)

		if err != nil {
			return c.Redirect(http.StatusPermanentRedirect, fmt.Sprintf("/signin?error=%s", constants.ClientErrInternal))
		}
		if ac.AccountId == 0 {
			return c.Redirect(http.StatusPermanentRedirect, "/signin")
		}

		return next(&ac)
	}
}
