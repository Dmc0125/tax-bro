package ixparser_test

import (
	"context"
	"tax-bro/pkg/ixparser"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"tax-bro/pkg/walletfetcher"
	"testing"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/require"
)

// disc 0
func TestATACreate(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

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
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	executedIx := tx.Ixs[2]
	events, associatedAccounts := ixparser.ParseInstruction(executedIx, wallet.PublicKey().String(), txResult.Signature)

	require.Len(t, associatedAccounts, 1)

	require.Len(t, events, 1)
	expectedEvent := &ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, events[0])
}

func TestATACreateWithNoData(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

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
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	executedIx := tx.Ixs[2]
	events, associatedAccounts := ixparser.ParseInstruction(executedIx, wallet.PublicKey().String(), txResult.Signature)

	require.Len(t, associatedAccounts, 1)

	require.Len(t, events, 1)
	expectedEvent := &ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, events[0])
}

func TestATACreateIdempotentWithoutBalance(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

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
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	executedIx := tx.Ixs[2]
	events, associatedAccounts := ixparser.ParseInstruction(executedIx, wallet.PublicKey().String(), txResult.Signature)

	require.Len(t, associatedAccounts, 1)

	require.Len(t, events, 1)
	expectedEvent := &ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, events[0])
}

func TestATACreateIdempotentWithNotEnoughBalance(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

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
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	executedIx := tx.Ixs[3]
	events, associatedAccounts := ixparser.ParseInstruction(executedIx, wallet.PublicKey().String(), txResult.Signature)

	require.Len(t, associatedAccounts, 1)

	require.Len(t, events, 1)
	expectedEvent := &ixparser.TransferEventData{
		From:    wallet.PublicKey().String(),
		To:      ata.String(),
		Amount:  amount - 1000,
		Program: associatedtokenaccount.ProgramID.String(),
		IsRent:  true,
	}
	require.Equal(t, expectedEvent, events[0])
}

func TestATACreateIdempotentWithEnoughBalance(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

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
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	executedIx := tx.Ixs[3]
	events, associatedAccounts := ixparser.ParseInstruction(executedIx, wallet.PublicKey().String(), txResult.Signature)

	require.Len(t, associatedAccounts, 1)
	require.Empty(t, events)
}
