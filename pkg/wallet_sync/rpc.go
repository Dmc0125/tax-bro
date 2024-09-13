package walletsync

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	instructionsparser "tax-bro/pkg/instructions_parser"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func decompileAccounts(msg *solana.Message, meta *rpc.TransactionMeta) []string {
	accounts := make([]string, len(msg.AccountKeys))
	for i := 0; i < len(msg.AccountKeys); i++ {
		accounts[i] = msg.AccountKeys[i].String()
	}
	for i := 0; i < len(meta.LoadedAddresses.Writable); i++ {
		accounts = append(accounts, meta.LoadedAddresses.Writable[i].String())
	}
	for i := 0; i < len(meta.LoadedAddresses.ReadOnly); i++ {
		accounts = append(accounts, meta.LoadedAddresses.ReadOnly[i].String())
	}
	return accounts
}

func parseLogMessage(msg string) string {
	var prefix string

	if strings.HasPrefix(msg, "Program log:") {
		prefix = "Program log: "
	} else if strings.HasPrefix(msg, "Program data:") {
		prefix = "Program data: "
	} else {
		return ""
	}

	logData := strings.Split(msg, prefix)[1]
	if logData == "" {
		return logData
	}

	bytes, err := base64.StdEncoding.DecodeString(logData)
	if err != nil {
		// not base 64
		// skip
		return logData
	}

	return hex.EncodeToString(bytes)
}

type onchainInstructionBase struct {
	programAddress string
	accounts       []string
	data           []byte
}

func (oix *onchainInstructionBase) prepareForInsert(
	insertedAccounts accounts,
) (programAccountId int32, accountsIds []int32, data string, err error) {
	programAccountId, err = insertedAccounts.getId(oix.programAddress)
	for _, a := range oix.accounts {
		aId, err1 := insertedAccounts.getId(a)
		if err1 != nil {
			err = err1
			return
		}
		accountsIds = append(accountsIds, aId)
	}
	data = hex.EncodeToString(oix.data)
	return
}

func (oix *onchainInstructionBase) GetAccountAddress(idx int) string {
	return oix.accounts[idx]
}

func (oix *onchainInstructionBase) GetProgramAddress() string {
	return oix.programAddress
}

func (oix *onchainInstructionBase) GetData() []byte {
	return oix.data
}

func (oix *onchainInstructionBase) setFromCompiled(
	compiled solana.CompiledInstruction,
	txAccounts []string,
) {
	oix.accounts = make([]string, len(compiled.Accounts))
	for i := 0; i < len(compiled.Accounts); i++ {
		accountIdx := compiled.Accounts[i]
		oix.accounts = append(oix.accounts, txAccounts[accountIdx])
	}
	oix.programAddress = txAccounts[compiled.ProgramIDIndex]
	oix.data = compiled.Data
}

type onchainInstruction struct {
	onchainInstructionBase
	InnerIxs []onchainInstructionBase
	events   []instructionsparser.Event
}

type onchainTransaction struct {
	Signature             string
	Err                   bool
	Fee                   uint64
	Timestamp             time.Time
	TimestampGranularized time.Time
	Slot                  uint64
	accounts              []string

	Ixs  []onchainInstruction
	Logs []string
}

var maxSupportedTransactionVersion = uint64(0)
var getTransactionOpts = rpc.GetTransactionOpts{
	MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
	Commitment:                     rpc.CommitmentConfirmed,
}

func fetchTransaction(rpcClient *rpc.Client, signature solana.Signature) onchainTransaction {
	txRes, err := callRpcWithRetries(func() (*rpc.GetTransactionResult, error) {
		return rpcClient.GetTransaction(
			context.Background(),
			signature,
			&getTransactionOpts,
		)
	}, 5)
	utils.Assert(err == nil, fmt.Sprintf("unable to fetch transaction: %s", err))
	decodedTx, err := txRes.Transaction.GetTransaction()
	utils.Assert(err == nil, fmt.Sprintf("unable to decode tx: %s", err))

	msg := &decodedTx.Message
	meta := txRes.Meta

	timestamp := txRes.BlockTime.Time()
	timestampGranularized := timestamp.Truncate(15 * time.Minute)
	accounts := decompileAccounts(msg, meta)

	ixs := make([]onchainInstruction, len(msg.Instructions))
	for i := 0; i < len(msg.Instructions); i++ {
		(&ixs[i]).setFromCompiled(msg.Instructions[i], accounts)
	}

	for i := 0; i < len(meta.InnerInstructions); i++ {
		iix := meta.InnerInstructions[i]
		innerIxs := make([]onchainInstructionBase, len(iix.Instructions))
		for i := 0; i < len(iix.Instructions); i++ {
			(&innerIxs[i]).setFromCompiled(iix.Instructions[i], accounts)
		}
		ixs[iix.Index].InnerIxs = innerIxs
	}

	logs := make([]string, 0)
	for _, msg := range meta.LogMessages {
		parsedLog := parseLogMessage(msg)
		if parsedLog != "" {
			logs = append(logs, parsedLog)
		}
	}

	return onchainTransaction{
		Signature:             signature.String(),
		Ixs:                   ixs,
		Logs:                  logs,
		accounts:              accounts,
		Timestamp:             timestamp,
		TimestampGranularized: timestampGranularized,
		Slot:                  txRes.Slot,
		Err:                   meta.Err != nil,
		Fee:                   meta.Fee,
	}
}

func callRpcWithRetries[T any](c func() (T, error), retries int8) (res T, err error) {
	i := 0

	for {
		if i > int(retries)-1 {
			err = fmt.Errorf("unable to execute rpc: %s", err)
			return
		}

		res, err = c()
		if err == nil {
			return
		}
	}
}
