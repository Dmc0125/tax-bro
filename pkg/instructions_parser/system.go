package instructionsparser

import (
	"encoding/binary"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go/programs/system"
)

var systemProgramAddress = system.ProgramID.String()

func (parser *Parser) parseSystemIx(ix ParsableInstruction) {
	dataWithDiscriminator := ix.GetData()
	utils.Assert(len(dataWithDiscriminator) >= 4, ErrMissingDiscriminator)

	disc := binary.LittleEndian.Uint32(dataWithDiscriminator)
	data := dataWithDiscriminator[4:]

	switch disc {
	case system.Instruction_Transfer:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)
		utils.Assert(len(data) >= 8, ErrInvalidData)

		lamports := binary.LittleEndian.Uint64(data)
		ix.AppendEvents(&TransferEventData{
			From:    accounts[0],
			To:      accounts[1],
			Amount:  lamports,
			Program: systemProgramAddress,
			IsRent:  false,
		})
	case system.Instruction_TransferWithSeed:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 3, ErrNotEnoughAccounts)
		utils.Assert(len(data) >= 8, ErrInvalidData)

		lamports := binary.LittleEndian.Uint64(data)
		ix.AppendEvents(&TransferEventData{
			From:    accounts[0],
			To:      accounts[2],
			Amount:  lamports,
			Program: systemProgramAddress,
			IsRent:  false,
		})
	case system.Instruction_CreateAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)
		utils.Assert(len(data) >= 8, ErrInvalidData)

		lamports := binary.LittleEndian.Uint64(data)
		ix.AppendEvents(&TransferEventData{
			From:    accounts[0],
			To:      accounts[1],
			Amount:  lamports,
			Program: systemProgramAddress,
			IsRent:  true,
		})
	case system.Instruction_CreateAccountWithSeed:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)

		utils.Assert(len(data) >= 40, ErrInvalidData)
		seedLen := binary.LittleEndian.Uint32(data[32:])
		seedPadding := binary.LittleEndian.Uint32(data[36:])
		utils.Assert(len(data) >= 40+int(seedLen+seedPadding), ErrInvalidData)
		lamports := binary.LittleEndian.Uint64(data[40+seedLen+seedPadding:])

		ix.AppendEvents(&TransferEventData{
			From:    accounts[0],
			To:      accounts[1],
			Amount:  lamports,
			Program: systemProgramAddress,
			IsRent:  true,
		})
	case system.Instruction_WithdrawNonceAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, ErrNotEnoughAccounts)
		ix.AppendEvents(&CloseAccountEventData{
			Account: accounts[0],
			To:      accounts[1],
		})
	default:
		return
	}

}
