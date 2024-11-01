package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"tax-bro/pkg/database"
	"tax-bro/pkg/logger"
	"tax-bro/pkg/utils"
	walletsync "tax-bro/pkg/wallet_sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/time/rate"
)

type postgresContainer struct {
	*sqlx.DB
	dbUrl   string
	cleanup func()
}

func newPostgresContainer(ctx context.Context) postgresContainer {
	_, filename, _, ok := runtime.Caller(0)
	utils.Assert(ok, "unable to get filename")
	migrationsDir := filepath.Join(filename, "../../../db_migrations")
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
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	utils.AssertNoErr(err, "unable to start postgres container")

	dbUrl, err := postgres.ConnectionString(context.Background(), "sslmode=disable")
	utils.AssertNoErr(err)
	db, err := sqlx.Open("postgres", dbUrl)
	utils.AssertNoErr(err, "unable to open postgres connection")

	return postgresContainer{
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

func getEnvVar(name string) string {
	v := os.Getenv(name)
	if v == "" {
		err := godotenv.Load()
		utils.Assert(err == nil, fmt.Sprintf("Unable to load env from .env %s", err))
		v = os.Getenv(name)
		if v == "" {
			log.Fatalf("Env variable \"%s\" missing", name)
		}
	}
	return v
}

func createUser(db *sqlx.DB, rpcClient *rpc.Client, walletAddress string) (int32, int32, string) {
	slog.Info("creating account", "walletAddress", walletAddress)
	account := struct{ Id int32 }{}
	err := db.Get(&account, "INSERT INTO account (selected_auth_provider, email) VALUES ($1, $2) RETURNING id", "github", "x@x.com")
	utils.AssertNoErr(err)

	pk, err := solana.PublicKeyFromBase58(walletAddress)
	utils.AssertNoErr(err)
	address := database.Account{}
	err = db.Get(&address, "INSERT INTO address (value) VALUES ($1) RETURNING id, value as address", walletAddress)
	utils.AssertNoErr(err)

	wallet := struct{ Id int32 }{}
	err = db.Get(&wallet, "INSERT INTO wallet (account_id, address_id, label) VALUES ($1, $2, $3) RETURNING id", account.Id, address.Id, "")
	utils.AssertNoErr(err)

	_, err = db.Exec("INSERT INTO sync_wallet_request (wallet_id) VALUES ($1)", wallet.Id)
	utils.AssertNoErr(err)
	slog.Info("account created")

	slog.Info("fetching all wallet signatures")
	limit := int(1000)
	opts := &rpc.GetSignaturesForAddressOpts{
		Limit:      &limit,
		Commitment: rpc.CommitmentConfirmed,
	}
	count := int32(0)
	lastSignature := ""

	for {
		signatures, err := walletsync.CallRpcWithRetries(func() ([]*rpc.TransactionSignature, error) {
			// can be context.Backround() - it always runs prior to everything
			return rpcClient.GetSignaturesForAddressWithOpts(context.Background(), pk, opts)
		}, 5)
		utils.AssertNoErr(err)
		if lastSignature == "" && len(signatures) > 0 {
			lastSignature = signatures[0].Signature.String()
		}
		count += int32(len(signatures))
		if len(signatures) < limit {
			break
		}
		opts.Before = signatures[len(signatures)-1].Signature
	}

	slog.Info("wallet signatures fetched", "signaturesCount", count)
	return count, wallet.Id, lastSignature
}

func main() {
	err := godotenv.Load(".env.sync_testing")
	utils.AssertNoErr(err, "unable to load env variables")

	var logPath string
	var logLevel int
	flag.StringVar(&logPath, "log-path", "", "path to log file")
	flag.IntVar(&logLevel, "log-level", -4, "logging level (-4: debug, 0: info, 4: warn, 8: error")
	flag.Parse()
	utils.Assert(logLevel == -4 || logLevel == 0 || logLevel == 4 || logLevel == 8, "invalid log level")

	rpcUrl := getEnvVar("RPC_URL")
	walletAddress := getEnvVar("WALLET")

	logger.NewPrettyLogger(logPath, logLevel)

	ctx, cancel := context.WithCancel(context.Background())
	pg := newPostgresContainer(ctx)
	defer pg.cleanup()

	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(rpcUrl, rate.Every(time.Second), 5))
	client := walletsync.NewClient(ctx, pg.DB, rpcClient)
	go client.Run()

	signaturesCount, walletId, lastSignature := createUser(pg.DB, rpcClient, walletAddress)

	ticker := time.NewTicker(5 * time.Second)
	for {
		<-ticker.C
		wallet := struct {
			SignaturesCount int32          `db:"signatures"`
			LastSignature   sql.NullString `db:"last_signature"`
		}{}
		err := pg.DB.Get(
			&wallet,
			"SELECT wallet.signatures, address.value as last_signature FROM wallet LEFT JOIN address ON address.id = wallet.last_signature_id WHERE wallet.id = $1",
			walletId,
		)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		utils.AssertNoErr(err)
		if wallet.SignaturesCount >= signaturesCount && wallet.LastSignature.Valid {
			slog.Info(
				"all signatures fetched",
				"wallet.signatures", wallet.SignaturesCount,
				"signaturesCount", signaturesCount,
				"wallet.last_signature", wallet.LastSignature.String,
				"lastSignature", lastSignature,
			)
			break
		}
	}

	cancel()
}
