package instructionsparser

import "github.com/gagliardetto/solana-go/programs/system"

type ParsableInstruction interface {
	GetProgramAddress() string
	GetAccountAddress(idx int) string
	GetData() []byte
}

type EventData interface {
	Serialize() string
}

type Event struct {
	Kind int
	Data EventData
}

type AssociatedAccount struct {
	Kind    int
	Address string
}

// passing a slice of interface this way works for some reason
func Parse[T ParsableInstruction](
	instruction ParsableInstruction,
	innerInstructions []T,
) ([]Event, []AssociatedAccount) {
	events := make([]Event, 0)
	associatedAccounts := make([]AssociatedAccount, 0)

	switch instruction.GetProgramAddress() {
	case system.ProgramID.String():
		//
	}

	return events, associatedAccounts
}
