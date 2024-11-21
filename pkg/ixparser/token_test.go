package ixparser_test

import (
	"context"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/ixparser"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"tax-bro/pkg/walletfetcher"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/stretchr/testify/require"
)

func TestTokenInitializeAccount(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)
	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute init mint")
	require.False(t, tx.Err)

	funcs := []func(solana.PublicKey, solana.PublicKey, solana.PublicKey) solana.Instruction{
		func(tokenAccount, mintAccount, sysvarRent solana.PublicKey) solana.Instruction {
			return token.NewInitializeAccountInstruction(
				tokenAccount,
				mintAccount,
				wallet.PublicKey(),
				solana.SysVarRentPubkey,
			).Build()
		},
		func(tokenAccount, mintAccount, syvarRent solana.PublicKey) solana.Instruction {
			return token.NewInitializeAccount2Instruction(
				wallet.PublicKey(),
				tokenAccount,
				mintAccount,
				syvarRent,
			).Build()
		},
		func(tokenAccount, mintAccount, pk3 solana.PublicKey) solana.Instruction {
			return token.NewInitializeAccount3Instruction(wallet.PublicKey(), tokenAccount, mintAccount).Build()
		},
	}

	for _, f := range funcs {
		tokenAccount := solana.NewWallet()
		ixs := []solana.Instruction{
			system.NewCreateAccountInstruction(5_000_000_000, 165, token.ProgramID, wallet.PublicKey(), tokenAccount.PublicKey()).Build(),
			f(tokenAccount.PublicKey(), mintAccount.PublicKey(), solana.SysVarRentPubkey),
		}

		txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, tokenAccount})
		tx := walletfetcher.DecompileOnchainTransaction(
			txResult.Signature,
			txResult.Slot,
			txResult.BlockTime,
			txResult.Msg,
			txResult.Meta,
		)
		utils.Assert(tx != nil, "unable to execute tx")
		require.False(t, tx.Err)

		_, events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[1], wallet.PublicKey().String(), txResult.Signature)

		require.Len(t, associatedAccounts, 1)
		tokenAccountAddress := tokenAccount.PublicKey().String()
		expectedAssociatedAccount := ixparser.AssociatedAccount{
			Type:    dbsqlc.AssociatedAccountTypeToken,
			Address: tokenAccountAddress,
		}
		require.Equal(t, &expectedAssociatedAccount, associatedAccounts[tokenAccountAddress])

		require.Len(t, events, 0)
	}
}

func TestTokenMintBurnClose(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)
	tokenAccount, initTAIxs := testutils.CreateInitTokenAccountIxs(rpcClient, wallet, mintAccount.PublicKey())

	ixs = append(ixs, initTAIxs...)
	ixs = append(
		ixs,
		token.NewMintToInstruction(5, mintAccount.PublicKey(), tokenAccount.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
		token.NewMintToCheckedInstruction(5, 0, mintAccount.PublicKey(), tokenAccount.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
		token.NewBurnInstruction(5, tokenAccount.PublicKey(), mintAccount.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
		token.NewBurnCheckedInstruction(5, 0, tokenAccount.PublicKey(), mintAccount.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
		token.NewCloseAccountInstruction(tokenAccount.PublicKey(), wallet.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
	)

	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, tokenAccount, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	mintEvents := []ixparser.Event{}

	for _, ix := range tx.Ixs[4:6] {
		_, events, associatedAccounts := ixparser.ParseInstruction(ix, wallet.PublicKey().String(), txResult.Signature)
		require.Empty(t, associatedAccounts)
		require.Len(t, events, 1)
		mintEvents = append(mintEvents, events...)
	}
	expectedMintEvent := &ixparser.MintEventData{
		To:     tokenAccount.PublicKey().String(),
		Amount: 5,
		Token:  mintAccount.PublicKey().String(),
	}
	for _, e := range mintEvents {
		require.Equal(t, expectedMintEvent, e)
	}

	burnEvents := []ixparser.Event{}
	for _, ix := range tx.Ixs[6:8] {
		_, events, associatedAccounts := ixparser.ParseInstruction(ix, wallet.PublicKey().String(), txResult.Signature)
		require.Empty(t, associatedAccounts)
		require.Len(t, events, 1)
		burnEvents = append(burnEvents, events...)
	}
	expectedBurnEvent := &ixparser.BurnEventData{
		From:   tokenAccount.PublicKey().String(),
		Amount: 5,
		Token:  mintAccount.PublicKey().String(),
	}
	for _, e := range burnEvents {
		require.Equal(t, expectedBurnEvent, e)
	}

	_, events, associatedAccounts := ixparser.ParseInstruction(tx.Ixs[8], wallet.PublicKey().String(), txResult.Signature)
	require.Len(t, associatedAccounts, 0)
	closeEvent := events[0]
	expectedCloseEvent := &ixparser.CloseAccountEventData{
		Account: tokenAccount.PublicKey().String(),
		To:      wallet.PublicKey().String(),
	}
	require.Equal(t, expectedCloseEvent, closeEvent)
}

func TestTokenTransfer(t *testing.T) {
	t.Parallel()

	rpcClient, wallet := testutils.InitSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)
	tokenAccount1, initTA1Ixs := testutils.CreateInitTokenAccountIxs(rpcClient, wallet, mintAccount.PublicKey())
	tokenAccount2, initTAI2xs := testutils.CreateInitTokenAccountIxs(rpcClient, wallet, mintAccount.PublicKey())

	ixs = append(ixs, initTA1Ixs...)
	ixs = append(ixs, initTAI2xs...)
	ixs = append(
		ixs,
		token.NewMintToCheckedInstruction(5, 0, mintAccount.PublicKey(), tokenAccount1.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
		token.NewTransferInstruction(1, tokenAccount1.PublicKey(), tokenAccount2.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
		token.NewTransferCheckedInstruction(1, 0, tokenAccount1.PublicKey(), mintAccount.PublicKey(), tokenAccount2.PublicKey(), wallet.PublicKey(), []solana.PublicKey{}).Build(),
	)

	txResult := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, tokenAccount1, tokenAccount2, mintAccount})
	tx := walletfetcher.DecompileOnchainTransaction(
		txResult.Signature,
		txResult.Slot,
		txResult.BlockTime,
		txResult.Msg,
		txResult.Meta,
	)
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	expectedTransferEvent := &ixparser.TransferEventData{
		From:    tokenAccount1.PublicKey().String(),
		To:      tokenAccount2.PublicKey().String(),
		Amount:  1,
		Program: token.ProgramID.String(),
		IsRent:  false,
	}

	for _, ix := range tx.Ixs[7:] {
		_, events, associatedAccounts := ixparser.ParseInstruction(ix, wallet.PublicKey().String(), txResult.Signature)
		require.Empty(t, associatedAccounts)
		require.Len(t, events, 1)
		require.Equal(t, expectedTransferEvent, events[0])
	}
}
