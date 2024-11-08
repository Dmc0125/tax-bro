package instructionsparser

import (
	"slices"
	"tax-bro/pkg/database"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/jmoiron/sqlx"
)

const (
	ErrNotEnoughAccounts    = "not enough accounts"
	ErrMissingDiscriminator = "missing discriminator"
	ErrInvalidData          = "invalid data"
)

type Event interface {
	Kind() string
	Serialize(database.Accounts) []byte
}

type ParsableInstructionBase interface {
	GetProgramAddress() string
	GetAccountsAddresses() []string
	GetData() []byte
}

type ParsableInstruction interface {
	ParsableInstructionBase
	GetInnerInstructions() []ParsableInstructionBase
	AppendEvents(...Event)
}

type Parser struct {
	WalletAddress      string
	AssociatedAccounts []*AssociatedAccount
}

func New(walletAddress string, walletId int32, db *sqlx.DB) *Parser {
	savedAssociatedAccounts := []*database.AssociatedAccount{}
	q := `
		SELECT address.value as address, signature.value AS last_signature, associated_account.type
		FROM associated_account
		INNER JOIN address ON address.id = associated_account.address_id
		INNER JOIN signature ON signature.id = associated_account.last_signature_id
		WHERE associated_account.wallet_id = $1
	`
	err := db.Select(&savedAssociatedAccounts, q, walletId)
	utils.AssertNoErr(err)

	p := &Parser{
		WalletAddress:      walletAddress,
		AssociatedAccounts: make([]*AssociatedAccount, 0),
	}
	for _, aa := range savedAssociatedAccounts {
		p.AssociatedAccounts = append(p.AssociatedAccounts, NewAssociatedAccountFromSaved(aa))
	}
	return p
}

func (parser *Parser) appendUniqueAssociatedAccounts(associatedAccounts []*AssociatedAccount) []*AssociatedAccount {
	if len(associatedAccounts) == 0 {
		return associatedAccounts
	}
	unique := make([]*AssociatedAccount, 0)
	for _, aa := range associatedAccounts {
		idx := slices.IndexFunc(parser.AssociatedAccounts, func(aaa *AssociatedAccount) bool {
			return aaa.Address == aa.Address
		})
		if idx == -1 {
			unique = append(unique, aa)
		}
	}
	parser.AssociatedAccounts = append(parser.AssociatedAccounts, unique...)
	return unique
}

// Parses events and instructions
// Instruction is mutated - events are appended
func (parser *Parser) Parse(instruction ParsableInstruction, signature string) []*AssociatedAccount {
	associatedAccounts := make([]*AssociatedAccount, 0)

	switch instruction.GetProgramAddress() {
	case system.ProgramID.String():
		parser.parseSystemIx(instruction)
	case token.ProgramID.String():
		associatedAccounts = parser.parseTokenIx(instruction)
	case solana.SPLAssociatedTokenAccountProgramID.String():
		associatedAccounts = parser.parseAssociatedTokenIx(instruction, signature)
	}

	return parser.appendUniqueAssociatedAccounts(associatedAccounts)
}
