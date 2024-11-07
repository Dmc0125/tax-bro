package main

import (
	"context"
	"tax-bro/pkg/logger"
	"tax-bro/pkg/utils"
	walletsync "tax-bro/pkg/wallet_sync"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

func main() {
	err := godotenv.Load()
	utils.AssertNoErr(err)

	logger.NewPrettyLogger("", 0)

	dbUrl := utils.GetEnvVar("DB_URL")
	rpcUrl := utils.GetEnvVar("RPC_URL")

	db, err := sqlx.Connect("postgres", dbUrl)
	utils.AssertNoErr(err, "unable to connect to db")

	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(rpcUrl, rate.Every(time.Second), 5))

	ctx, _ := context.WithCancel(context.Background())
	client := walletsync.NewClient(ctx, db, rpcClient)

	client.Run()
}
