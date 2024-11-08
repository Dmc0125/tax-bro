package instructionsparser_test

import (
	"context"
	instructionsparser "tax-bro/pkg/instructions_parser"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/test-go/testify/require"
)

func TestSystemCreateAccount(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	receiver := solana.NewWallet()
	amount := uint64(1_000_000_000)

	ix := system.NewCreateAccountInstruction(
		amount,
		100,
		system.ProgramID,
		wallet.PublicKey(),
		receiver.PublicKey(),
	).Build()
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, []solana.Instruction{ix}, []*solana.Wallet{wallet, receiver})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[0]
	associatedAccounts := parser.Parse(executedIx, signature)

	require.Len(t, associatedAccounts, 0)

	event := executedIx.Events[0]
	expectedEvent := instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      receiver.PublicKey().String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  true,
	}

	require.Equal(t, instructionsparser.EventTransfer, event.Kind())
	require.Equal(t, &expectedEvent, event)
}

func TestSystemCreateAccountWithSeed(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

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
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, []solana.Instruction{ix}, []*solana.Wallet{wallet, base})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[0]
	associatedAccounts := parser.Parse(executedIx, signature)

	require.Empty(t, associatedAccounts)

	event := executedIx.Events[0]
	expectedEvent := instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      receiver.String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  true,
	}

	require.Equal(t, instructionsparser.EventTransfer, event.Kind())
	require.Equal(t, &expectedEvent, event)
}

func TestSystemTransfer(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	amount := uint64(1_000_000_000)
	receiver := solana.NewWallet()

	ix := system.NewTransferInstruction(amount, wallet.PublicKey(), receiver.PublicKey()).Build()
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, []solana.Instruction{ix}, []*solana.Wallet{wallet})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[0]
	associatedAccounts := parser.Parse(executedIx, signature)

	require.Empty(t, associatedAccounts)

	event := executedIx.Events[0]
	expectedEvent := instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      receiver.PublicKey().String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  false,
	}

	require.Equal(t, instructionsparser.EventTransfer, event.Kind())
	require.Equal(t, &expectedEvent, event)
}

func TestSystemTransferWithSeed(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

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
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, base})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[1]
	associatedAccounts := parser.Parse(executedIx, signature)

	require.Empty(t, associatedAccounts)

	event := executedIx.Events[0]
	expectedEvent := instructionsparser.TransferEventData{
		From:    accountWithSeed.String(),
		To:      receiver.String(),
		Amount:  amount,
		Program: system.ProgramID.String(),
		IsRent:  false,
	}

	require.Equal(t, instructionsparser.EventTransfer, event.Kind())
	require.Equal(t, &expectedEvent, event)
}

func TestSystemWithdrawNonceAccount(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	nonceAccount := solana.NewWallet()
	ixs := []solana.Instruction{
		system.NewCreateAccountInstruction(1_000_000_000, 500, system.ProgramID, wallet.PublicKey(), nonceAccount.PublicKey()).Build(),
		system.NewWithdrawNonceAccountInstruction(5_000, nonceAccount.PublicKey(), wallet.PublicKey(), solana.SysVarRecentBlockHashesPubkey, solana.SysVarRentPubkey, wallet.PublicKey()).Build(),
	}
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, nonceAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[1]
	associatedAccounts := parser.Parse(executedIx, signature)

	require.Empty(t, associatedAccounts)

	event := executedIx.Events[0]
	expectedEvent := instructionsparser.CloseAccountEventData{
		Account: nonceAccount.PublicKey().String(),
		To:      wallet.PublicKey().String(),
	}

	require.Equal(t, instructionsparser.EventCloseAccount, event.Kind())
	require.Equal(t, &expectedEvent, event)
}
