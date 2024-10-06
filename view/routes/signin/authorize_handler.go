package signin

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"tax-bro/view/middlewares"
	"tax-bro/view/routes/signout"

	"github.com/labstack/echo/v4"
)

func checkIsSignedIn(c echo.Context) (bool, error) {
	cookie, err := c.Cookie("auth")
	if err != nil {
		return false, nil
	}

	db := c.(*middlewares.DbContext).Db
	session := struct {
		UserId int32 `db:"user_id"`
	}{}
	err = db.Get(&session, "SELECT user_id FROM \"session\" WHERE id = $1", cookie.Value)
	if errors.Is(err, sql.ErrNoRows) {
		c.SetCookie(signout.DeletedAuthCookie)
		return false, nil
	} else if err != nil {
		c.SetCookie(signout.DeletedAuthCookie)
		return false, errors.New("db error")
	}

	return true, nil
}

func AuthorizeHandler(c echo.Context) error {
	isSignedIn, err := checkIsSignedIn(c)
	log.Printf("isisngedin %t err %s\n", isSignedIn, err)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	if isSignedIn {
		return c.Redirect(http.StatusPermanentRedirect, "/dashboard/wallets")
	}

	provider := c.Param("provider")
	switch provider {
	case "github":
		return c.Redirect(
			http.StatusTemporaryRedirect,
			fmt.Sprintf(
				"https://github.com/login/oauth/authorize?scope=user&client_id=%s",
				os.Getenv("GITHUB_CLIENT_ID"),
			),
		)
	case "discord":
		return c.HTML(http.StatusOK, provider)
	default:
		return c.NoContent(http.StatusNotFound)
	}
}
