package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"runtime"
	"tax-bro/pkg/logger"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/joho/godotenv"
)

func createCurrentTestDir(testsDir string, timestamp int64) string {
	path := path.Join(testsDir, fmt.Sprintf("test-%d", timestamp))
	err := os.MkdirAll(path, 0755)
	utils.AssertNoErr(err)
	return path
}

func main() {
	err := godotenv.Load(".env.fuzz")
	utils.AssertNoErr(err, "unable to load env variables")

	var inline bool
	var logLevel int
	flag.BoolVar(&inline, "inline", false, "if true logs are displayed in console, otherwise saved in logfile and only stats are shown in console")
	flag.IntVar(&logLevel, "log-level", -4, "logging level (-4: debug, 0: info, 4: warn, 8: error")
	flag.Parse()
	utils.Assert(logLevel == -4 || logLevel == 0 || logLevel == 4 || logLevel == 8, "invalid log level")

	_, filename, _, ok := runtime.Caller(0)
	utils.Assert(ok, "unable to get filename")
	projectDir := path.Join(filename, "../../../")

	timestamp := time.Now().UnixMilli()

	testsDirs := path.Join(projectDir, "./fuzz-tests")
	currentTestDir := createCurrentTestDir(testsDirs, timestamp)

	logPath := ""
	if !inline {
		logPath = path.Join(currentTestDir, "/logs.txt")
	}
	logger.NewPrettyLogger(logPath, logLevel)

	ctx, _ := context.WithCancel(context.Background())

	rpcUrl := utils.GetEnvVar("RPC_URL")
	rpcClient := rpc.New(rpcUrl)

	wallet := solana.NewWallet()
	slog.Info("created wallet", "address", wallet.PublicKey().String())

	fuzzer := NewFuzzer(ctx, wallet, rpcClient, 1*time.Second)

	go fuzzer.Run(100)

	if !inline {
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			<-ticker.C
			fmt.Printf("[2J[1;1H\n")
			fmt.Print(fuzzer.String())
		}
	}
	<-ctx.Done()
}
