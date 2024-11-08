package testutils

import (
	"context"
	"log"
	"path/filepath"
	"tax-bro/pkg/utils"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
	*sqlx.DB
	dbUrl   string
	cleanup func()
}

func NewPostgresContainer(ctx context.Context, migrationsDir string) PostgresContainer {
	postgres, err := postgres.Run(
		ctx,
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

	dbUrl, err := postgres.ConnectionString(context.Background(), "sslmode=disable")
	utils.AssertNoErr(err)
	db, err := sqlx.Open("postgres", dbUrl)
	utils.AssertNoErr(err, "unable to open postgres connection")

	return PostgresContainer{
		dbUrl: dbUrl,
		DB:    db,
		cleanup: func() {
			db.Close()
			if err := testcontainers.TerminateContainer(postgres); err != nil {
				log.Printf("unable to gracefully terminate postgres container: %s", err)
			}
		},
	}
}
