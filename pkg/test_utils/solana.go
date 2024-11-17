package testutils

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

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

func InitSolana() (*rpc.Client, *solana.Wallet) {
	rpcPort := utils.GetEnvVar("RPC_PORT")
	rpcUrl := fmt.Sprintf("http://localhost:%s", rpcPort)
	rpcClient := rpc.New(rpcUrl)

	wallet := solana.NewWallet()
	ExecuteAirdrop(context.Background(), rpcClient, wallet.PublicKey())

	return rpcClient, wallet
}

type ExecutedTxResult struct {
	Signature string
	Slot      uint64
	BlockTime time.Time
	Msg       *solana.Message
	Meta      *rpc.TransactionMeta
}

func ExecuteTx(
	ctx context.Context,
	rpcClient *rpc.Client,
	instructions []solana.Instruction,
	signers []*solana.Wallet,
) *ExecutedTxResult {
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
			return &ExecutedTxResult{Signature: sig.String()}
		case <-ticker.C:
			txResult, err := rpcClient.GetTransaction(ctx, sig, getTxOpts)
			if errors.Is(err, rpc.ErrNotFound) {
				continue outer
			}
			utils.AssertNoErr(err)
			decodedTx, err := txResult.Transaction.GetTransaction()
			utils.AssertNoErr(err)
			return &ExecutedTxResult{
				Signature: sig.String(),
				Slot:      txResult.Slot,
				BlockTime: txResult.BlockTime.Time(),
				Msg:       &decodedTx.Message,
				Meta:      txResult.Meta,
			}
		}
	}
}
