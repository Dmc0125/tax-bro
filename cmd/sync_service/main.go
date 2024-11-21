package main

import (
	"context"
	"path"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/logger"
	"tax-bro/pkg/sync"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

func main() {
	err := godotenv.Load()
	utils.AssertNoErr(err)

	projectDir := utils.GetProjectDir()
	logger.NewPrettyLogger(path.Join(projectDir, "sync_service.log"), 0)

	dbUrl := utils.GetEnvVar("DB_URL")
	rpcUrl := utils.GetEnvVar("RPC_URL")

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbUrl)
	utils.AssertNoErr(err, "unable to connect to db")
	queries := dbsqlc.New(db)
	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(rpcUrl, rate.Every(time.Second), 5))

	sync.Run(ctx, db, queries, rpcClient)
}
