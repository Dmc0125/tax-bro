package ixparser_test

import (
	"context"
	"tax-bro/pkg/ixparser"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"tax-bro/pkg/walletfetcher"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/stretchr/testify/require"
)

func TestSystemCreateAccount(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	receiver := solana.NewWallet()
	amount := uint64(1_000_000_000)

	ix := system.NewCreateAccountInstruction(
		amount,
		100,
		system.ProgramID,
		wallet.PublicKey(),
		receiver.PublicKey(),
	).Build()
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, []solana.Instruction{ix}, []*solana.Wallet{wallet, receiver})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[0], wallet.PublicKey().String(), txResult.Signature)

	require.Len(t, associatedAccounts, 0)
	event := events[0]
	expectedEvent := ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      receiver.PublicKey().String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, &expectedEvent, event)
}

func TestSystemCreateAccountWithSeed(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	amount := uint64(1_000_000_000)
	seed := "seed"
	base := solana.NewWallet()
	receiver, err := solana.CreateWithSeed(base.PublicKey(), seed, system.ProgramID)
	utils.AssertNoErr(err)

	ix := system.NewCreateAccountWithSeedInstruction(
		base.PublicKey(),
		seed,
		amount,
		100,
		system.ProgramID,
		wallet.PublicKey(),
		receiver,
		base.PublicKey(),
	).Build()
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, []solana.Instruction{ix}, []*solana.Wallet{wallet, base})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[0], wallet.PublicKey().String(), txResult.Signature)

	require.Empty(t, associatedAccounts)
	event := events[0]
	expectedEvent := ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      receiver.String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, &expectedEvent, event)
}

func TestSystemTransfer(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	amount := uint64(1_000_000_000)
	receiver := solana.NewWallet()

	ix := system.NewTransferInstruction(amount, wallet.PublicKey(), receiver.PublicKey()).Build()
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, []solana.Instruction{ix}, []*solana.Wallet{wallet})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[0], wallet.PublicKey().String(), txResult.Signature)

	require.Empty(t, associatedAccounts)
	event := events[0]
	expectedEvent := ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      receiver.PublicKey().String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  false,
	}
	require.Equal(t, &expectedEvent, event)
}

func TestSystemTransferWithSeed(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	amount := uint64(100_000_000)
	seed := "seed"
	base := solana.NewWallet()
	accountWithSeed, err := solana.CreateWithSeed(base.PublicKey(), seed, system.ProgramID)
	utils.AssertNoErr(err)
	receiver := solana.NewWallet().PublicKey()

	ixs := []solana.Instruction{
		system.NewCreateAccountWithSeedInstruction(
			base.PublicKey(),
			seed,
			uint64(1_000_000_000),
			0,
			system.ProgramID,
			wallet.PublicKey(),
			accountWithSeed,
			base.PublicKey(),
		).Build(),
		system.NewTransferWithSeedInstruction(
			amount,
			seed,
			system.ProgramID,
			accountWithSeed,
			base.PublicKey(),
			receiver,
		).Build(),
	}
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, base})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[1], wallet.PublicKey().String(), txResult.Signature)

	require.Empty(t, associatedAccounts)
	event := events[0]
	expectedEvent := ixparser.TransferEventData{
		From:    accountWithSeed.String(),
		To:      receiver.String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  false,
	}
	require.Equal(t, &expectedEvent, event)
}

func TestSystemWithdrawNonceAccount(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	nonceAccount := solana.NewWallet()
	ixs := []solana.Instruction{
		system.NewCreateAccountInstruction(1_000_000_000, 500, system.ProgramID, wallet.PublicKey(), nonceAccount.PublicKey()).Build(),
		system.NewWithdrawNonceAccountInstruction(5_000, nonceAccount.PublicKey(), wallet.PublicKey(), solana.SysVarRecentBlockHashesPubkey, solana.SysVarRentPubkey, wallet.PublicKey()).Build(),
	}
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, nonceAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[1], wallet.PublicKey().String(), txResult.Signature)

	require.Empty(t, associatedAccounts)
	event := events[0]
	expectedEvent := ixparser.CloseAccountEventData{
		Account: nonceAccount.PublicKey().String(),
		To:      wallet.PublicKey().String(),
	}
	require.Equal(t, &expectedEvent, event)
}
