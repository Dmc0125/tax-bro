package middlewares

import (
	"fmt"
	"os"
	"tax-bro/pkg/utils"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type DbContext struct {
	echo.Context
	Db *sqlx.DB
}

func RegisterDb() func(next echo.HandlerFunc) echo.HandlerFunc {
	db, err := sqlx.Open("postgres", os.Getenv("DB_URL"))
	utils.Assert(err == nil, fmt.Sprint(err))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rc := &DbContext{
				Context: c,
				Db:      db,
			}
			return next(rc)
		}
	}
}
