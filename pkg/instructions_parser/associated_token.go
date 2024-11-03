package instructionsparser

import (
	"encoding/binary"
	"log/slog"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
)

var associatedTokenProgramAddress = solana.SPLAssociatedTokenAccountProgramID.String()

func (parser *Parser) parseAssociatedTokenIx(ix ParsableInstruction, signature string) (associatedAccounts []*AssociatedAccount) {
	data := ix.GetData()
	isCreate := len(data) == 0 || (data[0] == 0 || data[0] == 1)

	if isCreate {
		// CREATE | CREATE IDEMPOTENT
		innerIxs := ix.GetInnerInstructions()
		innerIxsLen := len(innerIxs)

		if innerIxsLen == 0 {
			return
		}

		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) > 3, errNotEnoughAccounts)

		from := accounts[0]
		to := accounts[1]

		if innerIxsLen == 4 || innerIxsLen == 5 {
			// create from zero lamports || create with lamports
			// rent index = 1
			createAccountIx := innerIxs[1]
			data := createAccountIx.GetData()[4:]
			lamports := binary.BigEndian.Uint64(data)

			ix.AppendEvents(&TransferEventData{
				from:    from,
				to:      to,
				isRent:  true,
				program: associatedTokenProgramAddress,
				amount:  lamports,
			})
		}

		associatedAccounts = append(associatedAccounts, &AssociatedAccount{
			Kind:    TokenAssociatedAccount,
			Address: to,
		})
	} else {
		// RECOVER NESTED
		slog.Error("unimplemented associated token instruction (recover nested)", "signature", signature)
	}

	return
}
