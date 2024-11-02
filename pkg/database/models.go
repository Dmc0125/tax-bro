package database

import (
	"database/sql"
	"errors"
	"slices"
)

type Account struct {
	Address string
	Id      int32
	Ord     int32
}

type Accounts []Account

func (accounts Accounts) FindId(address string) (int32, error) {
	idx := slices.IndexFunc(accounts, func(account Account) bool {
		return account.Address == address
	})
	if idx == -1 {
		return 0, errors.New("unable to find account")
	}
	return accounts[idx].Id, nil
}

type Signature struct {
	Value string
	Id    int32
}

type AssociatedAccount struct {
	Address       string
	LastSignature sql.NullString `db:"last_signature"`
	Type          string
}
