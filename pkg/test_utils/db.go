package testutils

import (
	"context"
	"log"
	"path"
	"path/filepath"
	"tax-bro/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func InitDb() (*pgx.Conn, func()) {
	projectDir := utils.GetProjectDir()
	migrationsDir := path.Join(projectDir, "./db_migrations")
	postgres, err := postgres.Run(
		context.Background(),
		"docker.io/postgres:16-alpine",
		postgres.WithInitScripts(
			filepath.Join(migrationsDir, "/00_setup.up.sql"),
			filepath.Join(migrationsDir, "/01_user.up.sql"),
			filepath.Join(migrationsDir, "/02_txs.up.sql"),
			filepath.Join(migrationsDir, "/03_user_txs.up.sql"),
		),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(10*time.Second)),
	)
	utils.AssertNoErr(err, "unable to start postgres container")
	db, err := pgx.Connect(context.Background(), postgres.MustConnectionString(context.Background(), "sslmode=disable"))
	utils.AssertNoErr(err, "unable to connect to db")
	cleanup := func() {
		db.Close(context.Background())
		if err := testcontainers.TerminateContainer(postgres); err != nil {
			log.Printf("unable to gracefully terminate postgres container: %s", err)
		}
	}
	return db, cleanup
}
