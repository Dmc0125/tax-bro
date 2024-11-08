package instructionsparser_test

import (
	"context"
	instructionsparser "tax-bro/pkg/instructions_parser"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"testing"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/test-go/testify/require"
)

// disc 0
func TestATACreate(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)

	ata, _, err := solana.FindProgramAddress([][]byte{
		wallet.PublicKey().Bytes(),
		token.ProgramID.Bytes(),
		mintAccount.PublicKey().Bytes(),
	}, associatedtokenaccount.ProgramID)
	utils.AssertNoErr(err)

	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	ixs = append(ixs, solana.NewInstruction(associatedtokenaccount.ProgramID, []*solana.AccountMeta{
		{IsSigner: true, IsWritable: true, PublicKey: wallet.PublicKey()},
		{IsWritable: true, PublicKey: ata},
		{PublicKey: wallet.PublicKey()},
		{PublicKey: mintAccount.PublicKey()},
		{PublicKey: system.ProgramID},
		{PublicKey: token.ProgramID},
	}, []byte{0}))
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[2]

	associatedAccounts := parser.Parse(executedIx, signature)
	require.Len(t, associatedAccounts, 1)

	require.Len(t, executedIx.Events, 1)
	expectedEvent := &instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, executedIx.Events[0])
}

func TestATACreateWithNoData(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)

	ata, _, err := solana.FindProgramAddress([][]byte{
		wallet.PublicKey().Bytes(),
		token.ProgramID.Bytes(),
		mintAccount.PublicKey().Bytes(),
	}, associatedtokenaccount.ProgramID)
	utils.AssertNoErr(err)

	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	ixs = append(ixs, solana.NewInstruction(associatedtokenaccount.ProgramID, []*solana.AccountMeta{
		{IsSigner: true, IsWritable: true, PublicKey: wallet.PublicKey()},
		{IsWritable: true, PublicKey: ata},
		{PublicKey: wallet.PublicKey()},
		{PublicKey: mintAccount.PublicKey()},
		{PublicKey: system.ProgramID},
		{PublicKey: token.ProgramID},
	}, []byte{}))
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[2]

	associatedAccounts := parser.Parse(executedIx, signature)
	require.Len(t, associatedAccounts, 1)

	require.Len(t, executedIx.Events, 1)
	expectedEvent := &instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, executedIx.Events[0])
}

// create idempotent without balance - disc: 1
func TestATACreateIdempotentWithoutBalance(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)

	ata, _, err := solana.FindProgramAddress([][]byte{
		wallet.PublicKey().Bytes(),
		token.ProgramID.Bytes(),
		mintAccount.PublicKey().Bytes(),
	}, associatedtokenaccount.ProgramID)
	utils.AssertNoErr(err)

	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	ixs = append(ixs, solana.NewInstruction(associatedtokenaccount.ProgramID, []*solana.AccountMeta{
		{IsSigner: true, IsWritable: true, PublicKey: wallet.PublicKey()},
		{IsWritable: true, PublicKey: ata},
		{PublicKey: wallet.PublicKey()},
		{PublicKey: mintAccount.PublicKey()},
		{PublicKey: system.ProgramID},
		{PublicKey: token.ProgramID},
	}, []byte{1}))
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[2]

	associatedAccounts := parser.Parse(executedIx, signature)
	require.Len(t, associatedAccounts, 1)

	require.Len(t, executedIx.Events, 1)
	expectedEvent := &instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, executedIx.Events[0])
}

// create idempotent with not enough balance - disc: 1
func TestATACreateIdempotentWithNotEnoughBalance(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)

	ata, _, err := solana.FindProgramAddress([][]byte{
		wallet.PublicKey().Bytes(),
		token.ProgramID.Bytes(),
		mintAccount.PublicKey().Bytes(),
	}, associatedtokenaccount.ProgramID)
	utils.AssertNoErr(err)

	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	ixs = append(
		ixs,
		system.NewTransferInstruction(1000, wallet.PublicKey(), ata).Build(),
		solana.NewInstruction(associatedtokenaccount.ProgramID, []*solana.AccountMeta{
			{IsSigner: true, IsWritable: true, PublicKey: wallet.PublicKey()},
			{IsWritable: true, PublicKey: ata},
			{PublicKey: wallet.PublicKey()},
			{PublicKey: mintAccount.PublicKey()},
			{PublicKey: system.ProgramID},
			{PublicKey: token.ProgramID},
		}, []byte{1}),
	)
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[3]

	associatedAccounts := parser.Parse(executedIx, signature)
	require.Len(t, associatedAccounts, 1)

	require.Len(t, executedIx.Events, 1)
	expectedEvent := &instructionsparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount - 1000,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, executedIx.Events[0])
}

// create idempotent with more than enough balance - disc: 1
func TestATACreateIdempotentWithEnoughBalance(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)

	ata, _, err := solana.FindProgramAddress([][]byte{
		wallet.PublicKey().Bytes(),
		token.ProgramID.Bytes(),
		mintAccount.PublicKey().Bytes(),
	}, associatedtokenaccount.ProgramID)
	utils.AssertNoErr(err)

	amount, err := rpcClient.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	ixs = append(
		ixs,
		system.NewTransferInstruction(amount+1000, wallet.PublicKey(), ata).Build(),
		solana.NewInstruction(associatedtokenaccount.ProgramID, []*solana.AccountMeta{
			{IsSigner: true, IsWritable: true, PublicKey: wallet.PublicKey()},
			{IsWritable: true, PublicKey: ata},
			{PublicKey: wallet.PublicKey()},
			{PublicKey: mintAccount.PublicKey()},
			{PublicKey: system.ProgramID},
			{PublicKey: token.ProgramID},
		}, []byte{1}),
	)
	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})

	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	executedIx := tx.Ixs[3]

	associatedAccounts := parser.Parse(executedIx, signature)
	require.Len(t, associatedAccounts, 1)

	require.Empty(t, executedIx.Events)
}
