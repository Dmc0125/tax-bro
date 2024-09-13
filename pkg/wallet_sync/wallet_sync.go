package walletsync

import (
	"context"
	"fmt"
	"log"
	"sync"
	instructionsparser "tax-bro/pkg/instructions_parser"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type parsedSavedTransactionMsg struct {
	walletId    int32
	signatureId int32
	accounts    []instructionsparser.AssociatedAccount
}

type parsedOnchainTransactionMsg struct {
	walletId    int32
	transaction onchainTransaction
	accounts    []instructionsparser.AssociatedAccount
}

type msgQueue struct {
	m     *sync.Mutex
	items []interface{}
}

func newMsgQueue() msgQueue {
	return msgQueue{
		items: make([]interface{}, 0),
		m:     &sync.Mutex{},
	}
}

func (q *msgQueue) append(item interface{}) {
	q.m.Lock()
	q.items = append(q.items, item)
	q.m.Unlock()
}

func (q *msgQueue) appendMessages(items []interface{}) {
	q.m.Lock()
	q.items = append(q.items, items...)
	q.m.Unlock()
}

func (q *msgQueue) walk(f func(interface{}) bool, exitEarly func() bool) {
	q.m.Lock()
	defer q.m.Unlock()
	i := 0
	for {
		if i > len(q.items)-1 {
			return
		}
		if exitEarly() {
			return
		}

		item := &q.items[i]
		shouldDelete := f(item)

		if shouldDelete {
			q.items = append(q.items[:i], q.items[i+1:])
		} else {
			i += 1
		}
	}
}

func fetchAndParse(rpcClient *rpc.Client, db *sqlx.DB, req syncWalletRequest, queue *msgQueue) {
	account := solana.MustPublicKeyFromBase58(req.WalletAddress)
	limit := int(100)
	getSignaturesOpts := rpc.GetSignaturesForAddressOpts{
		Commitment: rpc.CommitmentConfirmed,
		Limit:      &limit,
	}
	if req.LastSignature != "" {
		getSignaturesOpts.Until = solana.MustSignatureFromBase58(req.LastSignature)
	}

	for {
		signaturesRes, err := callRpcWithRetries(
			func() ([]*rpc.TransactionSignature, error) {
				return rpcClient.GetSignaturesForAddressWithOpts(
					context.Background(),
					account,
					&getSignaturesOpts,
				)
			},
			5,
		)
		utils.Assert(err == nil, fmt.Sprint(err))

		if len(signaturesRes) == 0 {
			break
		}

		isLast := len(signaturesRes) < limit

		signatures := make([]string, len(signaturesRes))
		for i := 0; i < len(signaturesRes); i++ {
			signatures[i] = signaturesRes[i].Signature.String()
		}

		savedTransactions := transactions{}
		savedTransactions.get(db, signatures)

		if len(savedTransactions) > 0 {
			messages := make([]interface{}, 0)

			for i := 0; i < len(savedTransactions); i++ {
				tx := &savedTransactions[i]
				msg := parsedSavedTransactionMsg{
					walletId:    req.WalletId,
					signatureId: tx.SignatureId,
					accounts:    make([]instructionsparser.AssociatedAccount, 0),
				}

				for j := 0; j < len(tx.Ixs); j++ {
					_, associatedAccounts := tx.Ixs[j].parse()
					msg.accounts = append(msg.accounts, associatedAccounts...)
				}

				messages = append(messages, msg)
			}

			queue.appendMessages(messages)
		}

		for i := 0; i < len(signatures); i++ {
			if savedTransactions.contains(signatures[i]) {
				continue
			}
			tx := fetchTransaction(rpcClient, signaturesRes[i].Signature)
			msg := parsedOnchainTransactionMsg{
				transaction: tx,
				walletId:    req.WalletId,
				accounts:    make([]instructionsparser.AssociatedAccount, 0),
			}

			for ixIndex := 0; ixIndex < len(tx.Ixs); ixIndex++ {
				ix := &tx.Ixs[ixIndex]
				innerIxs := make([]*onchainInstructionBase, len(ix.InnerIxs))
				for k := 0; k < len(ix.InnerIxs); k++ {
					innerIxs[k] = &ix.InnerIxs[k]
				}

				events, associatedAccounts := instructionsparser.Parse(ix, innerIxs)
				ix.events = append(ix.events, events...)

				msg.accounts = append(msg.accounts, associatedAccounts...)
			}

			queue.append(msg)
		}

		if isLast {
			break
		}
	}
}

type update struct {
	walletId           int32
	transactions       []onchainTransaction
	signaturesIds      []int32
	associatedAccounts []instructionsparser.AssociatedAccount
}

func (u *update) saveTransactions(db *sqlx.DB) {
	tx, err := db.Beginx()
	utils.Assert(err == nil, fmt.Sprintf("unable to begin tx error: %s", err))
	defer tx.Rollback()

	insertableSignatures := make(pq.StringArray, len(u.transactions))
	insertableAddresses := make(pq.StringArray, 0)
	for i := 0; i < len(u.transactions); i++ {
		tx := &u.transactions[i]
		insertableSignatures[i] = tx.Signature
		insertableAddresses = append(insertableAddresses, tx.accounts...)
	}

	insertedSignatures := signatures{}
	insertedSignatures.save(tx, insertableSignatures)

	for i := 0; i < len(insertedSignatures); i++ {
		u.signaturesIds = append(u.signaturesIds, insertedSignatures[i].Id)
	}

	insertedAccounts := accounts{}
	insertedAccounts.save(tx, insertableAddresses)

	insertableTxs := []map[string]interface{}{}
	insertableIxs := []map[string]interface{}{}
	insertableInnerIxs := []map[string]interface{}{}
	insertableLogs := []map[string]interface{}{}

	for i := 0; i < len(u.transactions); i++ {
		tx := &u.transactions[i]

		signatureId, err := insertedSignatures.getId(tx.Signature)
		utils.Assert(err == nil, fmt.Sprintf("invalid update: %s", err))
		accountsIds := make([]int32, len(tx.accounts))
		for _, a := range tx.accounts {
			aId, err := insertedAccounts.getId(a)
			utils.Assert(err == nil, fmt.Sprintf("invalid update: %s", err))
			accountsIds = append(accountsIds, aId)
		}

		insertableTxs = append(insertableTxs, map[string]interface{}{
			"signature_id":           signatureId,
			"accounts_ids":           accountsIds,
			"timestamp":              tx.Timestamp,
			"timestamp_granularized": tx.TimestampGranularized,
			"slot":                   tx.Slot,
			"err":                    tx.Err,
			"fee":                    tx.Fee,
		})

		if len(tx.Logs) > 0 {
			insertableLogs = append(insertableLogs, map[string]interface{}{
				"signature_id": signatureId,
				"logs":         pq.StringArray(tx.Logs),
			})
		}

		for ixIndex := 0; ixIndex < len(tx.Ixs); ixIndex++ {
			ix := &tx.Ixs[ixIndex]
			programAccountId, accountsIds, data, err := ix.prepareForInsert(insertedAccounts)
			utils.Assert(err == nil, fmt.Sprintf("invalid update: %s", err))

			insertableIxs = append(insertableIxs, map[string]interface{}{
				"signature_id":       signatureId,
				"index":              int16(ixIndex),
				"program_account_id": programAccountId,
				"accounts_ids":       accountsIds,
				"data":               data,
			})

			for innerIxIndex := 0; innerIxIndex < len(ix.InnerIxs); innerIxIndex++ {
				innerIx := &ix.InnerIxs[innerIxIndex]
				programAccountId, accountsIds, data, err := innerIx.prepareForInsert(
					insertedAccounts,
				)
				utils.Assert(err == nil, fmt.Sprintf("invalid update: %s", err))

				insertableInnerIxs = append(insertableInnerIxs, map[string]interface{}{
					"signature_id":       signatureId,
					"ix_index":           int16(ixIndex),
					"index":              int16(innerIxIndex),
					"program_account_id": programAccountId,
					"accounts_ids":       accountsIds,
					"data":               data,
				})
			}
		}
	}

	if len(insertableTxs) > 0 {
		_, err = tx.Exec(
			`INSERT INTO
				transaction (signature_id, accounts_ids, timestamp, timestamp_granularized, slot, err, fee)
			VALUES (:signature_id, :accounts_ids, :timestamp, :timestamp_granularized, :slot, :err, :fee)`,
			insertableTxs,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert txs: %s", err))
	}

	if len(insertableIxs) > 0 {
		_, err := tx.Exec(
			`INSERT INTO
				instruction (signature_id, index, program_account_id, accounts_ids, data)
			VALUES (:signature_id, :index, :program_account_id, :accounts_ids, :data)`,
			insertableIxs,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert ixs: %s", err))
	}

	if len(insertableInnerIxs) > 0 {
		_, err := tx.Exec(
			`INSERT INTO
				instruction (signature_id, ix_index, index, program_account_id, accounts_ids, data)
			VALUES (:signature_id, :ix_index,:index, :program_account_id, :accounts_ids, :data)`,
			insertableInnerIxs,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert inner ixs: %s", err))
	}

	if len(insertableLogs) > 0 {
		_, err := tx.Exec(
			`INSERT INTO transaction_logs VALUES (:signature_id, :logs)`,
			insertableLogs,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert logs: %s", err))
	}

	err = tx.Commit()
	utils.Assert(err == nil, fmt.Sprintf("unable to commit tx: %s", err))
}

func (u *update) updateUser(db *sqlx.DB) {
	tx, err := db.Beginx()
	utils.Assert(err == nil, fmt.Sprintf("unable to begin tx error: %s", err))
	defer tx.Rollback()

	if len(u.signaturesIds) > 0 {
		walletsIds := make([]int32, len(u.signaturesIds))
		for i := 0; i < len(u.signaturesIds); i++ {
			walletsIds[i] = u.walletId
		}

		res, err := tx.Exec(
			`INSERT INTO
				wallet_to_signature (signature_id, wallet_id) (
					SELECT * FROM unzip($1::integer[], $2::integer[])
				)
			ON CONFICT (signature_id, wallet_id) DO NOTHING`,
			u.signaturesIds,
			walletsIds,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert wts: %s", err))
		count, err := res.RowsAffected()
		utils.Assert(err == nil, fmt.Sprintf("unable to get rows affected: %s", err))

		if count > 0 {
			_, err = tx.Exec(
				"UPDATE wallet SET signatures = signatures + $1 WHERE id = $2",
				int32(count),
				u.walletId,
			)
			utils.Assert(err == nil, fmt.Sprintf("unable to update wallet: %s", err))
		}
	}

	if len(u.associatedAccounts) > 0 {
		addresses := make(pq.StringArray, len(u.associatedAccounts))
		for i := 0; i < len(u.associatedAccounts); i++ {
			addresses[i] = u.associatedAccounts[i].Address
		}
		accounts := []account{}
		tx.Select(
			&accounts,
			`SELECT
				address.id
			FROM
				address
			LEFT JOIN
				associated_account ON associated_account.address_id = address.id
			WHERE
				address.value = ANY($1) associated_account IS NULL`,
			addresses,
		)

		accountsIds := make([]int32, len(accounts))
		walletsIds := make([]int32, len(accounts))
		for i := 0; i < len(accounts); i++ {
			accountsIds[i] = accounts[i].Id
			walletsIds[i] = u.walletId
		}

		res, err := tx.Exec(
			"INSERT INTO associated_account (address_id, wallet_id) (SELECT * FROM unzip($1::integer[], $2::integer))",
			accountsIds,
			walletsIds,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert associated accounts: %s", err))
		count, err := res.RowsAffected()
		utils.Assert(err == nil, fmt.Sprintf("unable to get rows affected: %s", err))

		if count > 0 {
			_, err := tx.Exec(
				"UPDATE wallet SET associated_accounts = associated_accounts + $1 WHERE id = $2",
				int32(count),
				u.walletId,
			)
			utils.Assert(err == nil, fmt.Sprintf("unable to update wallet: %s", err))
		}
	}

	err = tx.Commit()
	utils.Assert(err == nil, fmt.Sprintf("unable to commit tx: %s", err))
}

func processMessages(queue *msgQueue) *update {
	u := update{
		walletId:           -1,
		transactions:       make([]onchainTransaction, 0),
		signaturesIds:      make([]int32, 0),
		associatedAccounts: make([]instructionsparser.AssociatedAccount, 0),
	}

	queue.walk(
		func(msgUnknown interface{}) bool {
			switch msg := msgUnknown.(type) {
			case parsedOnchainTransactionMsg:
				if u.walletId == msg.walletId || u.walletId == -1 {
					u.walletId = msg.walletId
					u.transactions = append(u.transactions, msg.transaction)
					u.associatedAccounts = append(u.associatedAccounts, msg.accounts...)
					return true
				}
			case parsedSavedTransactionMsg:
				if u.walletId == msg.walletId || u.walletId == -1 {
					u.walletId = msg.walletId
					u.signaturesIds = append(u.signaturesIds, msg.signatureId)
					u.associatedAccounts = append(u.associatedAccounts, msg.accounts...)
					return true
				}
			default:
				log.Fatalf("invalid message: %#v", msg)
			}

			return false
		},
		func() bool {
			return len(u.signaturesIds) > 20 || len(u.transactions) > 20 ||
				len(u.associatedAccounts) > 40
		},
	)

	return &u
}

func RunWalletSync(rpcClient *rpc.Client, db *sqlx.DB, ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	queue := newMsgQueue()

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncQueue := newSyncWalletQueue()
				syncQueue.load(db)

				if len(syncQueue) == 0 {
					continue
				}

				req := syncQueue[0]
				req.updateStatus(db, "processing")
				fetchAndParse(rpcClient, db, req, &queue)
			}
		}
	}()

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				u := processMessages(&queue)
				if u.walletId == -1 {
					continue
				}
				u.saveTransactions(db)
				u.updateUser(db)
			}
		}
	}()

	wg.Wait()
}
