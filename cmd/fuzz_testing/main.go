package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"sync"
	"syscall"
	"tax-bro/pkg/fuzzer"
	"tax-bro/pkg/logger"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	walletsync "tax-bro/pkg/wallet_sync"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
)

var validatorCmd *exec.Cmd
var ctx, cancel = context.WithCancel(context.Background())

func registerInterrupt() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Print("Killing solana")
	validatorCmd.Process.Kill()
	cancel()
}

func main() {
	_, fp, _, ok := runtime.Caller(0)
	utils.Assert(ok, "unable to get current filename")
	projectDir := path.Join(fp, "../../..")

	var rpcPort uint
	var inlineLogs bool
	var logLevel int
	flag.UintVar(&rpcPort, "rpc-port", uint(testutils.TestingRpcPort), "port of solana rpc")
	flag.BoolVar(&inlineLogs, "inline-logs", false, "if true logs are sent to stdout, otherwise to logfile")
	flag.IntVar(&logLevel, "log-level", -4, "logging level (-4: debug, 0: info, 4: warn, 8: error")
	flag.Parse()

	utils.Assert(rpcPort <= math.MaxUint16, "invalid rpc port")
	utils.Assert(logLevel == -4 || logLevel == 0 || logLevel == 4 || logLevel == 8, "invalid log level")

	timestamp := time.Now().Unix()
	fmt.Printf("Fuzz seed: %d", timestamp)
	currentTestDir := path.Join(projectDir, fmt.Sprintf("fuzz_tests/test_%d", timestamp))
	err := os.MkdirAll(currentTestDir, 0755)
	utils.AssertNoErr(err)

	logPath := path.Join(currentTestDir, "logs.txt")
	logger.NewPrettyLogger(logPath, logLevel)

	validatorCmd = testutils.StartSolanaValidator(currentTestDir, fmt.Sprintf("%d", rpcPort))
	go registerInterrupt()

	migrationsDir := path.Join(projectDir, "db_migrations")
	postgresContainer := testutils.NewPostgresContainer(ctx, migrationsDir)

	rpcClient := rpc.New(fmt.Sprintf("http://localhost:%d", rpcPort))

	walletSyncClient := walletsync.NewClient(ctx, postgresContainer.DB, rpcClient)
	go walletSyncClient.Run()

	r := rand.New(rand.NewSource(timestamp))
	f := fuzzer.Fuzzer{
		Ctx:       ctx,
		RpcClient: rpcClient,
		Db:        postgresContainer.DB,
		R:         r,
		M:         &sync.Mutex{},

		TxsRoundTimeout:        1 * time.Second,
		UserActionRoundTimeout: 5 * time.Second,

		AvailableWallets: map[string]*fuzzer.AvailableWallet{},
		Accounts:         []*fuzzer.FuzzAccount{},
		Mints:            []*fuzzer.TokenMint{},
	}
	go f.Run()

	var ticker *time.Ticker
	if inlineLogs {
		ticker = time.NewTicker(5000 * time.Millisecond)
	} else {
		ticker = time.NewTicker(100 * time.Millisecond)
	}
	for {
		select {
		case <-ctx.Done():
			slog.Info("Fuzz test was interrupted, exiting gracefully")
			return
		case <-ticker.C:
			if !inlineLogs {
				fmt.Print("\033[2J\033[H\n")
			}

			fmt.Println(f.String(timestamp))
		}
	}
}
