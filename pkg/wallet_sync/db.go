package walletsync

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	instructionsparser "tax-bro/pkg/instructions_parser"
	"tax-bro/pkg/utils"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type syncWalletRequest struct {
	WalletId      int32          `db:"wallet_id"`
	WalletAddress string         `db:"address"`
	LastSignature sql.NullString `db:"last_signature"`
}

func (req *syncWalletRequest) updateStatus(db *sqlx.DB, status string) {
	utils.Assert(status == "processing" || status == "done", "invalid status")
	_, err := db.Exec(
		"UPDATE sync_wallet_request SET status = 'processing' WHERE wallet_id = $1",
		req.WalletId,
	)
	utils.Assert(err == nil, fmt.Sprintf("unable to update syncWalletRequest: %s", err))
}

type syncWalletQueue []syncWalletRequest

func newSyncWalletQueue() syncWalletQueue {
	return []syncWalletRequest{}
}

func (q *syncWalletQueue) load(db *sqlx.DB) {
	err := db.Select(
		q,
		`SELECT
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
			1`,
	)
	utils.Assert(err == nil, fmt.Sprintf("unable to get syncWalletRequests: %s", err))
}

type signature struct {
	Id    int32
	Value string
}

type signatures []signature

func (s *signatures) save(tx *sqlx.Tx, insertableSignatures pq.StringArray) {
	if len(insertableSignatures) > 0 {
		insertedSignatures := []signature{}
		err := tx.Select(
			&insertedSignatures,
			`INSERT INTO signature (value) (
				SELECT * FROM unnest($1::varchar[])
			) RETURNING id, value`,
			insertableSignatures,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert singatures: %s", err))
		*s = append(*s, insertedSignatures...)
	}
}

func (s signatures) getId(signature string) (int32, error) {
	for i := 0; i < len(s); i++ {
		if s[i].Value == signature {
			return s[i].Id, nil
		}
	}
	return 0, errors.New("unable to find signature id")
}

type account struct {
	Id      int32
	Address string
	Ord     int32
}

type accounts []account

func (a *accounts) save(tx *sqlx.Tx, insertableAddressesUnique map[string]bool) {
	if len(insertableAddressesUnique) == 0 {
		return
	}

	insertableAddresses := make(pq.StringArray, 0)
	for address := range insertableAddressesUnique {
		insertableAddresses = append(insertableAddresses, address)
	}

	savedAddresses := []account{}
	err := tx.Select(
		&savedAddresses,
		`SELECT id, value as address FROM address WHERE value = ANY($1)`,
		insertableAddresses,
	)
	utils.Assert(err == nil, fmt.Sprintf("unable to select addresses: %s", err))
	*a = append(*a, savedAddresses...)

	unsavedAddresses := make(pq.StringArray, 0)
	for _, a := range insertableAddresses {
		idx := slices.IndexFunc(savedAddresses, func(sa account) bool {
			return sa.Address == a
		})
		if idx == -1 {
			unsavedAddresses = append(unsavedAddresses, a)
		}
	}

	if len(unsavedAddresses) > 0 {
		insertedAddresses := []account{}
		err := tx.Select(
			&insertedAddresses,
			`INSERT INTO address (value) (
				SELECT * FROM unnest($1::varchar[])
			) RETURNING id, value as address`,
			unsavedAddresses,
		)
		utils.Assert(err == nil, fmt.Sprintf("unable to insert accounts: %s", err))
		*a = append(*a, insertedAddresses...)
	}
}

func (a accounts) getId(address string) (int32, error) {
	for i := 0; i < len(a); i++ {
		if a[i].Address == address {
			return a[i].Id, nil
		}
	}
	return 0, errors.New("unable to find account id")
}

type instructionBase struct {
	ProgramAddress string `json:"program_address"`
	Accounts       []account
	Data           string
}

func (ix *instructionBase) GetProgramAddress() string {
	return ix.ProgramAddress
}

func (ix *instructionBase) GetAccountAddress(idx int) string {
	return ix.Accounts[idx].Address
}

func (ix *instructionBase) GetData() []byte {
	bytes, err := hex.DecodeString(ix.Data)
	utils.Assert(err == nil, fmt.Sprintf("unable to decode ix data: %s", err))
	return bytes
}

type instruction struct {
	instructionBase
	InnerIxs []instructionBase `json:"inner_ixs"`
}

func (ix *instruction) parse() ([]instructionsparser.Event, []instructionsparser.AssociatedAccount) {
	innerIxs := make([]*instructionBase, len(ix.InnerIxs))
	for j := 0; j < len(ix.InnerIxs); j++ {
		innerIxs[j] = &ix.InnerIxs[j]
	}
	return instructionsparser.Parse(ix, innerIxs)
}

type transaction struct {
	Signature   string
	SignatureId int32 `db:"signature_id"`
	Ixs         []instruction
	Logs        []string
}

type transactions []transaction

func (txs transactions) contains(signature string) bool {
	for i := 0; i < len(txs); i++ {
		if txs[i].Signature == signature {
			return true
		}
	}
	return false
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

type transactionQueryResult struct {
	transaction
	Ixs  json.RawMessage
	Logs pq.StringArray
}

func (qr *transactionQueryResult) unmarshalIxs() (ixs []instruction, err error) {
	err = json.Unmarshal(qr.Ixs, &ixs)
	return
}

func (txs *transactions) Get(db *sqlx.DB, signatures []string) {
	q := fmt.Sprintf(
		`SELECT
			signature.value AS signature,
			signature.id AS signature_id,
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
			transaction.logs
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
	result := []transactionQueryResult{}
	err := db.Select(&result, q, pq.StringArray(signatures))
	utils.Assert(err == nil, fmt.Sprintf("unable to get transactions: %s", err))

	for i := 0; i < len(result); i++ {
		qr := &result[i]
		ixs, err := qr.unmarshalIxs()
		utils.Assert(err == nil, fmt.Sprintf("unable to unmarshal ixs: %s", err))
		*txs = append((*txs), transaction{
			Signature:   qr.Signature,
			SignatureId: qr.SignatureId,
			Logs:        qr.Logs,
			Ixs:         ixs,
		})
	}
}
