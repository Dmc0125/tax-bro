package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"tax-bro/pkg/utils"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

type migrationTable struct {
	Exists bool
	Name   string
}

func loadMigrations(migrationsDir string, direction string) (migrations []string) {
	utils.Assert(direction == "up" || direction == "down", "invalid direction")

	err := filepath.WalkDir(migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n := d.Name()

			if strings.HasSuffix(n, fmt.Sprintf(".%s.sql", direction)) {
				migrations = append(migrations, n)
			}
		}
		return nil
	})
	utils.Assert(err == nil, fmt.Sprintf("unable to read directory: %s", err))

	return
}

type migrationsOpts struct {
	latest bool
	zero   bool
	steps  int
}

func prepareMigrations(
	migrations []string,
	direction, checkpoint string,
	opts *migrationsOpts,
) (string, []string) {
	newCheckpoint := ""
	executableMigrations := make([]string, len(migrations))
	copy(executableMigrations, migrations)

	if direction == "down" {
		slices.Reverse(executableMigrations)
	}

	if checkpoint != "" {
		idx := slices.IndexFunc(executableMigrations, func(filename string) bool {
			return strings.HasPrefix(filename, checkpoint)
		})
		if idx > -1 {
			if direction == "up" {
				idx = idx + 1
			}
			executableMigrations = executableMigrations[idx:]
		}
	}

	if len(executableMigrations) == 0 {
		return "", []string{}
	}

	if opts.steps != 0 {
		s := math.Min(float64(len(executableMigrations)), math.Abs(float64(opts.steps)))
		executableMigrations = executableMigrations[:int(s)]
	}

	if opts.zero {
		newCheckpoint = ""
	} else if direction == "down" {
		last := executableMigrations[len(executableMigrations)-1]
		idx := slices.Index(migrations, last)
		if idx > 0 {
			newCheckpoint = migrations[idx-1]
		} else {
			newCheckpoint = ""
		}
	} else {
		newCheckpoint = executableMigrations[len(executableMigrations)-1]
	}

	nameParts := strings.Split(newCheckpoint, ".")
	return nameParts[0], executableMigrations
}

func executeMigrations(
	db *sqlx.DB,
	migrationsDir string,
	migrations []string,
	newCheckpoint string,
) error {
	if len(migrations) == 0 {
		return errors.New("all migrations executed")
	}

	tx, err := db.Begin()
	utils.Assert(err == nil, fmt.Sprint(err))
	defer tx.Rollback()

	for _, filename := range migrations {
		log.Printf("Executing migration: %s\n", filename)

		content, err := os.ReadFile(filepath.Join(migrationsDir, "/", filename))
		if err != nil {
			return err
		}

		_, err = tx.Exec(string(content))
		if err != nil {
			return fmt.Errorf("unable to execute migration \"%s\": %s", filename, err)
		}
	}

	if newCheckpoint == "" {
		_, err = tx.Exec("UPDATE migration SET name = $1", nil)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec("UPDATE migration SET name = $1", newCheckpoint)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	utils.Assert(err == nil, fmt.Sprint(err))

	return nil
}

func main() {
	godotenv.Load()
	dbUrl := os.Getenv("DB_URL")
	utils.Assert(dbUrl != "", "missing DB_URL env var")

	latest := flag.Bool("latest", false, "execute all migrations up")
	zero := flag.Bool("zero", false, "execute all migrations down")
	steps := flag.Int("steps", 0, "number of migrations to run")
	flag.Parse()
	utils.Assert(*steps != 0 || *zero || *latest, "at least one flag needs to be set")

	db, err := sqlx.Open("postgres", dbUrl)
	utils.Assert(err == nil, fmt.Sprintf("unable to connect to db: %s", err))

	_, filename, _, ok := runtime.Caller(0)
	utils.Assert(ok, "unable to get filename")
	migrationsDir := filepath.Join(filename, "../../../db_migrations")

	currentMigration := migrationTable{}
	db.Get(&currentMigration, "SELECT exists, name FROM migration")

	if !currentMigration.Exists {
		_, err := db.Exec(`
			CREATE TABLE migration (
				id INTEGER PRIMARY KEY DEFAULT 0,
				exists BOOLEAN NOT NULL DEFAULT true,
				name VARCHAR
			);

			INSERT INTO migration DEFAULT VALUES;
		`)
		utils.Assert(err == nil, fmt.Sprint(err))
	}

	direction := ""
	if *latest || *steps > 0 {
		direction = "up"
	} else {
		direction = "down"
	}

	if currentMigration.Name == "" && direction == "down" {
		log.Fatal("unable to migrate down if no migrations were executed")
	}

	migrations := loadMigrations(migrationsDir, direction)

	opts := migrationsOpts{
		latest: *latest,
		zero:   *zero,
		steps:  *steps,
	}
	newCheckpoint, preparedMigrations := prepareMigrations(
		migrations,
		direction,
		currentMigration.Name,
		&opts,
	)
	err = executeMigrations(
		db,
		migrationsDir,
		preparedMigrations,
		newCheckpoint,
	)
	if err != nil {
		log.Fatalf("unable to execute migrations: %s", err)
	}

	log.Println("Migrations executed succesfully")
}
