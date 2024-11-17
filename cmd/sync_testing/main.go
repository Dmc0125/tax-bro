package main

// type postgresContainer struct {
// 	*sqlx.DB
// 	dbUrl   string
// 	cleanup func()
// }

// func newPostgresContainer(ctx context.Context) postgresContainer {
// 	_, filename, _, ok := runtime.Caller(0)
// 	utils.Assert(ok, "unable to get filename")
// 	migrationsDir := filepath.Join(filename, "../../../db_migrations")
// 	postgres, err := postgres.Run(
// 		ctx,
// 		"docker.io/postgres:16-alpine",
// 		postgres.WithInitScripts(
// 			filepath.Join(migrationsDir, "/00_setup.up.sql"),
// 			filepath.Join(migrationsDir, "/01_user.up.sql"),
// 			filepath.Join(migrationsDir, "/02_txs.up.sql"),
// 			filepath.Join(migrationsDir, "/03_user_txs.up.sql"),
// 		),
// 		testcontainers.WithWaitStrategy(
// 			wait.ForLog("database system is ready to accept connections").
// 				WithOccurrence(2).WithStartupTimeout(10*time.Second)),
// 	)
// 	utils.AssertNoErr(err, "unable to start postgres container")

// 	dbUrl, err := postgres.ConnectionString(context.Background(), "sslmode=disable")
// 	utils.AssertNoErr(err)
// 	db, err := sqlx.Open("postgres", dbUrl)
// 	utils.AssertNoErr(err, "unable to open postgres connection")

// 	return postgresContainer{
// 		dbUrl: dbUrl,
// 		DB:    db,
// 		cleanup: func() {
// 			db.Close()
// 			if err := testcontainers.TerminateContainer(postgres); err != nil {
// 				log.Printf("unable to gracefully terminate postgres container: %s", err)
// 			}
// 		},
// 	}
// }

// func createUser(db *sqlx.DB, rpcClient *rpc.Client, walletAddress string) (int32, int32, string) {
// 	slog.Info("creating account", "walletAddress", walletAddress)
// 	account := struct{ Id int32 }{}
// 	err := db.Get(&account, "INSERT INTO account (selected_auth_provider, email) VALUES ($1, $2) RETURNING id", "github", "x@x.com")
// 	utils.AssertNoErr(err)

// 	pk, err := solana.PublicKeyFromBase58(walletAddress)
// 	utils.AssertNoErr(err)
// 	address := database.Account{}
// 	err = db.Get(&address, "INSERT INTO address (value) VALUES ($1) RETURNING id, value as address", walletAddress)
// 	utils.AssertNoErr(err)

// 	wallet := struct{ Id int32 }{}
// 	err = db.Get(&wallet, "INSERT INTO wallet (account_id, address_id, label) VALUES ($1, $2, $3) RETURNING id", account.Id, address.Id, "")
// 	utils.AssertNoErr(err)

// 	_, err = db.Exec("INSERT INTO sync_wallet_request (wallet_id) VALUES ($1)", wallet.Id)
// 	utils.AssertNoErr(err)
// 	slog.Info("account created")

// 	slog.Info("fetching all wallet signatures")
// 	limit := int(1000)
// 	opts := &rpc.GetSignaturesForAddressOpts{
// 		Limit:      &limit,
// 		Commitment: rpc.CommitmentConfirmed,
// 	}
// 	count := int32(0)
// 	lastSignature := ""

// 	for {
// 		signatures, err := walletsync.CallRpcWithRetries(func() ([]*rpc.TransactionSignature, error) {
// 			// can be context.Backround() - it always runs prior to everything
// 			return rpcClient.GetSignaturesForAddressWithOpts(context.Background(), pk, opts)
// 		}, 5)
// 		utils.AssertNoErr(err)
// 		if lastSignature == "" && len(signatures) > 0 {
// 			lastSignature = signatures[0].Signature.String()
// 		}
// 		count += int32(len(signatures))
// 		if len(signatures) < limit {
// 			break
// 		}
// 		opts.Before = signatures[len(signatures)-1].Signature
// 	}

// 	slog.Info("wallet signatures fetched", "signaturesCount", count)
// 	return count, wallet.Id, lastSignature
// }

// func main() {
// 	err := godotenv.Load(".env.sync_testing")
// 	utils.AssertNoErr(err, "unable to load env variables")

// 	var logPath string
// 	var logLevel int
// 	flag.StringVar(&logPath, "log-path", "", "path to log file")
// 	flag.IntVar(&logLevel, "log-level", -4, "logging level (-4: debug, 0: info, 4: warn, 8: error")
// 	flag.Parse()
// 	utils.Assert(logLevel == -4 || logLevel == 0 || logLevel == 4 || logLevel == 8, "invalid log level")

// 	rpcUrl := utils.GetEnvVar("RPC_URL")
// 	walletAddress := utils.GetEnvVar("WALLET")

// 	logger.NewPrettyLogger(logPath, logLevel)

// 	ctx, cancel := context.WithCancel(context.Background())
// 	pg := newPostgresContainer(ctx)
// 	defer pg.cleanup()

// 	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(rpcUrl, rate.Every(time.Second), 5))
// 	client := walletsync.NewClient(ctx, pg.DB, rpcClient)
// 	go client.Run()

// 	signaturesCount, walletId, lastSignature := createUser(pg.DB, rpcClient, walletAddress)

// 	ticker := time.NewTicker(5 * time.Second)
// 	for {
// 		<-ticker.C
// 		wallet := struct {
// 			SignaturesCount int32          `db:"signatures"`
// 			LastSignature   sql.NullString `db:"last_signature"`
// 			Status          sql.NullString
// 		}{}
// 		q := `
// 			SELECT
// 				wallet.signatures, sync_wallet_request.status, signature.value as last_signature
// 			FROM
// 				wallet
// 			LEFT JOIN
// 				signature ON signature.id = wallet.last_signature_id
// 			LEFT JOIN
// 				sync_wallet_request ON sync_wallet_request.wallet_id = wallet.id
// 			WHERE
// 				wallet.id = $1
// 		`
// 		err := pg.DB.Get(&wallet, q, walletId)
// 		if errors.Is(err, sql.ErrNoRows) {
// 			continue
// 		}
// 		utils.AssertNoErr(err)
// 		if wallet.Status.String == "parsing_events" {
// 			slog.Info(
// 				"all signatures fetched",
// 				"wallet.signatures", wallet.SignaturesCount,
// 				"signaturesCount", signaturesCount,
// 				"wallet.last_signature", wallet.LastSignature.String,
// 				"lastSignature", lastSignature,
// 			)
// 			break
// 		}
// 	}

// 	cancel()
// }
