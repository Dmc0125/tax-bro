package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"tax-bro/pkg/utils"
	"tax-bro/view/auth"
	"tax-bro/view/components"
	"tax-bro/view/middlewares"
	vutils "tax-bro/view/utils"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	utils.Assert(err == nil, fmt.Sprint(err))

	e := echo.New()

	e.Use(middlewares.RegisterDb())
	e.Use(auth.AuthMiddleware)

	dir, err := os.Getwd()
	utils.Assert(err == nil, fmt.Sprint(err))
	e.Static("/assets/", path.Join(dir, "view/assets"))

	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, "HELLO")
	})

	e.GET("/signin", func(c echo.Context) error {
		return vutils.Render(c, components.SignInView())
	})
	e.GET("/signin/:provider", auth.HandleSignIn)
	e.GET("/signin/:provider/callback", auth.HandleSignInCallback)
	e.GET("/signout", auth.HandleSignOut)

	e.GET("/wallets", func(c echo.Context) error {
		return c.HTML(http.StatusOK, "WALLETS")
	})

	e.Logger.Fatal(e.Start(":3000"))
}
