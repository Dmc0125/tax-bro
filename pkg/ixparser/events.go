package ixparser

import (
	"errors"
	"slices"
	"tax-bro/pkg/dbsqlc"
)

func findAccountId(addresses []*dbsqlc.Address, account string) (int32, error) {
	idx := slices.IndexFunc(addresses, func(a *dbsqlc.Address) bool {
		return a.Value == account
	})
	if idx == -1 {
		return 0, errors.New("unable to fund account")
	}
	return addresses[idx].ID, nil
}

type TransferEventData struct {
	From    string
	To      string
	Program string
	Amount  uint64
	IsRent  bool
}

func (data *TransferEventData) Compile(addresses []*dbsqlc.Address) (CompiledEvent, error) {
	fromId, err := findAccountId(addresses, data.From)
	if err != nil {
		return nil, err
	}
	toId, err := findAccountId(addresses, data.To)
	if err != nil {
		return nil, err
	}
	programId, err := findAccountId(addresses, data.Program)
	if err != nil {
		return nil, err
	}
	return &CompiledTransferEventData{
		From:    fromId,
		To:      toId,
		Program: programId,
		Amount:  data.Amount,
		IsRent:  data.IsRent,
	}, nil
}

type CompiledTransferEventData struct {
	From    int32
	To      int32
	Program int32
	Amount  uint64
	IsRent  bool
}

func (data *CompiledTransferEventData) Type() dbsqlc.EventType {
	return dbsqlc.EventTypeTransfer
}

type MintEventData struct {
	To     string
	Token  string
	Amount uint64
}

func (data *MintEventData) Compile(addresses []*dbsqlc.Address) (CompiledEvent, error) {
	toId, err := findAccountId(addresses, data.To)
	if err != nil {
		return nil, err
	}
	tokenId, err := findAccountId(addresses, data.Token)
	if err != nil {
		return nil, err
	}
	return &CompiledMintEventData{
		To:     toId,
		Token:  tokenId,
		Amount: data.Amount,
	}, nil
}

type CompiledMintEventData struct {
	To     int32
	Token  int32
	Amount uint64
}

func (data *CompiledMintEventData) Type() dbsqlc.EventType {
	return dbsqlc.EventTypeMint
}

type BurnEventData struct {
	From   string
	Token  string
	Amount uint64
}

func (data *BurnEventData) Compile(addresses []*dbsqlc.Address) (CompiledEvent, error) {
	fromId, err := findAccountId(addresses, data.From)
	if err != nil {
		return nil, err
	}
	tokenId, err := findAccountId(addresses, data.Token)
	if err != nil {
		return nil, err
	}
	return &CompiledBurnEventData{
		From:   fromId,
		Token:  tokenId,
		Amount: data.Amount,
	}, nil
}

type CompiledBurnEventData struct {
	From   int32
	Token  int32
	Amount uint64
}

func (data *CompiledBurnEventData) Type() dbsqlc.EventType {
	return dbsqlc.EventTypeBurn
}

type CloseAccountEventData struct {
	Account string
	To      string
}

func (data *CloseAccountEventData) Compile(addresses []*dbsqlc.Address) (CompiledEvent, error) {
	accountId, err := findAccountId(addresses, data.Account)
	if err != nil {
		return nil, err
	}
	toId, err := findAccountId(addresses, data.To)
	if err != nil {
		return nil, err
	}
	return &CompiledCloseAccountEventData{
		Account: accountId,
		To:      toId,
	}, nil
}

type CompiledCloseAccountEventData struct {
	Account int32
	To      int32
}

func (data *CompiledCloseAccountEventData) Type() dbsqlc.EventType {
	return dbsqlc.EventTypeCloseAccount
}
