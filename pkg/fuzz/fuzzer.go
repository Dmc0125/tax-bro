package fuzz

import (
	"context"
	"math/rand"
	"tax-bro/pkg/database"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
)

type TokenAccount struct {
	balance uint64
	mint    *solana.PublicKey
}

type FuzzWallet struct {
	dbId          int32
	keypair       *solana.Wallet
	tokenMints    []*solana.PublicKey
	tokenAccounts []*TokenAccount
	balance       uint64
}

type FuzzAccount struct {
	dbId    int32
	wallets []*FuzzWallet
}

type Fuzzer struct {
	ctx       context.Context
	rpcClient *rpc.Client
	db        *sqlx.DB
	r         *rand.Rand

	accounts []*FuzzAccount
}

func NewFuzzer(ctx context.Context, rpcClient *rpc.Client, db *sqlx.DB, seed int64) *Fuzzer {
	return &Fuzzer{
		ctx:       ctx,
		db:        db,
		rpcClient: rpcClient,
		r:         rand.New(rand.NewSource(seed)),
		accounts:  make([]*FuzzAccount, 0),
	}
}

func (f *Fuzzer) run() {

}

func (f *Fuzzer) createUserAccount() *FuzzAccount {
	account := new(struct{ Id int32 })
	err := f.db.Get(account, database.QueryWithReturn(database.QueryInsertAccount, "id"), "github", "email@email.com")
	utils.AssertNoErr(err)

	fuzzAccount := &FuzzAccount{
		dbId:    account.Id,
		wallets: make([]*FuzzWallet, 0),
	}
	f.accounts = append(f.accounts, fuzzAccount)

	return fuzzAccount
}

func (f *Fuzzer) createWallet(fuzzAccount *FuzzAccount) *FuzzWallet {
	wallet := solana.NewWallet()
	address := new(struct{ Id int32 })
	err := f.db.Get(address, database.QueryWithReturn(database.QueryInsertAddress, "id"), wallet.PublicKey().String())
	utils.AssertNoErr(err)

	insertedWallet := new(struct{ Id int32 })
	err = f.db.Get(insertedWallet, database.QueryWithReturn(database.QueryInsertWallet, "id"), fuzzAccount.dbId, address.Id, nil)
	utils.AssertNoErr(err)

	fuzzWallet := &FuzzWallet{
		dbId:          insertedWallet.Id,
		keypair:       wallet,
		tokenMints:    make([]*solana.PublicKey, 0),
		tokenAccounts: make([]*TokenAccount, 0),
	}
	fuzzAccount.wallets = append(fuzzAccount.wallets, fuzzWallet)

	return fuzzWallet
}

func (f *Fuzzer) syncWallet(fuzzWallet *FuzzWallet) {
	_, err := f.db.Exec(database.QueryInsertSyncWalletRequest, fuzzWallet.dbId)
	utils.AssertNoErr(err)
}
