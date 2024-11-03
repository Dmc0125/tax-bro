package instructionsparser

import (
	"log/slog"
	"os"
	"slices"
	"tax-bro/pkg/database"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
)

const (
	errNotEnoughAccounts    = "not enough accounts"
	errMissingDiscriminator = "missing discriminator"
	errInvalidData          = "invalid data"
)

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

type Event interface {
	Kind() string
	Serialize(database.Accounts) []byte
}

type AssociatedAccountKind uint8

func (kind AssociatedAccountKind) String() string {
	switch kind {
	case 0:
		return "token"
	}
	slog.Error("invalid associated account kind", "value", kind)
	os.Exit(1)
	return ""
}

const (
	TokenAssociatedAccount AssociatedAccountKind = 0
)

type AssociatedAccount struct {
	Kind          AssociatedAccountKind
	Address       string
	LastSignature string
}

func newAssociatedAccountFromSaved(saved *database.AssociatedAccount) *AssociatedAccount {
	var kind AssociatedAccountKind
	switch saved.Type {
	case "token":
		kind = 0
	default:
		slog.Error("invalid associated account type", "value", saved.Type)
		os.Exit(1)
	}

	return &AssociatedAccount{
		Kind:          kind,
		Address:       saved.Address,
		LastSignature: saved.LastSignature.String,
	}
}

func (a *AssociatedAccount) GetFetchSignaturesConfig() (solana.PublicKey, *rpc.GetSignaturesForAddressOpts) {
	pk, err := solana.PublicKeyFromBase58(a.Address)
	utils.AssertNoErr(err)

	limit := int(1000)
	opts := &rpc.GetSignaturesForAddressOpts{
		Limit:      &limit,
		Commitment: rpc.CommitmentConfirmed,
	}
	if len(a.LastSignature) > 0 {
		opts.Until, err = solana.SignatureFromBase58(a.LastSignature)
		utils.AssertNoErr(err)
	}

	return pk, opts
}

func (a *AssociatedAccount) IsWallet() bool {
	return false
}

type Parser struct {
	walletAddress      string
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
		walletAddress:      walletAddress,
		AssociatedAccounts: make([]*AssociatedAccount, 0),
	}
	for _, aa := range savedAssociatedAccounts {
		p.AssociatedAccounts = append(p.AssociatedAccounts, newAssociatedAccountFromSaved(aa))
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
