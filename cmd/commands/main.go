package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var commands = []string{"new-account", "new-wallet", "new-address", "new-sync-request", "unknown-ixs"}

func main() {
	err := godotenv.Load()
	utils.AssertNoErr(err, "unable to load .env")

	dbUrl := utils.GetEnvVar("DB_URL")

	ctx := context.Background()
	db, err := pgx.Connect(ctx, dbUrl)
	utils.AssertNoErr(err, "unable to connect to db")
	q := dbsqlc.New(db)

	var command string
	var args string
	flag.StringVar(&command, "command", "", fmt.Sprintf("command to execute (%s)", strings.Join(commands, ", ")))
	flag.StringVar(&args, "args", "", "database query arguments")
	flag.Parse()

	switch command {
	case commands[0]:
		fmt.Printf("args %s", args)

		qArgs := new(dbsqlc.InsertAccountParams)
		err := json.Unmarshal([]byte(args), qArgs)
		utils.AssertNoErr(err, "unable to unmarshal args")

		id, err := q.InsertAccount(ctx, qArgs)
		utils.AssertNoErr(err, "unable to insert account")

		fmt.Printf("Inserted new account (id: %d)\n", id)
	case commands[1]:
		qArgs := new(dbsqlc.InsertWalletParams)
		err := json.Unmarshal([]byte(args), qArgs)
		utils.AssertNoErr(err, "unable to unmarshal args")

		id, err := q.InsertWallet(ctx, qArgs)
		utils.AssertNoErr(err, "unable to insert wallet")

		fmt.Printf("Inserted new wallet (id: %d)\n", id)
	case commands[2]:
		_, err := solana.PublicKeyFromBase58(args)
		utils.AssertNoErr(err, "invalid address")

		id, err := q.InsertAddress(ctx, args)
		utils.AssertNoErr(err, "unable to insert address")

		fmt.Printf("Inserted new adress (id: %d)\n", id)
	case commands[3]:
		walletId, err := strconv.ParseInt(args, 10, 32)
		utils.AssertNoErr(err, "invalid wallet id")
		err = q.InsertSyncWalletRequest(ctx, int32(walletId))
		utils.AssertNoErr(err, "unable to insert sync wallet request")

		fmt.Println("Inserted new sync wallet request")
	case commands[4]:

	default:
		fmt.Printf("invalid command: %s (commands: %s)\n", command, strings.Join(commands, ", "))
		os.Exit(1)
	}
}
