package walletsync

import (
	testutils "tax-bro/pkg/test_utils"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/test-go/testify/require"
	"github.com/test-go/testify/suite"
)

func TestDbTestSuite(t *testing.T) {
	suite.Run(t, new(DbTestSuite))
}

type DbTestSuite struct {
	suite.Suite
	db *sqlx.DB
}

func (s *DbTestSuite) SetupSuite() {
	s.db = testutils.SetupDb(s.T())
}

func (s *DbTestSuite) seedTransactionsGet() (signature, []account) {
	t := s.T()
	db := s.db

	sig := signature{}
	err := db.Get(&sig, "INSERT INTO signature (value) VALUES ('123abc') RETURNING id, value")
	require.Nil(t, err)

	accounts := []account{}
	err = db.Select(&accounts, "INSERT INTO address (value) VALUES ('not_an_address'), ('yes_address') RETURNING id, value AS address")
	require.Nil(t, err)

	_, err = db.Exec(
		`INSERT INTO
			transaction (signature_id, accounts_ids, timestamp, timestamp_granularized, slot, logs, err, fee)
		VALUES
			($1, $2, now(), now(), 1, ARRAY['this is a log'], false, 1000)
		`,
		sig.Id,
		pq.Int32Array{accounts[0].Id, accounts[1].Id, accounts[0].Id},
	)
	require.Nil(t, err)

	_, err = db.Exec(
		`INSERT INTO
			instruction (signature_id, index, program_account_id, accounts_ids, data)
		VALUES
			($1, 0, $4, $2, '\x062a25ea915bd19379b400fa093d5813'),
			($1, 1, $5, $3, '\x2e1581f042be23e5184bfefa1dc4b8a7')
		`,
		sig.Id,
		pq.Int32Array{accounts[0].Id, accounts[1].Id, accounts[0].Id},
		pq.Int32Array{accounts[1].Id, accounts[1].Id, accounts[0].Id},
		accounts[0].Id,
		accounts[1].Id,
	)
	require.Nil(t, err)

	_, err = db.Exec(
		`INSERT INTO
			inner_instruction (signature_id, ix_index, index, program_account_id, accounts_ids, data)
		VALUES
			($1, 0, 0, $3, $2, '\x062a'),
			($1, 0, 1, $3, $2, '\x062a25ea915bd19379b400fa093d5813')
		`,
		sig.Id,
		pq.Int32Array{accounts[0].Id, accounts[1].Id, accounts[0].Id},
		accounts[0].Id,
	)
	require.Nil(t, err)

	return sig, accounts
}

func (s *DbTestSuite) TestTransactionsGet() {
	t := s.T()
	db := s.db

	sig, accounts := s.seedTransactionsGet()

	txs := transactions{}
	txs.Get(db, []string{"123abc"})

	tx := txs[0]

	require.Equal(t, sig.Value, tx.Signature, "invalid signature")
	require.Equal(t, sig.Id, tx.SignatureId, "invalid signature id")
	require.Equal(t, 1, len(tx.Logs), "invalid logs len")
	require.Equal(t, "this is a log", tx.Logs[0], "invalid log")
	require.Equal(t, 2, len(tx.Ixs), "invalid ixs len")

	ix := tx.Ixs[0]
	require.Equal(t, accounts[0].Address, ix.ProgramAddress, "invalid program address")

	expectedAccounts := []account{
		accounts[0],
		accounts[1],
		accounts[0],
	}
	for i, account := range ix.Accounts {
		if account.Id != expectedAccounts[i].Id || account.Address != expectedAccounts[i].Address {
			require.Equal(t, expectedAccounts[i], account, "invalid account")
		}
	}
	expectedData := "\\x062a25ea915bd19379b400fa093d5813"
	require.Equal(t, expectedData, ix.Data, "invalid data")

	require.Equal(t, 2, len(ix.InnerIxs), "invalid len of inner ixs")
	require.Equal(t, "\\x062a", ix.InnerIxs[0].Data, "invalid first inner ix")
	require.Equal(t, expectedData, ix.InnerIxs[1].Data, "invalid second inner ix")
}

func (s *DbTestSuite) seedSyncWalletRequest() (signature, account) {
	t := s.T()
	db := s.db

	sig := signature{}
	err := db.Get(&sig, "INSERT INTO signature (value) VALUES ('1234abc') RETURNING id, value")
	require.Nil(t, err)

	account := account{}
	err = db.Get(&account, "INSERT INTO address (value) VALUES ('account1') RETURNING id, value AS address")
	require.Nil(t, err)

	_, err = db.Exec("INSERT INTO \"account\" (auth_provider) VALUES ('github')")
	require.Nil(t, err)
	_, err = db.Exec("INSERT INTO wallet (account_id, address_id, label) VALUES (1, $1, 'lll')", account.Id)
	require.Nil(t, err)
	_, err = db.Exec("INSERT INTO sync_wallet_request (wallet_id) VALUES (1)")
	require.Nil(t, err)

	return sig, account
}

func (s *DbTestSuite) TestSyncWalletRequest() {
	t := s.T()
	db := s.db

	sig, account := s.seedSyncWalletRequest()

	q := newSyncWalletQueue()
	q.load(db)

	require.Equal(t, 1, len(q), "invalid queue len")

	req := q[0]
	require.Equal(t, int32(1), req.WalletId, "invalid wallet id")
	require.Equal(t, false, req.LastSignature.Valid, "invalid last signature")
	require.Equal(t, account.Address, req.WalletAddress, "invalid wallet address")

	if _, err := db.Exec("UPDATE wallet SET last_signature_id = 1 WHERE id = 1"); err != nil {
		t.Error(err)
	}

	q.load(db)
	req = q[0]
	require.Equal(t, true, req.LastSignature.Valid, "invalid last signature")
	require.Equal(t, sig.Value, req.LastSignature.String, "invalid last signature")
}
