package walletfetcher

import (
	"encoding/base64"
	"encoding/hex"
	"slices"
	"strings"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/ixparser"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgtype"
)

type onchainInstructionBase struct {
	programAddress string
	accounts       []string
	data           []byte
}

func (ix *onchainInstructionBase) GetProgramAddress() string {
	return ix.programAddress
}

func (ix *onchainInstructionBase) GetAccounts() []string {
	return ix.accounts
}

func (ix *onchainInstructionBase) GetData() []byte {
	return ix.data
}

func findAccountIdFromAddress(addresses []*dbsqlc.Address, address string) int32 {
	idx := slices.IndexFunc(addresses, func(a *dbsqlc.Address) bool {
		return a.Value == address
	})
	utils.Assert(idx > -1, "unable to find account")
	return addresses[idx].ID
}

func (ix *onchainInstructionBase) intoParams(insertedAddresses []*dbsqlc.Address) (int32, []int32, string) {
	programAddressId := findAccountIdFromAddress(insertedAddresses, ix.programAddress)
	accountsIds := make([]int32, len(ix.accounts))
	for i, account := range ix.accounts {
		accountsIds[i] = findAccountIdFromAddress(insertedAddresses, account)
	}
	data := base64.RawStdEncoding.EncodeToString(ix.data)
	return programAddressId, accountsIds, data
}

type onchainInstruction struct {
	onchainInstructionBase
	innerIxs []*onchainInstructionBase
	events   []ixparser.Event
}

func (ix *onchainInstruction) GetInnerInstructions() (parsableInnerIxs []ixparser.ParsableInstructionBase) {
	for _, innerIx := range ix.innerIxs {
		parsableInnerIxs = append(parsableInnerIxs, innerIx)
	}
	return
}

type OnchainTransaction struct {
	Signature             string
	Accounts              []string
	Logs                  []string
	Ixs                   []*onchainInstruction
	Timestamp             time.Time
	TimestampGranularized time.Time
	Slot                  uint64
	Err                   bool
	Fee                   uint64
}

func (otx *OnchainTransaction) decompileAccounts(msg *solana.Message, meta *rpc.TransactionMeta) {
	for i := 0; i < len(msg.AccountKeys); i++ {
		otx.Accounts = append(otx.Accounts, msg.AccountKeys[i].String())
	}
	for i := 0; i < len(meta.LoadedAddresses.Writable); i++ {
		otx.Accounts = append(otx.Accounts, meta.LoadedAddresses.Writable[i].String())
	}
	for i := 0; i < len(meta.LoadedAddresses.ReadOnly); i++ {
		otx.Accounts = append(otx.Accounts, meta.LoadedAddresses.ReadOnly[i].String())
	}
}

func (otx *OnchainTransaction) parseLogs(meta *rpc.TransactionMeta) {
	otx.Logs = []string{}

	for _, msg := range meta.LogMessages {
		var prefix string

		if strings.HasPrefix(msg, "Program log:") {
			prefix = "Program log: "
		} else if strings.HasPrefix(msg, "Program data:") {
			prefix = "Program data: "
		} else {
			continue
		}

		logData := strings.Split(msg, prefix)[1]
		if logData == "" {
			continue
		}

		bytes, err := base64.StdEncoding.DecodeString(logData)
		if err != nil {
			continue
		}

		if parsedLog := hex.EncodeToString(bytes); parsedLog != "" {
			otx.Logs = append(otx.Logs, parsedLog)
		}
	}
}

func (tx *OnchainTransaction) intoParams(
	signatureId int32,
	insertedAddresses []*dbsqlc.Address,
) *dbsqlc.InsertTransactionsParams {
	params := new(dbsqlc.InsertTransactionsParams)
	params.SignatureID = signatureId

	params.AccountsIds = make([]int32, len(tx.Accounts))
	for i, account := range tx.Accounts {
		idx := slices.IndexFunc(insertedAddresses, func(a *dbsqlc.Address) bool {
			return a.Value == account
		})
		utils.Assert(idx > -1, "unable to find account")
		params.AccountsIds[i] = insertedAddresses[idx].ID
	}

	params.Timestamp = pgtype.Timestamptz{Time: tx.Timestamp, Valid: true}
	params.TimestampGranularized = pgtype.Timestamptz{Time: tx.TimestampGranularized, Valid: true}
	params.Slot = int64(tx.Slot)
	params.Err = tx.Err
	params.Logs = tx.Logs
	params.Fee = int64(tx.Fee)

	return params
}

func decompileInstruction(
	compiled solana.CompiledInstruction,
	txAccounts []string,
) *onchainInstructionBase {
	decompiled := new(onchainInstructionBase)
	decompiled.accounts = make([]string, len(compiled.Accounts))
	for i := 0; i < len(compiled.Accounts); i++ {
		accountIdx := compiled.Accounts[i]
		decompiled.accounts[i] = txAccounts[accountIdx]
	}
	decompiled.programAddress = txAccounts[compiled.ProgramIDIndex]
	decompiled.data = compiled.Data
	return decompiled
}

func DecompileOnchainTransaction(
	signature string,
	slot uint64,
	blockTime time.Time,
	msg *solana.Message,
	meta *rpc.TransactionMeta,
) *OnchainTransaction {
	otx := new(OnchainTransaction)
	otx.decompileAccounts(msg, meta)
	otx.parseLogs(meta)

	otx.Signature = signature
	otx.Slot = slot
	otx.Err = meta.Err != nil
	otx.Timestamp = blockTime
	otx.TimestampGranularized = otx.Timestamp.Truncate(15 * time.Minute)

	for i := 0; i < len(msg.Instructions); i++ {
		ix := &onchainInstruction{
			onchainInstructionBase: *decompileInstruction(msg.Instructions[i], otx.Accounts),
			innerIxs:               make([]*onchainInstructionBase, 0),
			events:                 make([]ixparser.Event, 0),
		}
		otx.Ixs = append(otx.Ixs, ix)
	}

	for i := 0; i < len(meta.InnerInstructions); i++ {
		iix := &meta.InnerInstructions[i]
		ixIndex := iix.Index
		for j := 0; j < len(iix.Instructions); j++ {
			decompiledInnerIx := decompileInstruction(iix.Instructions[j], otx.Accounts)
			otx.Ixs[ixIndex].innerIxs = append(otx.Ixs[ixIndex].innerIxs, decompiledInnerIx)
		}
	}

	return otx
}
