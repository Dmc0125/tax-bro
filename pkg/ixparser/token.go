package ixparser

import (
	"encoding/binary"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

var tokenProgramAddress = token.ProgramID.String()

func parseTokenIx(
	ix ParsableInstruction,
	walletAddress string,
) ([]Event, map[string]*AssociatedAccount) {

	dataWithDiscriminator := ix.GetData()
	utils.Assert(len(dataWithDiscriminator) >= 1, ErrMissingDiscriminator)

	disc := uint8(dataWithDiscriminator[0])
	data := dataWithDiscriminator[1:]

	events := make([]Event, 0)
	associatedAccounts := make(map[string]*AssociatedAccount)

	switch disc {
	case token.Instruction_InitializeAccount:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 3, ErrNotEnoughAccounts)

		owner := accounts[2]
		if owner == walletAddress {
			address := accounts[0]
			associatedAccounts[address] = &AssociatedAccount{
				Type:    dbsqlc.AssociatedAccountTypeToken,
				Address: address,
			}
		}
	case token.Instruction_InitializeAccount3:
		fallthrough
	case token.Instruction_InitializeAccount2:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 1, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 32, ErrInvalidData)
		ownerBytes := data[:32]
		owner := solana.PublicKeyFromBytes(ownerBytes).String()
		if owner == walletAddress {
			address := accounts[0]
			associatedAccounts[address] = &AssociatedAccount{
				Type:    dbsqlc.AssociatedAccountTypeToken,
				Address: address,
			}
		}
	case token.Instruction_MintToChecked:
		fallthrough
	case token.Instruction_MintTo:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		events = append(events, &MintEventData{
			To:     accounts[1],
			Amount: amount,
			Token:  accounts[0],
		})
	case token.Instruction_BurnChecked:
		fallthrough
	case token.Instruction_Burn:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		events = append(events, &BurnEventData{
			From:   accounts[0],
			Amount: amount,
			Token:  accounts[1],
		})
	case token.Instruction_CloseAccount:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		events = append(events, &CloseAccountEventData{
			Account: accounts[0],
			To:      accounts[1],
		})
	case token.Instruction_Transfer:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		events = append(events, &TransferEventData{
			From:    accounts[0],
			To:      accounts[1],
			Amount:  amount,
			Program: tokenProgramAddress,
			IsRent:  false,
		})
	case token.Instruction_TransferChecked:
		accounts := ix.GetAccounts()
		utils.Assert(len(accounts) >= 3, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 8, ErrInvalidData)
		amount := binary.LittleEndian.Uint64(data)

		events = append(events, &TransferEventData{
			From:    accounts[0],
			To:      accounts[2],
			Amount:  amount,
			Program: tokenProgramAddress,
			IsRent:  false,
		})
	}

	return events, associatedAccounts
}
