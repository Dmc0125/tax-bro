package instructionsparser

import (
	"encoding/binary"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go/programs/system"
)

var systemProgramAddress = system.ProgramID.String()

func (parser *Parser) parseSystemIx(ix ParsableInstruction) {
	dataWithDiscriminator := ix.GetData()
	utils.Assert(len(dataWithDiscriminator) >= 4, "missing discriminator")

	disc := binary.BigEndian.Uint32(dataWithDiscriminator)
	data := dataWithDiscriminator[4:]

	switch disc {
	case system.Instruction_Transfer:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, "not enough accounts")
		utils.Assert(len(data) >= 8, "invalid data")

		lamports := binary.BigEndian.Uint64(data)
		ix.AppendEvents(&TransferEventData{
			from:    accounts[0],
			to:      accounts[1],
			amount:  lamports,
			program: systemProgramAddress,
			isRent:  false,
		})
	case system.Instruction_TransferWithSeed:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 3, "not enough accounts")
		utils.Assert(len(data) >= 8, "invalid data")

		lamports := binary.BigEndian.Uint64(data)
		ix.AppendEvents(&TransferEventData{
			from:    accounts[0],
			to:      accounts[2],
			amount:  lamports,
			program: systemProgramAddress,
			isRent:  false,
		})
	case system.Instruction_CreateAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, "not enough accounts")
		utils.Assert(len(data) >= 8, "invalid data")

		lamports := binary.BigEndian.Uint64(data)
		ix.AppendEvents(&TransferEventData{
			from:    accounts[0],
			to:      accounts[1],
			amount:  lamports,
			program: systemProgramAddress,
			isRent:  true,
		})
	case system.Instruction_CreateAccountWithSeed:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, "not enough accounts")

		utils.Assert(len(data) >= 40, "invalid data")
		seedLen := binary.BigEndian.Uint32(data[32:])
		seedPadding := binary.BigEndian.Uint32(data[36:])
		utils.Assert(len(data) >= 40+int(seedLen+seedPadding), "invalid data")
		lamports := binary.BigEndian.Uint64(data[36+seedLen+seedPadding:])

		ix.AppendEvents(&TransferEventData{
			from:    accounts[0],
			to:      accounts[1],
			amount:  lamports,
			program: systemProgramAddress,
			isRent:  true,
		})
	case system.Instruction_WithdrawNonceAccount:
		accounts := ix.GetAccountsAddresses()
		utils.Assert(len(accounts) >= 2, "not enough accounts")
		ix.AppendEvents(&CloseAccountEventData{
			account: accounts[0],
			to:      accounts[1],
		})
	default:
		return
	}

}
