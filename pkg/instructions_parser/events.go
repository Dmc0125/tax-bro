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
	EventTransfer     = "transfer"
	EventMint         = "mint"
	EventBurn         = "burn"
	EventCloseAccount = "close_account"
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
	From    string
	To      string
	Amount  uint64
	Program string
	IsRent  bool
}

func (data *TransferEventData) Kind() string {
	return EventTransfer
}

func (data *TransferEventData) Serialize(accounts database.Accounts) []byte {
	fromId, err := accounts.FindId(data.From)
	utils.AssertNoErr(err)
	toId, err := accounts.FindId(data.To)
	utils.AssertNoErr(err)
	programId, err := accounts.FindId(data.Program)
	utils.AssertNoErr(err)

	// 4 + 4 + 8 + 4 + 1 = 21
	serialized := make([]byte, 21)
	binary.BigEndian.PutUint32(serialized, uint32(fromId))
	binary.BigEndian.PutUint32(serialized[4:], uint32(toId))
	binary.BigEndian.PutUint64(serialized[8:], data.Amount)
	binary.BigEndian.PutUint32(serialized[16:], uint32(programId))
	if data.IsRent {
		serialized[20] = 1
	}

	return serialized
}

type MintEventData struct {
	To     string
	Amount uint64
	Token  string
}

func (data *MintEventData) Kind() string {
	return EventMint
}

func (data *MintEventData) Serialize(accounts database.Accounts) []byte {
	toId, err := accounts.FindId(data.To)
	utils.AssertNoErr(err)
	tokenId, err := accounts.FindId(data.Token)
	utils.AssertNoErr(err)

	// 4 + 8 + 4 = 16
	serialized := make([]byte, 16)
	binary.BigEndian.PutUint32(serialized, uint32(toId))
	binary.BigEndian.PutUint64(serialized[4:], uint64(data.Amount))
	binary.BigEndian.PutUint32(serialized[12:], uint32(tokenId))

	return serialized
}

type BurnEventData struct {
	From   string
	Amount uint64
	Token  string
}

func (data *BurnEventData) Kind() string {
	return EventBurn
}

func (data *BurnEventData) Serialize(accounts database.Accounts) []byte {
	fromId, err := accounts.FindId(data.From)
	utils.AssertNoErr(err)
	tokenId, err := accounts.FindId(data.Token)
	utils.AssertNoErr(err)

	// 4 + 8 + 4 = 16
	serialized := make([]byte, 16)
	binary.BigEndian.PutUint32(serialized, uint32(fromId))
	binary.BigEndian.PutUint64(serialized[4:], uint64(data.Amount))
	binary.BigEndian.PutUint32(serialized[12:], uint32(tokenId))

	return serialized
}

type CloseAccountEventData struct {
	Account string
	To      string
}

func (data *CloseAccountEventData) Kind() string {
	return EventCloseAccount
}

func (data *CloseAccountEventData) Serialize(accounts database.Accounts) []byte {
	accountId, err := accounts.FindId(data.Account)
	utils.AssertNoErr(err)
	toId, err := accounts.FindId(data.To)
	utils.AssertNoErr(err)

	// 4 + 4 = 8
	serialized := make([]byte, 8)
	binary.BigEndian.PutUint32(serialized, uint32(accountId))
	binary.BigEndian.PutUint32(serialized[4:], uint32(toId))

	return serialized
}
