package instructionsparser_test

import (
	"context"
	instructionsparser "tax-bro/pkg/instructions_parser"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/test-go/testify/require"
)

func TestTokenInitializeAccount(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

	mintAccount, ixs := testutils.CreateInitMintIxs(rpcClient, wallet)
	_, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, mintAccount})
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

		signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, tokenAccount})
		utils.Assert(tx != nil, "unable to execute tx")
		require.False(t, tx.Err)

		parser := instructionsparser.Parser{
			WalletAddress:      wallet.PublicKey().String(),
			AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
		}

		executedIx := tx.Ixs[1]
		associatedAccounts := parser.Parse(executedIx, signature)

		require.Len(t, associatedAccounts, 1)
		expectedAssociatedAccount := instructionsparser.AssociatedAccount{
			Kind:    0,
			Address: tokenAccount.PublicKey().String(),
		}
		require.Equal(t, &expectedAssociatedAccount, associatedAccounts[0])

		require.Len(t, executedIx.Events, 0)
	}
}

func TestTokenMintBurnClose(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

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

	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, tokenAccount, mintAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	for _, ix := range tx.Ixs[4:] {
		associatedAccounts := parser.Parse(ix, signature)
		require.Empty(t, associatedAccounts)
		require.Len(t, ix.Events, 1)
	}

	mintEvents := []instructionsparser.Event{
		tx.Ixs[4].Events[0],
		tx.Ixs[5].Events[0],
	}
	expectedMintEvent := &instructionsparser.MintEventData{
		To:     tokenAccount.PublicKey().String(),
		Amount: 5,
		Token:  mintAccount.PublicKey().String(),
	}
	for _, e := range mintEvents {
		require.Equal(t, expectedMintEvent, e)
	}

	burnEvents := []instructionsparser.Event{
		tx.Ixs[6].Events[0],
		tx.Ixs[7].Events[0],
	}
	expectedBurnEvent := &instructionsparser.BurnEventData{
		From:   tokenAccount.PublicKey().String(),
		Amount: 5,
		Token:  mintAccount.PublicKey().String(),
	}
	for _, e := range burnEvents {
		require.Equal(t, expectedBurnEvent, e)
	}

	closeEvent := tx.Ixs[8].Events[0]
	expectedCloseEvent := &instructionsparser.CloseAccountEventData{
		Account: tokenAccount.PublicKey().String(),
		To:      wallet.PublicKey().String(),
	}
	require.Equal(t, expectedCloseEvent, closeEvent)
}

func TestTokenTransfer(t *testing.T) {
	t.Parallel()

	wallet, rpcClient := testutils.InitSolana()
	defer testutils.CleanupSolana()

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

	signature, tx := testutils.ExecuteTx(context.Background(), rpcClient, ixs, []*solana.Wallet{wallet, tokenAccount1, tokenAccount2, mintAccount})
	utils.Assert(tx != nil, "unable to execute tx")
	require.False(t, tx.Err)

	parser := instructionsparser.Parser{
		WalletAddress:      wallet.PublicKey().String(),
		AssociatedAccounts: []*instructionsparser.AssociatedAccount{},
	}

	expectedTransferIx := &instructionsparser.TransferEventData{
		From:    tokenAccount1.PublicKey().String(),
		To:      tokenAccount2.PublicKey().String(),
		Amount:  1,
		Program: token.ProgramID.String(),
		IsRent:  false,
	}

	for _, ix := range tx.Ixs[7:] {
		associatedAccounts := parser.Parse(ix, signature)
		require.Empty(t, associatedAccounts)
		require.Len(t, ix.Events, 1)
		require.Equal(t, expectedTransferIx, ix.Events[0])
	}
}
