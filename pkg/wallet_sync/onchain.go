package walletsync

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"math"
	"slices"
	"strings"
	"tax-bro/pkg/database"
	instructionsparser "tax-bro/pkg/instructions_parser"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/lib/pq"
)

type onchainInstructionBase struct {
	programAddress string
	accounts       []string
	data           []byte
}

func (base *onchainInstructionBase) GetProgramAddress() string {
	return base.programAddress
}

func (base *onchainInstructionBase) GetAccountsAddresses() []string {
	return base.accounts
}

func (base *onchainInstructionBase) GetData() []byte {
	return base.data
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

func (base *onchainInstructionBase) intoInsertable(accounts []database.Account) insertableInstructionBase {
	insertableInstructionBase := insertableInstructionBase{
		Data:        base64.StdEncoding.EncodeToString(base.data),
		AccountsIds: make(pq.Int32Array, len(base.accounts)),
	}

	programAccountIdx := slices.IndexFunc(accounts, func(a database.Account) bool {
		return a.Address == base.programAddress
	})
	utils.Assert(programAccountIdx != -1, "unable to find program account idx")
	insertableInstructionBase.ProgramAccountId = accounts[programAccountIdx].Id

	for i, address := range base.accounts {
		idx := slices.IndexFunc(accounts, func(a database.Account) bool {
			return a.Address == address
		})
		if idx > -1 {
			insertableInstructionBase.AccountsIds[i] = accounts[idx].Id
		}
	}

	return insertableInstructionBase
}

type onchainInstruction struct {
	onchainInstructionBase
	innerIxs []*onchainInstructionBase
	Events   []instructionsparser.Event
}

func (oix *onchainInstruction) GetInnerInstructions() []instructionsparser.ParsableInstructionBase {
	pixs := make([]instructionsparser.ParsableInstructionBase, len(oix.innerIxs))
	for i, ix := range oix.innerIxs {
		pixs[i] = ix
	}
	return pixs
}

func (oix *onchainInstruction) AppendEvents(events ...instructionsparser.Event) {
	for _, event := range events {
		oix.Events = append(oix.Events, event)
	}
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

var maxSupportedTransactionVersion = uint64(0)
var getTransactionOpts = rpc.GetTransactionOpts{
	MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
	Commitment:                     rpc.CommitmentConfirmed,
}

func DecompileOnchainTransaction(signature string, slot uint64, blockTime time.Time, msg *solana.Message, meta *rpc.TransactionMeta) *OnchainTransaction {
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
			Events:                 make([]instructionsparser.Event, 0),
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

func fetchOnchainTransaction(ctx context.Context, rpcClient *rpc.Client, signature solana.Signature) *OnchainTransaction {
	txRes, err := CallRpcWithRetries(func() (*rpc.GetTransactionResult, error) {
		return rpcClient.GetTransaction(ctx, signature, &getTransactionOpts)
	}, 5)
	utils.AssertNoErr(err)
	decodedTx, err := txRes.Transaction.GetTransaction()
	utils.AssertNoErr(err)

	msg := &decodedTx.Message
	meta := txRes.Meta

	return DecompileOnchainTransaction(signature.String(), txRes.Slot, txRes.BlockTime.Time(), msg, meta)
}

type insertableTransaction struct {
	SignatureId           int32         `db:"signature_id"`
	AccountsIds           pq.Int32Array `db:"accounts_ids"`
	Timestamp             time.Time
	TimestampGranularized time.Time `db:"timestamp_granularized"`
	Slot                  int64
	Logs                  pq.StringArray
	Err                   bool
	Fee                   int64
}

type insertableInstructionBase struct {
	ProgramAccountId int32         `db:"program_account_id"`
	AccountsIds      pq.Int32Array `db:"accounts_ids"`
	Data             string
}

type insertableInstruction struct {
	SignatureId int32 `db:"signature_id"`
	Index       int16
	insertableInstructionBase
}

type insertableInnerInstruction struct {
	IxIndex int16 `db:"ix_index"`
	insertableInstruction
}

type insertableEvent struct {
	SignatureId int32 `db:"signature_id"`
	IxIndex     int16 `db:"ix_index"`
	Index       int16
	Type        string
	Data        []byte
}

type insertable struct {
	transaction       *insertableTransaction
	instructions      []*insertableInstruction
	innerInstructions []*insertableInnerInstruction
	events            []*insertableEvent
}

func (otx *OnchainTransaction) intoInsertable(signatureId int32, accounts []database.Account) insertable {
	utils.Assert(otx.Slot <= math.MaxInt32, "slot overflow")
	utils.Assert(otx.Fee <= math.MaxInt64, "fee overflow")

	txAccountsIds := make(pq.Int32Array, len(otx.Accounts))
	for i, address := range otx.Accounts {
		idx := slices.IndexFunc(accounts, func(a database.Account) bool {
			return a.Address == address
		})
		if idx > -1 {
			txAccountsIds[i] = accounts[idx].Id
		}
	}

	itx := &insertableTransaction{
		SignatureId:           signatureId,
		AccountsIds:           txAccountsIds,
		Timestamp:             otx.Timestamp,
		TimestampGranularized: otx.TimestampGranularized,
		Slot:                  int64(otx.Slot),
		Logs:                  otx.Logs,
		Err:                   otx.Err,
		Fee:                   int64(otx.Fee),
	}
	// slog.Debug("insertable transaction", "value", itx)

	utils.Assert(len(otx.Ixs) <= math.MaxInt16, "ixs len overflows")
	insertableIxs := make([]*insertableInstruction, 0)
	insertableInnerIxs := []*insertableInnerInstruction{}
	insertableEvents := []*insertableEvent{}

	for i, ix := range otx.Ixs {
		ixIndex := int16(i)
		insertableIxs = append(insertableIxs, &insertableInstruction{
			insertableInstructionBase: ix.intoInsertable(accounts),
			SignatureId:               signatureId,
			Index:                     ixIndex,
		})
		// slog.Debug("insertable instruction", "value", insertableIxs[i])

		for eventIndex, event := range ix.Events {
			ev := &insertableEvent{
				SignatureId: signatureId,
				IxIndex:     ixIndex,
				Index:       int16(eventIndex),
				Data:        []byte(base64.StdEncoding.EncodeToString(event.Serialize(accounts))),
				Type:        event.Kind(),
			}
			insertableEvents = append(insertableEvents, ev)
			// slog.Debug("insertable event", "value", insertableEvents[len(insertableEvents)-1])
		}

		for j, innerIx := range ix.innerIxs {
			utils.Assert(j <= math.MaxInt16, "inner ixs len overflow")
			insertableInnerIxs = append(insertableInnerIxs, &insertableInnerInstruction{
				IxIndex: ixIndex,
				insertableInstruction: insertableInstruction{
					SignatureId:               signatureId,
					Index:                     int16(j),
					insertableInstructionBase: innerIx.intoInsertable(accounts),
				},
			})
			// slog.Debug("insertable inner instruction", "value", insertableInnerIxs[len(insertableInnerIxs)-1])
		}

	}

	return insertable{
		transaction:       itx,
		instructions:      insertableIxs,
		innerInstructions: insertableInnerIxs,
		events:            insertableEvents,
	}
}

type onchainMessage struct {
	associatedAccountAddress string

	txs                []*OnchainTransaction
	associatedAccounts []*instructionsparser.AssociatedAccount
	lastSignature      string
}

func (msg *onchainMessage) addAssociatedAccounts(associatedAccounts []*instructionsparser.AssociatedAccount) {
	for _, aa := range associatedAccounts {
		idx := slices.IndexFunc(msg.associatedAccounts, func(aaa *instructionsparser.AssociatedAccount) bool {
			return aaa.Address == aa.Address
		})
		if idx == -1 {
			msg.associatedAccounts = append(msg.associatedAccounts, aa)
		}
	}
}

func (msg *onchainMessage) intoInsertable(signatures []database.Signature, accounts []database.Account) (
	insertableTransactions []*insertableTransaction,
	insertableInstructions []*insertableInstruction,
	insertableInnerInstructions []*insertableInnerInstruction,
	insertableEvents []*insertableEvent,
) {
	for _, tx := range msg.txs {
		signaturesIdx := slices.IndexFunc(signatures, func(signature database.Signature) bool {
			return signature.Value == tx.Signature
		})
		utils.Assert(signaturesIdx != -1, "unable to find signature")

		insertable := tx.intoInsertable(signatures[signaturesIdx].Id, accounts)
		insertableTransactions = append(insertableTransactions, insertable.transaction)
		insertableInstructions = append(insertableInstructions, insertable.instructions...)
		insertableInnerInstructions = append(insertableInnerInstructions, insertable.innerInstructions...)
		insertableEvents = append(insertableEvents, insertable.events...)
	}
	return
}
