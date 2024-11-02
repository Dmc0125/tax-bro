package instructionsparser

import (
	"encoding/binary"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

var tokenProgramAddress = token.ProgramID.String()

func (parser *Parser) parseTokenIx(ix ParsableInstruction) (associatedAccounts []*AssociatedAccount) {
	dataWithDiscriminator := ix.GetData()
	utils.Assert(len(dataWithDiscriminator) >= 1, errMissingDiscriminator)

	disc := uint8(dataWithDiscriminator[0])
	data := dataWithDiscriminator[1:]

	switch disc {
	case token.Instruction_InitializeAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 3, errNotEnoughAccounts)

		owner := accounts[2]
		if owner == parser.walletAddress {
			associatedAccounts = append(associatedAccounts, &AssociatedAccount{
				Kind:    TokenAssociatedAccount,
				Address: accounts[0],
			})
		}
	case token.Instruction_InitializeAccount3:
		fallthrough
	case token.Instruction_InitializeAccount2:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 1, errNotEnoughAccounts)

		utils.Assert(len(data) >= 32, errInvalidData)
		ownerBytes := data[:32]
		owner := solana.PublicKeyFromBytes(ownerBytes).String()
		if owner == parser.walletAddress {
			associatedAccounts = append(associatedAccounts, &AssociatedAccount{
				Kind:    TokenAssociatedAccount,
				Address: accounts[0],
			})
		}
	case token.Instruction_MintToChecked:
		fallthrough
	case token.Instruction_MintTo:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, errNotEnoughAccounts)

		utils.Assert(len(data) >= 8, errInvalidData)
		amount := binary.BigEndian.Uint64(data)

		ix.AppendEvents(&MintEventData{
			to:     accounts[1],
			amount: amount,
			token:  accounts[0],
		})
	case token.Instruction_BurnChecked:
		fallthrough
	case token.Instruction_Burn:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, errNotEnoughAccounts)

		utils.Assert(len(data) >= 8, errInvalidData)
		amount := binary.BigEndian.Uint64(data)

		ix.AppendEvents(&BurnEventData{
			from:   accounts[0],
			amount: amount,
			token:  accounts[1],
		})
	case token.Instruction_CloseAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, errNotEnoughAccounts)

		ix.AppendEvents(&CloseAccountEventData{
			account: accounts[0],
			to:      accounts[1],
		})
	case token.Instruction_Transfer:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, errNotEnoughAccounts)

		utils.Assert(len(data) >= 8, errInvalidData)
		amount := binary.BigEndian.Uint64(data)

		ix.AppendEvents(&TransferEventData{
			from:    accounts[0],
			to:      accounts[1],
			amount:  amount,
			program: tokenProgramAddress,
			isRent:  false,
		})
	case token.Instruction_TransferChecked:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 3, errNotEnoughAccounts)

		utils.Assert(len(data) >= 8, errInvalidData)
		amount := binary.BigEndian.Uint64(data)

		ix.AppendEvents(&TransferEventData{
			from:    accounts[0],
			to:      accounts[2],
			amount:  amount,
			program: tokenProgramAddress,
			isRent:  false,
		})
	}

	return
}
