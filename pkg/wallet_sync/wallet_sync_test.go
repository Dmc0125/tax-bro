package walletsync

import (
	"database/sql"
	"encoding/hex"
	"slices"
	"sync"
	testutils "tax-bro/pkg/test_utils"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/suite"
)

func TestMsgQueueTestSuite(t *testing.T) {
	suite.Run(t, new(MsgQueueTestSuite))
}

func TestSync(t *testing.T) {
	suite.Run(t, new(SyncTestSuite))
}

type MsgQueueTestSuite struct {
	suite.Suite
}

func (s *MsgQueueTestSuite) TestWalk() {
	t := s.T()

	q := newMsgQueue()
	items := []int{1, 2, 3, 4, 5, 6}
	for _, i := range items {
		q.append(i)
	}

	t.Run("No deletion", func(t *testing.T) {
		count := 0
		q.walk(func(item interface{}) bool {
			count++
			return false
		}, func() bool { return false })
		require.Equal(t, 6, count)
	})

	t.Run("Delete even numbers", func(t *testing.T) {
		q.walk(func(item interface{}) bool {
			return item.(int)%2 == 0
		}, func() bool { return false })
		require.Equal(t, 3, len(q.items))
		for _, item := range q.items {
			require.Equal(t, 1, item.(int)%2)
		}
	})

	t.Run("Exit early", func(t *testing.T) {
		count := 0
		q.walk(func(item interface{}) bool {
			count++
			return false
		}, func() bool { return count >= 2 })
		require.Equal(t, 2, count)
	})
}

func (s *MsgQueueTestSuite) TestConcurrency() {
	t := s.T()
	q := newMsgQueue()
	numGoroutines := 100
	itemsPerGoroutine := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				q.append(j)
			}
		}()
	}

	wg.Wait()
	require.Equal(t, numGoroutines*itemsPerGoroutine, len(q.items))
}

type SyncTestSuite struct {
	suite.Suite
	db        *sqlx.DB
	rpcClient *rpc.Client
	wallets   []solana.Wallet
}

func (s *SyncTestSuite) SetupSuite() {
	s.db = testutils.SetupDb(s.T())
	rpcClient, wallets := testutils.SetupSolana()

	s.rpcClient = rpcClient
	s.wallets = wallets
}

func (s *SyncTestSuite) seedTransactions() []string {
	txs := [][]solana.Instruction{}
	dummyAccounts := []string{}

	for i := 0; i < 3; i++ {
		wallet := s.wallets[0]

		dummyWallet := solana.NewWallet()
		dummyAccounts = append(dummyAccounts, dummyWallet.PublicKey().String())
		lamports := 10_000_000 * (i + 1)

		ix := system.NewTransferInstruction(uint64(lamports), wallet.PublicKey(), dummyWallet.PublicKey()).Build()
		txs = append(txs, []solana.Instruction{ix})
	}

	testutils.ForceSendTxs(s.rpcClient, &s.wallets[0], txs)
	return dummyAccounts
}

func (s *SyncTestSuite) TestWalletSync() {
	s.seedTransactions()
	t := s.T()

	queue := newMsgQueue()
	req := syncWalletRequest{
		WalletId:      1,
		WalletAddress: s.wallets[0].PublicKey().String(),
	}

	fetchAndParse(s.rpcClient, s.db, req, &queue)

	// 3 txs
	// 1 last
	require.Equal(t, 4, len(queue.items), "invalid queue len")
	for idx, i := range queue.items {
		switch msg := i.(type) {
		case parsedOnchainTransactionMsg:

			require.Equal(t, 0, len(msg.accounts), "invalid msg accounts len")
			tx := msg.transaction
			require.Equal(t, 3, len(tx.accounts), "invalid accounts len")
			require.Equal(t, 1, len(tx.Ixs), "invalid ixs len")
		case lasgTransactionMsg:
			require.Equal(t, 3, idx)
			require.Equal(t, int32(1), msg.walletId)
		default:
			t.FailNow()
		}
	}

	u := processMessages(&queue)

	require.Equal(t, int32(1), u.walletId, "invalid update - wallet id")
	require.Equal(t, 3, len(u.transactions), "invalid update - txs len")
	require.Equal(t, 0, len(u.associatedAccounts), "invalid update - associated accounts len")

	db := s.db
	u.saveTransactions(db)

	require.Equal(t, 3, len(u.signaturesIds), "invalid update - signatures ids len")

	signatures := pq.StringArray{}
	for _, tx := range u.transactions {
		signatures = append(signatures, tx.Signature)
	}

	type TestTx struct {
		Id                    int32
		Signature             string
		AccountsIds           pq.Int32Array `db:"accounts_ids"`
		Timestamp             time.Time
		TimestampGranularized time.Time `db:"timestamp_granularized"`
		Slot                  int64
		BlockIndex            sql.NullInt32 `db:"block_index"`
		Logs                  pq.StringArray
		Err                   bool
		Fee                   int64
	}

	txs := []TestTx{}
	err := db.Select(
		&txs,
		`SELECT
			transaction.id,
			transaction.accounts_ids,
			transaction.timestamp,
			transaction.timestamp_granularized,
			transaction.slot,
			transaction.block_index,
			transaction.logs,
			transaction.err,
			transaction.fee,
			signature.id,
			signature.value AS signature
		FROM
			signature
		INNER JOIN
			transaction ON transaction.signature_id = signature.id
		WHERE
			signature.value = ANY($1)
		ORDER BY
			transaction.slot ASC, transaction.timestamp ASC, transaction.block_index ASC`,
		signatures,
	)
	require.NoError(t, err)
	require.Equal(t, 3, len(txs), "invalid txs len")

	prevSlot := int64(0)
	prevTimestamp := time.Unix(0, 0)

	for _, tx := range txs {
		idx := slices.IndexFunc(u.transactions, func(utx onchainTransaction) bool {
			return utx.Signature == tx.Signature
		})
		require.Greater(t, idx, -1)
		utx := u.transactions[idx]

		require.Equal(t, utx.Signature, tx.Signature)
		require.Equal(t, int64(utx.Slot), tx.Slot)
		require.Equal(t, utx.Timestamp.In(time.UTC), tx.Timestamp)
		require.Equal(t, utx.TimestampGranularized.In(time.UTC), tx.TimestampGranularized)
		require.Equal(t, utx.Err, tx.Err)
		require.Equal(t, utx.Fee, uint64(tx.Fee))

		for i, l1 := range tx.Logs {
			l2 := utx.Logs[i]
			require.Equal(t, l1, l2)
		}

		require.GreaterOrEqual(t, tx.Slot, prevSlot)
		require.GreaterOrEqual(t, tx.Timestamp, prevTimestamp)
		require.Equal(t, false, tx.BlockIndex.Valid)
	}

	type TestIx struct {
		Signature      string
		Index          int16
		ProgramAddress string `db:"program_address"`
		Data           string
		AccountsIds    pq.Int32Array `db:"accounts_ids"`
	}
	ixs := []TestIx{}
	err = db.Select(
		&ixs,
		`SELECT
			signature.value as signature,
			address.value as program_address,
			instruction.index,
			instruction.data,
			instruction.accounts_ids
		FROM
			instruction
		INNER JOIN
			signature ON signature.id = instruction.signature_id
		INNER JOIN
			address ON address.id = instruction.program_account_id`,
	)
	require.NoError(t, err)

	for _, ix := range ixs {
		uix := onchainInstruction{}
		for _, tx := range u.transactions {
			if tx.Signature != ix.Signature {
				continue
			}
			for ixIndex, uTxIx := range tx.Ixs {
				if ixIndex == int(ix.Index) {
					uix = uTxIx
				}
			}
		}
		require.NotEqual(t, onchainInstruction{}, uix)
		require.Equal(t, uix.programAddress, ix.ProgramAddress)

		b, err := hex.DecodeString(ix.Data)
		require.NoError(t, err)
		require.Equal(t, uix.data, b)

		accounts := []account{}
		err = db.Select(&accounts, "SELECT id, value AS address, array_position($1, id) AS ord FROM address WHERE id = ANY($1) ORDER BY ord ASC", ix.AccountsIds)
		require.NoError(t, err)
		for i, a := range accounts {
			require.Equal(t, uix.accounts[i], a.Address)
		}
	}

	_, err = db.Exec("INSERT INTO \"account\" (auth_provider) VALUES ('github')")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO wallet (account_id, address_id, label) VALUES (1, 1, 'test')")
	require.NoError(t, err)

	u.updateAccount(db)

	type TestWti struct {
		SignatureId int32 `db:"signature_id"`
		WalletId    int32 `db:"wallet_id"`
	}
	testWti := []TestWti{}
	err = db.Select(&testWti, "SELECT signature_id, wallet_id FROM wallet_to_signature")
	require.NoError(t, err)

	require.Equal(t, 3, len(testWti))
	for _, wti := range testWti {
		idx := slices.IndexFunc(u.signaturesIds, func(sid int32) bool {
			return sid == wti.SignatureId
		})
		require.Greater(t, idx, -1)
		require.Equal(t, int32(1), wti.WalletId)
	}

	type TestWallet struct {
		LastSignatureId    sql.NullInt32 `db:"last_signature_id"`
		Signatures         int32
		AssociatedAccounts int32 `db:"associated_accounts"`
	}
	updatedWallet := TestWallet{}
	err = db.Get(&updatedWallet, "SELECT last_signature_id, signatures, associated_accounts FROM wallet WHERE id = 1")
	require.NoError(t, err)

	require.Equal(t, int32(0), updatedWallet.AssociatedAccounts)
	require.Equal(t, int32(3), updatedWallet.Signatures)
	require.Equal(t, true, updatedWallet.LastSignatureId.Valid)
	require.Equal(t, int32(1), updatedWallet.LastSignatureId.Int32)
}
