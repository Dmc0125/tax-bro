package testutils

import (
	"context"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func CreateInitMintIxs(rpcClient *rpc.Client, wallet *solana.Wallet) (*solana.Wallet, []solana.Instruction) {
	space := uint64(82)
	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), space, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	mint := solana.NewWallet()
	ixs := []solana.Instruction{
		system.NewCreateAccountInstruction(amount, space, token.ProgramID, wallet.PublicKey(), mint.PublicKey()).Build(),
		token.NewInitializeMint2Instruction(0, wallet.PublicKey(), solana.PublicKey{}, mint.PublicKey()).Build(),
	}
	return mint, ixs
}

func CreateInitTokenAccountIxs(rpcClient *rpc.Client, wallet *solana.Wallet, mint solana.PublicKey) (*solana.Wallet, []solana.Instruction) {
	space := uint64(165)
	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), space, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	tokenAccount := solana.NewWallet()
	ixs := []solana.Instruction{
		system.NewCreateAccountInstruction(amount, space, token.ProgramID, wallet.PublicKey(), tokenAccount.PublicKey()).Build(),
		token.NewInitializeAccount3Instruction(wallet.PublicKey(), tokenAccount.PublicKey(), mint).Build(),
	}
	return tokenAccount, ixs
}
