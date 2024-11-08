package instructionsparser

import (
	"log/slog"
	"os"
	"tax-bro/pkg/database"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

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

func NewAssociatedAccountFromSaved(saved *database.AssociatedAccount) *AssociatedAccount {
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
