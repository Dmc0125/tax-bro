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
) ([]Event, map[string]*AssociatedAccount) {
	switch ix.GetProgramAddress() {
	case systemProgramAddress:
		event := parseSystemIx(ix)
		events := make([]Event, 0)
		if event != nil {
			events = append(events, event)
		}
		return events, make(map[string]*AssociatedAccount)
	case tokenProgramAddress:
		return parseTokenIx(ix, walletAddress)
	case associatedTokenProgramAddress:
		return parseAssociatedTokenIx(ix, walletAddress, signature)
	}

	return make([]Event, 0), make(map[string]*AssociatedAccount)
}
