package testutils

import (
	"context"
	"log"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func SetupDb(t *testing.T) *sqlx.DB {
	var (
		dbName      = "db"
		dbPassword  = "pwd"
		dbUser      = "user"
		_, fp, _, _ = runtime.Caller(0)
	)

	ctx := context.Background()
	pgContainer, err := postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.WithInitScripts(
			filepath.Join(fp, "../../../db_migrations/00_setup.up.sql"),
			filepath.Join(fp, "../../../db_migrations/01_user.up.sql"),
			filepath.Join(fp, "../../../db_migrations/02_txs.up.sql"),
			filepath.Join(fp, "../../../db_migrations/03_user_txs.up.sql"),
		),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		log.Fatal(err)
	}

	dbUrl, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	db, err := sqlx.Open("postgres", dbUrl)
	if err != nil {
		log.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate pg container: %s", err)
		}
	})

	return db
}
