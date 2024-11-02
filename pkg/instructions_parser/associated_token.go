package instructionsparser

import (
	"encoding/binary"
	"log/slog"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
)

var associatedTokenProgramAddress = solana.SPLAssociatedTokenAccountProgramID.String()

func (parser *Parser) parseAssociatedTokenIx(ix ParsableInstruction, signature string) (associatedAccounts []*AssociatedAccount) {
	dataWithDiscriminator := ix.GetData()
	utils.Assert(len(dataWithDiscriminator) >= 1, errMissingDiscriminator)
	disc := uint8(dataWithDiscriminator[0])
	utils.Assert(disc < 3, errInvalidData)

	switch disc {
	case 0:
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
	case 2:
		// RECOVER NESTED
		slog.Error("unimplemented associated token instruction (recover nested)", "signature", signature)
	}

	return
}
