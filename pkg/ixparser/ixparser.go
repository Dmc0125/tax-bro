package ixparser

import (
	"tax-bro/pkg/dbsqlc"
)

const (
	ErrNotEnoughAccounts    = "not enough accounts"
	ErrMissingDiscriminator = "missing discriminator"
	ErrInvalidData          = "invalid data"
)

type AssociatedAccount struct {
	Address       string
	Type          dbsqlc.AssociatedAccountType
	LastSignature string
}

type CompiledEvent interface {
	Type() dbsqlc.EventType
}

type Event interface {
	Compile([]*dbsqlc.Address) (CompiledEvent, error)
}

type parseResult struct {
	isKnown            bool
	events             []Event
	associatedAccounts map[string]*AssociatedAccount
}

func newParseResult() *parseResult {
	return &parseResult{
		events:             make([]Event, 0),
		associatedAccounts: make(map[string]*AssociatedAccount),
	}
}

func (result *parseResult) appendAssociatedAccount(account *AssociatedAccount) {
	result.associatedAccounts[account.Address] = account
}

func (result *parseResult) intoTuple() (bool, []Event, map[string]*AssociatedAccount) {
	return result.isKnown, result.events, result.associatedAccounts
}

type ParsableInstructionBase interface {
	GetProgramAddress() string
	GetAccounts() []string
	GetData() []byte
}

type ParsableInstruction interface {
	ParsableInstructionBase
	GetInnerInstructions() []ParsableInstructionBase
}

func ParseInstruction(
	ix ParsableInstruction,
	walletAddress, signature string,
) (bool, []Event, map[string]*AssociatedAccount) {
	result := newParseResult()

	switch ix.GetProgramAddress() {
	case systemProgramAddress:
		parseSystemIx(result, ix)
	case tokenProgramAddress:
		parseTokenIx(result, ix, walletAddress)
	case associatedTokenProgramAddress:
		parseAssociatedTokenIx(result, ix, walletAddress, signature)
	}

	return result.intoTuple()
}
