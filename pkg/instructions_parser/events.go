package instructionsparser

import (
	"encoding/binary"
	"log/slog"
	"os"
	"tax-bro/pkg/database"
	"tax-bro/pkg/logger"
	"tax-bro/pkg/utils"
)

const (
	eventTransfer     = "transfer"
	eventMint         = "mint"
	eventBurn         = "burn"
	eventCloseAccount = "close_account"
)

type EventKind int8

func (ek EventKind) String() string {
	switch ek {
	case 0:
		return "transfer"
	}
	slog.Error("invalid event kind", "value", ek)
	logger.PrintStackTrace()
	os.Exit(1)
	return ""
}

type TransferEventData struct {
	from    string
	to      string
	amount  uint64
	program string
	isRent  bool
}

func (data *TransferEventData) Kind() string {
	return eventTransfer
}

func (data *TransferEventData) Serialize(accounts database.Accounts) []byte {
	fromId, err := accounts.FindId(data.from)
	utils.AssertNoErr(err)
	toId, err := accounts.FindId(data.to)
	utils.AssertNoErr(err)
	programId, err := accounts.FindId(data.program)
	utils.AssertNoErr(err)

	// 4 + 4 + 8 + 4 + 1 = 21
	serialized := make([]byte, 21)
	binary.BigEndian.PutUint32(serialized, uint32(fromId))
	binary.BigEndian.PutUint32(serialized[4:], uint32(toId))
	binary.BigEndian.PutUint64(serialized[8:], data.amount)
	binary.BigEndian.PutUint32(serialized[16:], uint32(programId))
	if data.isRent {
		serialized[20] = 1
	}

	return serialized
}

type MintEventData struct {
	to     string
	amount uint64
	token  string
}

func (data *MintEventData) Kind() string {
	return eventMint
}

func (data *MintEventData) Serialize(accounts database.Accounts) []byte {
	toId, err := accounts.FindId(data.to)
	utils.AssertNoErr(err)
	tokenId, err := accounts.FindId(data.token)
	utils.AssertNoErr(err)

	// 4 + 8 + 4 = 16
	serialized := make([]byte, 16)
	binary.BigEndian.PutUint32(serialized, uint32(toId))
	binary.BigEndian.PutUint64(serialized[4:], uint64(data.amount))
	binary.BigEndian.PutUint32(serialized[12:], uint32(tokenId))

	return serialized
}

type BurnEventData struct {
	from   string
	amount uint64
	token  string
}

func (data *BurnEventData) Kind() string {
	return eventBurn
}

func (data *BurnEventData) Serialize(accounts database.Accounts) []byte {
	fromId, err := accounts.FindId(data.from)
	utils.AssertNoErr(err)
	tokenId, err := accounts.FindId(data.token)
	utils.AssertNoErr(err)

	// 4 + 8 + 4 = 16
	serialized := make([]byte, 16)
	binary.BigEndian.PutUint32(serialized, uint32(fromId))
	binary.BigEndian.PutUint64(serialized[4:], uint64(data.amount))
	binary.BigEndian.PutUint32(serialized[12:], uint32(tokenId))

	return serialized
}

type CloseAccountEventData struct {
	account string
	to      string
}

func (data *CloseAccountEventData) Kind() string {
	return eventCloseAccount
}

func (data *CloseAccountEventData) Serialize(accounts database.Accounts) []byte {
	accountId, err := accounts.FindId(data.account)
	utils.AssertNoErr(err)
	toId, err := accounts.FindId(data.to)
	utils.AssertNoErr(err)

	// 4 + 4 = 8
	serialized := make([]byte, 8)
	binary.BigEndian.PutUint32(serialized, uint32(accountId))
	binary.BigEndian.PutUint32(serialized[4:], uint32(toId))

	return serialized
}
