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
	utils.Assert(len(dataWithDiscriminator) >= 1, ErrMissingDiscriminator)

	disc := uint8(dataWithDiscriminator[0])
	data := dataWithDiscriminator[1:]

	switch disc {
	case token.Instruction_InitializeAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 3, ErrNotEnoughAccounts)

		owner := accounts[2]
		if owner == parser.WalletAddress {
			associatedAccounts = append(associatedAccounts, &AssociatedAccount{
				Kind:    TokenAssociatedAccount,
				Address: accounts[0],
			})
		}
	case token.Instruction_InitializeAccount3:
		fallthrough
	case token.Instruction_InitializeAccount2:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 1, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 32, ErrInvalidData)
		ownerBytes := data[:32]
		owner := solana.PublicKeyFromBytes(ownerBytes).String()
		if owner == parser.WalletAddress {
			associatedAccounts = append(associatedAccounts, &AssociatedAccount{
				Kind:    TokenAssociatedAccount,
				Address: accounts[0],
			})
		}
	case token.Instruction_MintToChecked:
		fallthrough
	case token.Instruction_MintTo:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		ix.AppendEvents(&MintEventData{
			To:     accounts[1],
			Amount: amount,
			Token:  accounts[0],
		})
	case token.Instruction_BurnChecked:
		fallthrough
	case token.Instruction_Burn:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		ix.AppendEvents(&BurnEventData{
			From:   accounts[0],
			Amount: amount,
			Token:  accounts[1],
		})
	case token.Instruction_CloseAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		ix.AppendEvents(&CloseAccountEventData{
			Account: accounts[0],
			To:      accounts[1],
		})
	case token.Instruction_Transfer:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		ix.AppendEvents(&TransferEventData{
			From:    accounts[0],
			To:      accounts[1],
			Amount:  amount,
			Program: tokenProgramAddress,
			IsRent:  false,
		})
	case token.Instruction_TransferChecked:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 3, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		ix.AppendEvents(&TransferEventData{
			From:    accounts[0],
			To:      accounts[2],
			Amount:  amount,
			Program: tokenProgramAddress,
			IsRent:  false,
		})
	}

	return
}
