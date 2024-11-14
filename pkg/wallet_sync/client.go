package walletsync

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"tax-bro/pkg/database"
	instructionsparser "tax-bro/pkg/instructions_parser"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const selectSyncWalletRequestQuery = `
SELECT
	sync_wallet_request.wallet_id,
	address.value AS address,
	signature.value AS last_signature
FROM
	sync_wallet_request
INNER JOIN
	wallet ON wallet.id = sync_wallet_request.wallet_id
INNER JOIN
	address ON address.id = wallet.address_id
LEFT JOIN
	signature ON signature.id = wallet.last_signature_id
WHERE
	sync_wallet_request.status = 'queued'
ORDER BY
	sync_wallet_request.created_at ASC
LIMIT
	1
`

type syncWalletRequest struct {
	WalletId      int32 `db:"wallet_id"`
	Address       string
	LastSignature sql.NullString `db:"last_signature"`
}

func fetchSyncWalletRequest(db *sqlx.DB) (syncWalletRequest, error) {
	req := syncWalletRequest{}
	err := db.Get(&req, selectSyncWalletRequestQuery)
	return req, err
}

func (req *syncWalletRequest) IsWallet() bool {
	return true
}

func (req *syncWalletRequest) watchDelete(ctx context.Context, db *sqlx.DB, interruptChan chan struct{}) {
	const q = "SELECT id FROM wallet WHERE id = $1"
	w := struct{ Id int32 }{}
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			slog.Info("context done, exiting req.watchDelete")
			return
		case <-ticker.C:
			err := db.Get(&w, q, req.WalletId)
			if errors.Is(err, sql.ErrNoRows) {
				slog.Info("sync_wallet_request was deleted, sending interrupt msg", "process", "watchDelete")
				interruptChan <- struct{}{}
				return
			}
			utils.AssertNoErr(err)
		}
	}
}

func (req *syncWalletRequest) GetFetchSignaturesConfig() (solana.PublicKey, *rpc.GetSignaturesForAddressOpts) {
	pubkey, err := solana.PublicKeyFromBase58(req.Address)
	utils.AssertNoErr(err)

	limit := int(1000)
	getSignaturesOpts := rpc.GetSignaturesForAddressOpts{
		Commitment: rpc.CommitmentConfirmed,
		Limit:      &limit,
	}
	if req.LastSignature.Valid {
		sig, err := solana.SignatureFromBase58(req.LastSignature.String)
		utils.AssertNoErr(err)
		getSignaturesOpts.Until = sig
	}

	return pubkey, &getSignaturesOpts
}

type Client struct {
	ctx       context.Context
	db        *sqlx.DB
	rpcClient *rpc.Client
}

func NewClient(ctx context.Context, db *sqlx.DB, rpcClient *rpc.Client) *Client {
	return &Client{
		ctx,
		db,
		rpcClient,
	}
}

func CallRpcWithRetries[T any](c func() (T, error), retries int8) (res T, err error) {
	i := 0
	for {
		if i > int(retries)-1 {
			err = fmt.Errorf("unable to execute rpc call: %s", err)
			return
		}
		res, err = c()
		if err == nil {
			return
		}
	}
}

func selectAccountsCoalesce(tableName string) string {
	return fmt.Sprintf(
		`(
			SELECT coalesce(json_agg(agg), '[]') FROM (
				SELECT
					address.id,
					address.value AS address,
					array_position(%s.accounts_ids, id) AS ord
				FROM
					address
				WHERE
					address.id = ANY(%s.accounts_ids)
				ORDER BY
					ord ASC
			) AS agg
		)`,
		tableName,
		tableName,
	)
}

var fetchTransactionQuery = fmt.Sprintf(
	`SELECT
		(
			SELECT coalesce(json_agg(agg), '[]') FROM (
				SELECT
					address.value as program_address,
					instruction.data,
					%s AS accounts,
					(
						SELECT coalesce(json_agg(agg), '[]') FROM (
							SELECT
								address.value AS program_address,
								%s AS accounts,
								inner_instruction.data
							FROM
								inner_instruction
							INNER JOIN
								address ON address.id = inner_instruction.program_account_id
							WHERE
								inner_instruction.signature_id = instruction.signature_id
								AND inner_instruction.ix_index = instruction.index
							ORDER BY
								inner_instruction.index ASC
						) AS agg
					) AS inner_ixs
				FROM
					instruction
				INNER JOIN
					address ON address.id = instruction.program_account_id
				WHERE
					instruction.signature_id = signature.id
				ORDER BY
					instruction.index ASC
			) AS agg
		) AS ixs,
		transaction.logs,
		transaction.err,
		signature.value as signature,
		signature.id as signature_id
	FROM
		signature
	INNER JOIN
		transaction ON transaction.signature_id = signature.id
	WHERE
		signature.value = ANY($1)
	`,
	selectAccountsCoalesce("instruction"),
	selectAccountsCoalesce("inner_instruction"),
)

type savedInstructionBase struct {
	ProgramAddress string `json:"program_address"`
	Accounts       []database.Account
	Data           string
}

func (ix *savedInstructionBase) GetProgramAddress() string {
	return ix.ProgramAddress
}

func (ix *savedInstructionBase) GetAccountsAddresses() []string {
	addresses := make([]string, len(ix.Accounts))
	for i, account := range ix.Accounts {
		addresses[i] = account.Address
	}
	return addresses
}

func (ix *savedInstructionBase) GetData() []byte {
	bytes, err := base64.StdEncoding.DecodeString(ix.Data)
	utils.AssertNoErr(err)
	return bytes
}

type savedInstruction struct {
	savedInstructionBase
	InnerIxs []*savedInstructionBase `json:"inner_ixs"`
}

func (oix *savedInstruction) GetInnerInstructions() []instructionsparser.ParsableInstructionBase {
	pixs := make([]instructionsparser.ParsableInstructionBase, len(oix.InnerIxs))
	for i, ix := range oix.InnerIxs {
		pixs[i] = ix
	}
	return pixs
}

func (ix *savedInstruction) AppendEvents(events ...instructionsparser.Event) {}

func (ix *savedInstruction) intoParsable() (*savedInstruction, []*savedInstructionBase) {
	innerIxs := []*savedInstructionBase{}
	for i := 0; i < len(ix.InnerIxs); i += 1 {
		innerIxs = append(innerIxs, ix.InnerIxs[i])
	}
	return ix, innerIxs
}

type savedTransaction struct {
	SignatureId int32 `db:"signature_id"`
	Signature   string
	Logs        pq.StringArray
	Ixs         []*savedInstruction
	Err         bool
}

func fetchSavedTransactions(db *sqlx.DB, signatures []string) []*savedTransaction {
	savedTransactions := []struct {
		savedTransaction
		Ixs []byte
	}{}
	err := db.Select(&savedTransactions, fetchTransactionQuery, pq.StringArray(signatures))
	utils.AssertNoErr(err)

	txs := make([]*savedTransaction, len(savedTransactions))
	for i := 0; i < len(savedTransactions); i += 1 {
		tx := &savedTransactions[i]

		ixs := []*savedInstruction{}
		err := json.Unmarshal(tx.Ixs, &ixs)
		utils.AssertNoErr(err)

		txs[i] = &savedTransaction{
			Signature:   tx.Signature,
			SignatureId: tx.SignatureId,
			Logs:        tx.Logs,
			Err:         tx.Err,
			Ixs:         ixs,
		}
	}

	return txs
}

type dedupedSignatures struct {
	saved   []*savedTransaction
	unsaved []solana.Signature
}

func (c *Client) dedupSavedSignatures(sigsChunk []*rpc.TransactionSignature) dedupedSignatures {
	signatures := []string{}
	for _, s := range sigsChunk {
		signatures = append(signatures, s.Signature.String())
	}

	savedTransactions := fetchSavedTransactions(c.db, signatures)
	slog.Debug("fetched saved transactions", "len", len(savedTransactions))

	ds := dedupedSignatures{}
	for _, s := range sigsChunk {
		idx := slices.IndexFunc(savedTransactions, func(tx *savedTransaction) bool {
			return tx.Signature == s.Signature.String()
		})
		if idx == -1 {
			ds.unsaved = append(ds.unsaved, s.Signature)
		} else {
			ds.saved = append(ds.saved, savedTransactions[idx])
		}
	}

	return ds
}

type savedMessage struct {
	associatedAccountAddress string

	signaturesIds      []int32
	associatedAccounts []*instructionsparser.AssociatedAccount
	lastSignature      string
}

func newSavedMessage(txs []*savedTransaction, parser *instructionsparser.Parser, isWallet bool) *savedMessage {
	msg := &savedMessage{
		signaturesIds:      make([]int32, len(txs)),
		associatedAccounts: make([]*instructionsparser.AssociatedAccount, 0),
	}

	for i, tx := range txs {
		msg.signaturesIds[i] = tx.SignatureId

		if tx.Err {
			continue
		}

		for _, ix := range tx.Ixs {
			// parsableIx, parsableInnerIxs := ix.intoParsable()
			associatedAccounts := parser.Parse(ix, tx.Signature)

			if !isWallet {
				continue
			}

			for _, aa := range associatedAccounts {
				idx := slices.IndexFunc(msg.associatedAccounts, func(a *instructionsparser.AssociatedAccount) bool {
					return a.Address == aa.Address
				})
				if idx == -1 {
					msg.associatedAccounts = append(msg.associatedAccounts, aa)
				}
			}
		}
	}

	return msg
}

type unsyncedAddress interface {
	GetFetchSignaturesConfig() (solana.PublicKey, *rpc.GetSignaturesForAddressOpts)
	IsWallet() bool
}

func (c *Client) fetchAndParseTransactions(
	ctx context.Context,
	req unsyncedAddress,
	parser *instructionsparser.Parser,
	msgChan chan interface{},
) {
	pubkey, getSignaturesOpts := req.GetFetchSignaturesConfig()
	newLastSignature := ""

	for {
		select {
		case <-ctx.Done():
			return
		default:
			slog.Info("fetching signatures", "limit", getSignaturesOpts.Limit, "until", getSignaturesOpts.Until, "before", getSignaturesOpts.Before)
			signatures, err := CallRpcWithRetries(func() ([]*rpc.TransactionSignature, error) {
				return c.rpcClient.GetSignaturesForAddressWithOpts(c.ctx, pubkey, getSignaturesOpts)
			}, 5)
			utils.AssertNoErr(err, "unable to fetch signatures")

			if len(signatures) == 0 {
				slog.Info("signatures empty", "newLastSignature", newLastSignature)
				if newLastSignature != "" {
					slog.Info("sending lastSignature msg")
					msgChan <- &savedMessage{
						lastSignature: newLastSignature,
					}
				}
				return
			}

			getSignaturesOpts.Before = signatures[len(signatures)-1].Signature
			if newLastSignature == "" {
				newLastSignature = signatures[0].Signature.String()
				slog.Info("setting last signature", "newLastSignature", newLastSignature)
			}

			isLastIter := len(signatures) < *getSignaturesOpts.Limit
			chunk := []*rpc.TransactionSignature{}

			for i, signature := range signatures {
				isLastChunk := i == len(signatures)-1
				chunk = append(chunk, signature)

				if len(chunk) == 100 || isLastChunk {
					dedupedSignatures := c.dedupSavedSignatures(chunk)

					if len(dedupedSignatures.saved) > 0 {
						msg := newSavedMessage(dedupedSignatures.saved, parser, req.IsWallet())
						if isLastChunk && isLastIter && newLastSignature != "" {
							msg.lastSignature = newLastSignature
						}
						slog.Info("sending saved msg", "isLast", isLastIter, "lastSignature", msg.lastSignature)
						msgChan <- msg
					}

					if len(dedupedSignatures.unsaved) > 0 {
						slog.Info("creating onchain message", "txsLen", len(dedupedSignatures.unsaved))
						msg := &onchainMessage{
							txs: make([]*OnchainTransaction, len(dedupedSignatures.unsaved)),
						}
						if isLastIter && isLastChunk && len(newLastSignature) > 0 {
							msg.lastSignature = newLastSignature
						}
						for ixIndex, signature := range dedupedSignatures.unsaved {
							select {
							case <-ctx.Done():
								slog.Info("context done, exiting fetchOnchainTransaction")
								return
							default:
								otx := fetchOnchainTransaction(ctx, c.rpcClient, signature)
								if !otx.Err {
									for _, ix := range otx.Ixs {
										associatedAccounts := parser.Parse(ix, otx.Signature)
										if req.IsWallet() {
											msg.addAssociatedAccounts(associatedAccounts)
										}
									}
								}
								msg.txs[ixIndex] = otx
							}
						}
						slog.Info(
							"sending onchain msg",
							"isLastMsg", isLastIter && isLastChunk,
							"lastSignature", msg.lastSignature,
							"txsLen", len(msg.txs),
							"associatedAccountsLen", len(msg.associatedAccounts),
						)
						msgChan <- msg
					}

					chunk = []*rpc.TransactionSignature{}
				}
			}

			if isLastIter {
				return
			}
		}
	}
}

func insertSignaturesAndAccounts(tx *sqlx.Tx, msg *onchainMessage) ([]database.Signature, []database.Account) {
	slog.Info("inserting signatures and accounts")
	insertableSignatures := make(pq.StringArray, len(msg.txs))
	insertableAccounts := make(pq.StringArray, 0)
	for i, tx := range msg.txs {
		insertableSignatures[i] = tx.Signature
		for _, account := range tx.Accounts {
			if !slices.Contains(insertableAccounts, account) {
				insertableAccounts = append(insertableAccounts, account)
			}
		}
	}

	insertedSignatures := []database.Signature{}
	q := "INSERT INTO signature (value) (SELECT * FROM unnest($1::varchar[])) RETURNING id, value"
	err := tx.Select(&insertedSignatures, q, insertableSignatures)
	utils.AssertNoErr(err)

	accounts := []database.Account{}
	q = "SELECT id, value as address FROM address WHERE value = ANY($1)"
	err = tx.Select(&accounts, q, insertableAccounts)
	utils.AssertNoErr(err)

	if len(accounts) < len(insertableAccounts) {
		missingAccounts := pq.StringArray{}
		for _, address := range insertableAccounts {
			idx := slices.IndexFunc(accounts, func(a database.Account) bool {
				return a.Address == address
			})
			if idx == -1 {
				missingAccounts = append(missingAccounts, address)
			}
		}

		insertedAccounts := []database.Account{}
		q = "INSERT INTO address (value) (SELECT * FROM unnest($1::varchar[])) RETURNING id, value as address"
		err = tx.Select(&insertedAccounts, q, missingAccounts)
		utils.AssertNoErr(err)

		accounts = append(accounts, insertedAccounts...)
	}

	slog.Info("inserted signatures and accounts")
	return insertedSignatures, accounts
}

func insertTransactionsData(tx *sqlx.Tx, signatures []database.Signature, accounts []database.Account, msg *onchainMessage) {
	slog.Info("inserting transactions data")
	insertableTransactions, insertableInstructions, insertableInnerInstructions, insertableEvents := msg.intoInsertable(signatures, accounts)
	utils.Assert(len(insertableTransactions) > 0, "0 insertableTransactions")

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		q := `
			INSERT INTO
				transaction (signature_id, accounts_ids, timestamp, timestamp_granularized, slot, logs, err, fee)
			VALUES
				(:signature_id, :accounts_ids, :timestamp, :timestamp_granularized, :slot, :logs, :err, :fee)
		`
		_, err := tx.NamedExec(q, insertableTransactions)
		utils.AssertNoErr(err)
	}()

	if len(insertableInstructions) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if len(insertableInstructions) > 0 {
				q := `
					INSERT INTO
						instruction (signature_id, index, program_account_id, accounts_ids, data)
					VALUES
						(:signature_id, :index, :program_account_id, :accounts_ids, :data)
				`
				_, err := tx.NamedExec(q, insertableInstructions)
				utils.AssertNoErr(err)
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				if len(insertableInnerInstructions) > 0 {
					q := `
							INSERT INTO
								inner_instruction (signature_id, index, ix_index, program_account_id, accounts_ids, data)
							VALUES
								(:signature_id, :index, :ix_index, :program_account_id, :accounts_ids, :data)
						`
					_, err := tx.NamedExec(q, insertableInnerInstructions)
					utils.AssertNoErr(err)
				}
				if len(insertableEvents) > 0 {
					q := `
						INSERT INTO
							instruction_event (signature_id, ix_index, index, type, data)
						VALUES
							(:signature_id, :ix_index, :index, :type, :data)
					`
					_, err := tx.NamedExec(q, insertableEvents)
					utils.AssertNoErr(err)
				}
			}()
		}()
	}

	wg.Wait()
	slog.Info("inserted transactions data")
}

type walletToSignature struct {
	WalletId    int32 `db:"wallet_id"`
	SignatureId int32 `db:"signature_id"`
}

func insertWalletData(
	db *sqlx.DB,
	walletId int32,
	associatedAccountAddress string,
	signaturesIds []int32,
	associatedAccounts []*instructionsparser.AssociatedAccount,
	lastSignature string,
) error {
	utils.Assert(
		(associatedAccountAddress != "" && len(associatedAccounts) == 0) || associatedAccountAddress == "",
		"associated account must not have associated accounts",
	)
	utils.Assert(
		len(signaturesIds) > 0 || len(associatedAccounts) > 0 || lastSignature != "",
		"unable to insert account data, msg is empty",
	)

	tx, err := db.Beginx()
	utils.AssertNoErr(err)
	defer tx.Rollback()

	signaturesCount := int64(0)
	associatedAccountsCount := int64(0)

	if len(signaturesIds) > 0 {
		insertable := make([]walletToSignature, len(signaturesIds))
		for i, signatureId := range signaturesIds {
			insertable[i] = walletToSignature{WalletId: walletId, SignatureId: signatureId}
		}
		q := "INSERT INTO wallet_to_signature (wallet_id, signature_id) VALUES (:wallet_id, :signature_id)"
		res, err := tx.NamedExec(q, insertable)
		if err != nil {
			return err
		}
		if signaturesCount, err = res.RowsAffected(); err != nil {
			return err
		}
	}

	if len(associatedAccounts) > 0 {
		addresses := make([]string, len(associatedAccounts))
		for i, associatedAccount := range associatedAccounts {
			addresses[i] = associatedAccount.Address
		}

		addressesIds := []*database.Account{}
		q := "SELECT value AS address, id FROM address WHERE id = ANY($1)"
		if err = tx.Select(&addressesIds, q, addresses); err != nil {
			return err
		}

		insertableAssociatedAccounts := make([]map[string]int32, len(associatedAccounts))
		for i, associatedAccount := range associatedAccounts {
			addressIdx := slices.IndexFunc(addressesIds, func(a *database.Account) bool {
				return a.Address == associatedAccount.Address
			})
			utils.Assert(addressIdx > -1, "unable to find associated account address")
			insertableAssociatedAccounts[i] = map[string]int32{
				"wallet_id":  walletId,
				"address_id": addressesIds[addressIdx].Id,
			}
		}
		q = "INSERT INTO associated_account (wallet_id, address_id) VALUES (:wallet_id, :address_id)"
		res, err := tx.NamedExec(q, insertableAssociatedAccounts)
		if err != nil {
			return err
		}
		if associatedAccountsCount, err = res.RowsAffected(); err != nil {
			return err
		}
	}

	if associatedAccountAddress == "" {
		q := strings.Builder{}
		q.WriteString("UPDATE wallet SET ")
		args := []interface{}{}
		if signaturesCount > 0 {
			q.WriteString(fmt.Sprintf("signatures = signatures + $%d ", len(args)+1))
			args = append(args, signaturesCount)
		}
		if associatedAccountsCount > 0 {
			q.WriteString(fmt.Sprintf(",associated_accounts = associated_accounts + $%d ", len(args)+1))
			args = append(args, associatedAccountsCount)
		}
		if lastSignature != "" {
			q.WriteString(fmt.Sprintf(",last_signature = $%d ", len(args)+1))
			args = append(args, lastSignature)
		}
		q.WriteString(fmt.Sprintf("WHERE wallet_id = $%d", len(args)+1))
		args = append(args, walletId)
		if _, err = tx.Exec(q.String(), args...); err != nil {
			return err
		}
	} else if associatedAccountAddress != "" && lastSignature != "" {
		q := `
			UPDATE associated_account
			SET last_signature = $1
			WHERE wallet_id = $2 AND address_id = (
				SELECT address.id FROM address WHERE address.value = $3
			)
		`
		if _, err = tx.Exec(q, lastSignature, walletId, associatedAccountAddress); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func (c *Client) handleParsedTransactions(request *syncWalletRequest, msgChan chan interface{}) {
	for {
		msgUnknown, ok := <-msgChan
		if !ok {
			slog.Info("msg chan was closed, exiting client.handleParsedTransaction")
			return
		}
		switch msg := msgUnknown.(type) {
		case *savedMessage:
			slog.Info("received saved msg")
			insertWalletData(c.db, request.WalletId, msg.associatedAccountAddress, msg.signaturesIds, msg.associatedAccounts, msg.lastSignature)
		case *onchainMessage:
			slog.Info("received onchain msg")
			tx, err := c.db.Beginx()
			utils.AssertNoErr(err)

			signatures, accounts := insertSignaturesAndAccounts(tx, msg)
			insertTransactionsData(tx, signatures, accounts, msg)

			err = tx.Commit()
			utils.AssertNoErr(err)
			slog.Info("committed signatures, accounts and transactions")

			signaturesIds := make([]int32, len(signatures))
			for i, signature := range signatures {
				signaturesIds[i] = signature.Id
			}
			insertWalletData(c.db, request.WalletId, msg.associatedAccountAddress, signaturesIds, msg.associatedAccounts, msg.lastSignature)
		default:
			st := utils.GetStackTrace()
			log.Fatalf("invalid msg in channel\n%s", st)
		}
	}
}

func (c *Client) deduplicateTransactions() {
	type signatureWithSlot struct {
		Slot        int64
		SignatureId int32 `db:"signature_id"`
		Signature   string
	}
	txs := []*signatureWithSlot{}
	q := `
		SELECT
			t1.slot,
			t1.signature_id,
			signature.value AS signature
		FROM
			transaction t1
		INNER JOIN
			signature ON signature.id = transaction.signature_id
		WHERE EXISTS(
			SELECT
				1
			FROM
				transaction t2
			WHERE
				t1.id != t2.id AND t1.timestamp = t2.timestamp AND t1.slot = t2.slot AND t1.block_index IS NULL AND t2.block_index IS NULL
		)
	`
	err := c.db.Select(&txs, q)
	utils.AssertNoErr(err)
	if len(txs) == 0 {
		return
	}

	slots := make(map[int64]bool)
	for _, tx := range txs {
		slots[tx.Slot] = true
	}

	type txWithBlockIndex struct {
		SignatureId int32 `db:"signature_id"`
		BlockIndex  int32 `db:"block_index"`
	}
	txsWithBlockIndexes := []*txWithBlockIndex{}
	for slot := range maps.Keys(slots) {
		blockRes, err := CallRpcWithRetries(func() (*rpc.GetBlockResult, error) {
			return c.rpcClient.GetBlock(c.ctx, uint64(slot))
		}, 5)
		utils.AssertNoErr(err)
		for blockIndex, signature := range blockRes.Signatures {
			idx := slices.IndexFunc(txs, func(tx *signatureWithSlot) bool {
				return tx.Signature == signature.String()
			})
			if idx != -1 {
				txsWithBlockIndexes = append(txsWithBlockIndexes, &txWithBlockIndex{
					SignatureId: txs[idx].SignatureId,
					BlockIndex:  int32(blockIndex),
				})
			}
		}
	}

	q = `
		UPDATE transaction SET transaction.block_index = v.bi FROM (
			VALUES (:signature_id, :block_index)
		) AS v(sid, bi) WHERE transaction.signature_id = v.sid
	`
	_, err = c.db.NamedExec(q, txsWithBlockIndexes)
	utils.AssertNoErr(err)
}

func (c *Client) Run() {
	slog.Info("starting sync service")
	ticker := time.NewTicker(10 * time.Second)

fetchLoop:
	for {
		select {
		case <-c.ctx.Done():
			slog.Info("context done, exiting client.Run")
			return
		case <-ticker.C:
			request, err := fetchSyncWalletRequest(c.db)
			if errors.Is(err, sql.ErrNoRows) {
				slog.Info("sync wallet queue empty")
				continue fetchLoop
			}
			utils.AssertNoErr(err)
			slog.Info("fetched sync wallet request", "wallet_id", request.WalletId, "address", request.Address, "last_signature", request.LastSignature)

			_, err = c.db.Exec("UPDATE sync_wallet_request SET status = 'fetching_transactions' WHERE wallet_id = $1", request.WalletId)
			utils.AssertNoErr(err)

			interruptChan := make(chan struct{})
			defer close(interruptChan)

			reqCtx, cancel := context.WithCancel(c.ctx)
			defer cancel()
			go request.watchDelete(reqCtx, c.db, interruptChan)
			go func() {
				_, ok := <-interruptChan
				if !ok {
					slog.Info("interrupt chan closed")
					return
				}
				slog.Info("received interrupt msg")
				cancel()
			}()

			msgChan := make(chan interface{})
			parser := instructionsparser.New(request.Address, request.WalletId, c.db)

			go c.handleParsedTransactions(&request, msgChan)

			slog.Info("syncing main wallet")
			c.fetchAndParseTransactions(reqCtx, &request, parser, msgChan)

			slog.Debug("syncing associated accounts", "len", len(parser.AssociatedAccounts))
			for _, associatedAccount := range parser.AssociatedAccounts {
				slog.Debug("current account", "address", associatedAccount.Address)
				c.fetchAndParseTransactions(reqCtx, associatedAccount, parser, msgChan)
			}

			_, err = c.db.Exec("UPDATE sync_wallet_request SET status = 'parsing_events' WHERE wallet_id = $1", request.WalletId)
			utils.AssertNoErr(err)

			close(msgChan)

			c.deduplicateTransactions()
		}
	}
}
