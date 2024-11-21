package ixparser

import (
	"encoding/binary"
	"log/slog"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
)

var associatedTokenProgramAddress = solana.SPLAssociatedTokenAccountProgramID.String()

func parseAssociatedTokenIx(
	result *parseResult,
	ix ParsableInstruction,
	walletAddress, signature string,
) {
	data := ix.GetData()
	isCreate := len(data) == 0 || data[0] == 0 || data[0] == 1

	if isCreate {
		// CREATE || CREATE IDEMPOTENT
		result.isKnown = true
		innerIxs := ix.GetInnerInstructions()
		innerIxsLen := len(innerIxs)

		if innerIxsLen == 0 {
			return
		}

		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 3, ErrNotEnoughAccounts)

		from := accounts[0]
		to := accounts[1]

		if innerIxsLen == 4 || innerIxsLen == 6 {
			// create from zero lamports || create with lamports
			// rent index = 1
			createAccountIx := innerIxs[1]
			data := createAccountIx.GetData()[4:]
			lamports := binary.LittleEndian.Uint64(data)

			result.events = append(result.events, &TransferEventData{
				From:    from,
				To:      to,
				IsRent:  true,
				Program: associatedTokenProgramAddress,
				Amount:  lamports,
			})
		}

		owner := accounts[2]
		if owner == walletAddress {
			result.appendAssociatedAccount(&AssociatedAccount{
				Type:    dbsqlc.AssociatedAccountTypeToken,
				Address: to,
			})
		}
	} else {
		// RECOVER NESTED
		slog.Error("unimplemented associated token instruction (recover nested)", "signature", signature)
	}
}
