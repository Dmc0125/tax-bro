package testutils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"slices"
	"sync/atomic"
	"syscall"
	"tax-bro/pkg/utils"
	walletsync "tax-bro/pkg/wallet_sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func getProjectDir() string {
	_, fp, _, ok := runtime.Caller(0)
	utils.Assert(ok, "unable to get current filename")
	return path.Join(fp, "../../..")
}

var TestingRpcPort = uint16(6420)
var TestingRpcPortStr = fmt.Sprintf("%d", TestingRpcPort)
var TestingRpcUrl = fmt.Sprintf("http://localhost:%s", TestingRpcPortStr)

var validatorCmd *exec.Cmd
var count atomic.Uint32
var ctrlcRegistered atomic.Bool

func waitForStart(rpcClient *rpc.Client) {
	startTime := time.Now().Unix()
	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		<-ticker.C
		if startTime+10 < time.Now().Unix() {
			fmt.Printf("unable to start solana-test-validator")
			os.Exit(1)
		}
		_, err := rpcClient.GetVersion(context.Background())
		if err == nil {
			return
		}
	}
}

func initWallet(rpcClient *rpc.Client) *solana.Wallet {
	wallet := solana.NewWallet()

	err := ExecuteAirdrop(context.Background(), rpcClient, wallet.PublicKey())
	utils.AssertNoErr(err, "unable to airdrop")

	return wallet
}

func CleanupSolana() {
	c := count.Load()
	if c > 1 {
		count.Store(c - 1)
		return
	}

	if validatorCmd != nil {
		validatorCmd.Process.Kill()
		validatorCmd = nil
		count.Store(0)
	}
}

func RegisterCtrlC() {
	if ctrlcRegistered.Load() {
		return
	}

	ctrlcRegistered.Store(true)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	count.Store(1)
	CleanupSolana()
	os.Exit(1)
}

func StartSolanaValidator(dir string, rpcPort string) *exec.Cmd {
	testLedgerPath := path.Join(dir, "test-ledger")
	cmd := exec.Command(
		"solana-test-validator",
		"--ledger", testLedgerPath,
		"--rpc-port", rpcPort,
		"--faucet-sol", fmt.Sprintf("%d", uint64(math.MaxUint64)),
		"--reset",
		"--quiet",
	)
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	utils.AssertNoErr(err)

	rpcClient := rpc.New(TestingRpcUrl)
	waitForStart(rpcClient)

	return cmd
}

func InitSolana() (*solana.Wallet, *rpc.Client) {
	count.Add(1)
	if count.Load() > 1 {
		rpcClient := rpc.New(TestingRpcUrl)
		waitForStart(rpcClient)
		wallet := initWallet(rpcClient)
		return wallet, rpcClient
	}

	projectDir := getProjectDir()
	validatorCmd = StartSolanaValidator(projectDir, TestingRpcPortStr)

	go RegisterCtrlC()

	rpcClient := rpc.New(TestingRpcUrl)
	wallet := initWallet(rpcClient)

	return wallet, rpcClient
}

func ExecuteTx(ctx context.Context, rpcClient *rpc.Client, instructions []solana.Instruction, signers []*solana.Wallet) (string, *walletsync.OnchainTransaction) {
	blockhash, err := rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)
	transaction, err := solana.NewTransaction(
		instructions,
		blockhash.Value.Blockhash,
		solana.TransactionPayer(signers[0].PublicKey()),
	)
	utils.AssertNoErr(err)

	transaction.Sign(func(signerAddress solana.PublicKey) *solana.PrivateKey {
		idx := slices.IndexFunc(signers, func(signer *solana.Wallet) bool {
			return signer.PublicKey().Equals(signerAddress)
		})
		utils.Assert(idx > -1, "unable to find signer")
		return &signers[idx].PrivateKey
	})

	sig, err := rpcClient.SendTransactionWithOpts(ctx, transaction, rpc.TransactionOpts{
		SkipPreflight: true,
	})
	utils.AssertNoErr(err, "unable to send transaction")

	v := uint64(0)
	getTxOpts := &rpc.GetTransactionOpts{
		Encoding:                       solana.EncodingBase64,
		Commitment:                     rpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &v,
	}

	ticker := time.NewTicker(100 * time.Millisecond)
outer:
	for {
		select {
		case <-ctx.Done():
			return sig.String(), nil
		case <-ticker.C:
			txResult, err := rpcClient.GetTransaction(ctx, sig, getTxOpts)
			if errors.Is(err, rpc.ErrNotFound) {
				continue outer
			}
			utils.AssertNoErr(err)
			decodedTx, err := txResult.Transaction.GetTransaction()
			utils.AssertNoErr(err)
			msg := &decodedTx.Message
			meta := txResult.Meta
			return sig.String(), walletsync.DecompileOnchainTransaction(sig.String(), txResult.Slot, txResult.BlockTime.Time(), msg, meta)
		}
	}
}

func ExecuteAirdrop(ctx context.Context, rpcClient *rpc.Client, account solana.PublicKey) error {
	sig, err := rpcClient.RequestAirdrop(ctx, account, 1_000_000_000_000_000, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	start := time.Now().Unix()
	ticker := time.NewTicker(100 * time.Millisecond)
outer:
	for {
		if start+10 < time.Now().Unix() {
			return errors.New("unable to confirm airdrop")
		}

		select {
		case <-ctx.Done():
			return errors.New("canceled")
		case <-ticker.C:
			res, err := rpcClient.GetSignatureStatuses(ctx, false, sig)
			if errors.Is(err, rpc.ErrNotFound) {
				continue outer
			}
			if len(res.Value) == 0 || res.Value[0] == nil {
				continue outer
			}
			s := res.Value[0]
			if s.ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
				return nil
			}
		}
	}
}
