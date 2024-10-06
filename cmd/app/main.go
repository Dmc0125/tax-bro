package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"tax-bro/pkg/utils"
	"tax-bro/view/middlewares"
	"tax-bro/view/routes/dashboard"
	"tax-bro/view/routes/signin"
	"tax-bro/view/routes/signout"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

func main() {
	loadAndValidateEnv("GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "DB_URL")

	e := echo.New()

	// e.Use(middleware.Logger())
	e.Use(middlewares.RegisterDb())
	e.Use(middlewares.Auth)

	dir, err := os.Getwd()
	utils.Assert(err == nil, fmt.Sprint(err))
	e.Static("/assets/", path.Join(dir, "view/assets"))

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.GET("/signin", signin.SigninHandler)
	e.GET("/signin/:provider", signin.AuthorizeHandler)
	e.GET("/signin/:provider/callback", signin.CallbackHandler)

	e.GET("/signout", signout.SignoutHandler)

	e.GET("/dashboard/wallets", dashboard.WalletsHandler)

	e.Logger.Fatal(e.Start(":3000"))
}

func loadAndValidateEnv(keys ...string) {
	err := godotenv.Load()
	utils.Assert(err == nil, fmt.Sprint(err))

	for _, k := range keys {
		v := os.Getenv(k)
		utils.Assert(len(v) > 0, fmt.Sprintf("ENV variable \"%s\" is missing", k))
	}

}
