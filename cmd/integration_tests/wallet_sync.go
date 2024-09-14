package main

import (
	"errors"
	"fmt"
	walletsync "tax-bro/pkg/wallet_sync"
)

type TestTransactionsGet struct{}

func (t *TestTransactionsGet) name() string {
	return "transactions.get"
}

func (t *TestTransactionsGet) seed() error {
	_, err := db.Exec("INSERT INTO signature (value) VALUES ('123abc')")
	if err != nil {
		return errors.Join(errors.New("db err"), err)
	}
	_, err = db.Exec("INSERT INTO address (value) VALUES ('not_an_address'), ('yes_address')")
	if err != nil {
		return errors.Join(errors.New("db err"), err)
	}
	_, err = db.Exec(
		`INSERT INTO
			transaction (signature_id, accounts_ids, timestamp, timestamp_granularized, slot, logs, err, fee)
		VALUES
			(1, ARRAY[1, 2], now(), now(), 1, ARRAY['this is a log'], false, 1000)
		`,
	)
	if err != nil {
		return errors.Join(errors.New("db err"), err)
	}
	_, err = db.Exec(
		`INSERT INTO
			instruction (signature_id, index, program_account_id, accounts_ids, data)
		VALUES
			(1, 0, 1, ARRAY[1, 2, 1], '\x062a25ea915bd19379b400fa093d5813'),
			(1, 1, 2, ARRAY[2, 2, 1], '\x2e1581f042be23e5184bfefa1dc4b8a7')
		`,
	)
	if err != nil {
		return errors.Join(errors.New("db err"), err)
	}
	_, err = db.Exec(
		`INSERT INTO
			inner_instruction (signature_id, ix_index, index, program_account_id, accounts_ids, data)
		VALUES
			(1, 0, 0, 1, ARRAY[1, 2, 1], '\x062a'),
			(1, 0, 1, 1, ARRAY[1, 2, 1], '\x062a25ea915bd19379b400fa093d5813')
		`,
	)
	if err != nil {
		return errors.Join(errors.New("db err"), err)
	}

	return nil
}

func (t *TestTransactionsGet) execute() error {
	err := t.seed()
	if err != nil {
		return err
	}

	txs := walletsync.Transactions{}
	txs.Get(db, []string{"123abc"})

	tx := txs[0]

	if tx.Signature != "123abc" {
		return expectErrMsg("123abc", tx.SignatureId, "invalid signature")
	}
	if tx.SignatureId != 1 {
		return expectErrMsg(1, tx.Signature, "invalid signature id")
	}

	if len(tx.Logs) != 1 || tx.Logs[0] != "this is a log" {
		return expectErrMsg([]string{"this is a log"}, tx.Logs, "invalid logs")
	}

	if len(tx.Ixs) != 2 {
		return expectErrMsg(2, len(tx.Ixs), "invalid ixs len")
	}

	ix := tx.Ixs[0]
	if ix.ProgramAddress != "not_an_address" {
		return expectErrMsg("not_an_address", ix.ProgramAddress, "invalid program address")
	}
	expectedAccounts := []walletsync.Account{
		{Id: 1, Address: "not_an_address"},
		{Id: 2, Address: "yes_address"},
		{Id: 1, Address: "not_an_address"},
	}
	for i, account := range ix.Accounts {
		if account.Id != expectedAccounts[i].Id || account.Address != expectedAccounts[i].Address {
			return expectErrMsg(expectedAccounts[i], account, "invalid account")
		}
	}
	expectedData := "\\x062a25ea915bd19379b400fa093d5813"
	if ix.Data != expectedData {
		return expectErrMsg(expectedData, ix.Data, "invalid data")
	}

	if len(ix.InnerIxs) != 2 {
		return expectErrMsg(1, len(ix.InnerIxs), "invalid count of inner ixs")
	}
	if ix.InnerIxs[0].Data != "\\x062a" && ix.InnerIxs[1].Data != expectedData {
		return fmt.Errorf("invalid inner ixs data or invalid ordering: %#v", ix.InnerIxs)
	}

	return nil
}
