package fuzzer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"tax-bro/pkg/database"
	testutils "tax-bro/pkg/test_utils"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jmoiron/sqlx"
)

const (
	ixSystemTransfer = iota
	ixMintInit
)

type TokenAccount struct {
	balance uint64
	mint    *solana.PublicKey
}

type TokenMint struct {
	authority *solana.Wallet
	address   solana.PublicKey
}

type FuzzWallet struct {
	dbId          int32
	keypair       *solana.Wallet
	tokenAccounts []*TokenAccount
	balance       uint64
}

type FuzzAccount struct {
	dbId    int32
	wallets []*FuzzWallet
}

type AvailableWallet struct {
	wallet   *solana.Wallet
	isUsed   bool
	txsCount uint
}

type Fuzzer struct {
	Ctx       context.Context
	RpcClient *rpc.Client
	Db        *sqlx.DB
	R         *rand.Rand
	M         *sync.Mutex

	TxsRoundTimeout        time.Duration
	UserActionRoundTimeout time.Duration

	AvailableWallets map[string]*AvailableWallet
	Accounts         []*FuzzAccount
	Mints            []*TokenMint
}

func (f *Fuzzer) String(seed int64) string {
	output := strings.Builder{}
	output.WriteString(fmt.Sprintf("Fuzz seed: %d\n", seed))
	output.WriteString("----------------------\n")

	output.WriteString(fmt.Sprintf("Accounts: %d\n\n", len(f.Accounts)))
	for i, account := range f.Accounts {
		output.WriteString(fmt.Sprintf("Account %d Wallets: %d\n", i+1, len(account.wallets)))
	}

	output.WriteString("----------------------\n")

	usedWallets := 0
	for _, w := range f.AvailableWallets {
		if w.isUsed {
			usedWallets++
		}
	}
	output.WriteString(fmt.Sprintf("Blockchain wallets: %d Used: %d\n\n", len(f.AvailableWallets), usedWallets))
	addresses := slices.Sorted(maps.Keys(f.AvailableWallets))
	for _, address := range addresses {
		wallet := f.AvailableWallets[address]
		output.WriteString(fmt.Sprintf("Wallet %s IsUsed %t Txs: %d\n", wallet.wallet.PublicKey().String(), wallet.isUsed, wallet.txsCount))
	}

	return output.String()
}

func (f *Fuzzer) Run() {
	wallet := solana.NewWallet()
	f.AvailableWallets[wallet.PublicKey().String()] = &AvailableWallet{
		wallet: wallet,
		isUsed: false,
	}
	testutils.ExecuteAirdrop(f.Ctx, f.RpcClient, wallet.PublicKey())

	f.createUserAccount()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(f.TxsRoundTimeout)
		for {
			select {
			case <-f.Ctx.Done():
				return
			case <-ticker.C:
				if f.shouldHappen(2, 10) {
					f.M.Lock()
					wallet := solana.NewWallet()
					f.AvailableWallets[wallet.PublicKey().String()] = &AvailableWallet{
						wallet: wallet,
						isUsed: false,
					}
					f.M.Unlock()
					testutils.ExecuteAirdrop(f.Ctx, f.RpcClient, wallet.PublicKey())
				}

				if f.shouldHappen(7, 10) {
					walletsAddresses := slices.Collect(maps.Keys(f.AvailableWallets))
					numWallets := f.R.Intn(len(walletsAddresses))

					var txsWg sync.WaitGroup
					txsWg.Add(numWallets)
					for i := range numWallets {
						go func() {
							wallet := f.AvailableWallets[walletsAddresses[i]]
							f.executeTxForWallet(wallet)
							txsWg.Done()
						}()
					}
					txsWg.Wait()
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(f.UserActionRoundTimeout)
		for {
			select {
			case <-f.Ctx.Done():
				return
			case <-ticker.C:
				if f.shouldHappen(2, 10) {
					f.createUserAccount()
				}

				if f.shouldHappen(5, 10) {
					accountIdx := f.R.Intn(len(f.Accounts))
					account := f.Accounts[accountIdx]
					wallet := f.createWallet(account)

					if f.shouldHappen(5, 10) {
						f.syncWallet(wallet)
					}
				}

				if f.shouldHappen(5, 10) {
					accountsIdxs := []int{}
					for i, a := range f.Accounts {
						if len(a.wallets) > 0 {
							accountsIdxs = append(accountsIdxs, i)
						}
					}

					if len(accountsIdxs) > 0 {
						accountIdx := f.R.Intn(len(accountsIdxs))
						account := f.Accounts[accountsIdxs[accountIdx]]

						walletIdx := f.R.Intn(len(account.wallets))
						wallet := account.wallets[walletIdx]

						f.syncWallet(wallet)
					}
				}

				if f.shouldHappen(2, 10) {
					wallets := []*FuzzWallet{}
					for _, acc := range f.Accounts {
						wallets = append(wallets, acc.wallets...)
					}

					if len(wallets) > 0 {
						idx := f.R.Intn(len(wallets))
						f.deleteSyncRequest(wallets[idx])
					}
				}
			}
		}
	}()

	wg.Wait()
}

func (f *Fuzzer) shouldHappen(odds, total int) bool {
	x := f.R.Intn(total)
	return x <= odds
}

func (f *Fuzzer) executeTxForWallet(wallet *AvailableWallet) {
	rix := f.selectRandomInstructions(wallet.wallet)
	utils.Assert(rix != nil, "invalid ixs selected")

	recentBlockHash, err := f.RpcClient.GetRecentBlockhash(f.Ctx, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)
	tx, err := solana.NewTransaction(rix.ixs, recentBlockHash.Value.Blockhash, solana.TransactionPayer(wallet.wallet.PublicKey()))
	utils.AssertNoErr(err)

	tx.Sign(func(pk solana.PublicKey) *solana.PrivateKey {
		return rix.signers[pk.String()]
	})

	signature, err := f.RpcClient.SendTransactionWithOpts(f.Ctx, tx, rpc.TransactionOpts{SkipPreflight: true})
	utils.AssertNoErr(err)

	ticker := time.NewTicker(200 * time.Millisecond)
	droppedAt := time.Now().Unix() + 100
	txErr := false

	for {
		<-ticker.C
		if droppedAt < time.Now().Unix() {
			return
		}

		signaturesStatuses, err := f.RpcClient.GetSignatureStatuses(f.Ctx, false, signature)
		if errors.Is(err, rpc.ErrNotFound) {
			continue
		}
		utils.AssertNoErr(err)
		if len(signaturesStatuses.Value) == 0 || signaturesStatuses.Value[0] == nil {
			continue
		}
		txResult := signaturesStatuses.Value[0]
		if txResult.ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
			txErr = txResult.Err != nil
			break
		}
	}

	if !txErr && rix.onSuccessFn != nil {
		rix.onSuccessFn()
	}
	f.M.Lock()
	wallet.txsCount += 1
	f.M.Unlock()
}

func (f *Fuzzer) createUserAccount() *FuzzAccount {
	account := new(struct{ Id int32 })
	ts := time.Now().Unix()
	err := f.Db.Get(account, database.QueryWithReturn(database.QueryInsertAccount, "id"), "github", fmt.Sprintf("email%d@email.com", ts))
	utils.AssertNoErr(err)

	fuzzAccount := &FuzzAccount{
		dbId:    account.Id,
		wallets: make([]*FuzzWallet, 0),
	}
	f.M.Lock()
	f.Accounts = append(f.Accounts, fuzzAccount)
	f.M.Unlock()

	return fuzzAccount
}

func (f *Fuzzer) createWallet(fuzzAccount *FuzzAccount) *FuzzWallet {
	unusedWalletsAddresses := []string{}
	for address, wallet := range f.AvailableWallets {
		if !wallet.isUsed {
			unusedWalletsAddresses = append(unusedWalletsAddresses, address)
		}
	}
	if len(unusedWalletsAddresses) == 0 {
		return nil
	}
	idx := f.R.Intn(len(unusedWalletsAddresses))
	f.M.Lock()
	w := f.AvailableWallets[unusedWalletsAddresses[idx]]
	w.isUsed = true
	f.M.Unlock()
	wallet := w.wallet

	address := new(struct{ Id int32 })
	q := "SELECT id FROM address WHERE value = $1"
	err := f.Db.Get(address, q, wallet.PublicKey().String())
	if errors.Is(err, sql.ErrNoRows) {
		err = f.Db.Get(address, database.QueryWithReturn(database.QueryInsertAddress, "id"), wallet.PublicKey().String())
		utils.AssertNoErr(err)
	}
	utils.AssertNoErr(err)

	insertedWallet := new(struct{ Id int32 })
	err = f.Db.Get(insertedWallet, database.QueryWithReturn(database.QueryInsertWallet, "id"), fuzzAccount.dbId, address.Id, nil)
	utils.AssertNoErr(err)

	fuzzWallet := &FuzzWallet{
		dbId:          insertedWallet.Id,
		keypair:       wallet,
		tokenAccounts: make([]*TokenAccount, 0),
	}
	f.M.Lock()
	fuzzAccount.wallets = append(fuzzAccount.wallets, fuzzWallet)
	f.M.Unlock()

	return fuzzWallet
}

func (f *Fuzzer) syncWallet(fuzzWallet *FuzzWallet) {
	_, err := f.Db.Exec(database.QueryInsertSyncWalletRequest, fuzzWallet.dbId)
	if err != nil && strings.Contains(err.Error(), "pq: duplicate key value violates unique constraint") {
		return
	}
	utils.AssertNoErr(err)
}

func (f *Fuzzer) deleteSyncRequest(fuzzWallet *FuzzWallet) {
	_, err := f.Db.Exec("DELETE FROM sync_wallet_request WHERE wallet_id = $1", fuzzWallet.dbId)
	utils.AssertNoErr(err)
}

func newSigners(wallets ...*solana.Wallet) map[string]*solana.PrivateKey {
	signers := make(map[string]*solana.PrivateKey)
	for _, wallet := range wallets {
		signers[wallet.PublicKey().String()] = &wallet.PrivateKey
	}
	return signers
}

type randomInstructionData struct {
	ixs         []solana.Instruction
	signers     map[string]*solana.PrivateKey
	onSuccessFn func()
}

func (f *Fuzzer) selectRandomInstructions(wallet *solana.Wallet) *randomInstructionData {
	instructionsSet := []int{ixSystemTransfer, ixMintInit}
	idx := f.R.Intn(len(instructionsSet))

	switch instructionsSet[idx] {
	case ixSystemTransfer:
		// 0.0001 - 0.001 SOL
		amount := f.R.Intn(900_000 + 100_000)
		allRecipients := []solana.PublicKey{}
		for _, w := range f.AvailableWallets {
			if !w.wallet.PublicKey().Equals(wallet.PublicKey()) {
				allRecipients = append(allRecipients, w.wallet.PublicKey())
			}
		}
		idx := f.R.Intn(len(allRecipients))
		return &randomInstructionData{
			ixs:     []solana.Instruction{system.NewTransferInstruction(uint64(amount), wallet.PublicKey(), allRecipients[idx]).Build()},
			signers: newSigners(wallet),
		}
	case ixMintInit:
		mintAccount, ixs := testutils.CreateInitMintIxs(f.RpcClient, wallet)
		return &randomInstructionData{
			ixs: ixs,
			onSuccessFn: func() {
				f.M.Lock()
				f.Mints = append(f.Mints, &TokenMint{
					address:   mintAccount.PublicKey(),
					authority: wallet,
				})
				f.M.Unlock()
			},
			signers: newSigners(wallet, mintAccount),
		}
	}

	return nil
}
