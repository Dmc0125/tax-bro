package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"tax-bro/pkg/utils"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	InstrTypeSystemTransfer         = "system_transfer"
	InstrTypeSystemTransferWithSeed = "system_transfer_with_seed"
	InstrTypeSystemCreate           = "system_create"
	InstrTypeSystemCreateWithSeed   = "system_create_with_seed"

	InstrTypeTokenTransfer     = "token_transfer"
	InstrTypeTokenMint         = "token_mint"
	InstrTypeTokenBurn         = "token_burn"
	InstrTypeTokenInitAccount  = "token_init_account"
	InstrTypeTokenInitMint     = "token_init_mint"
	InstrTypeTokenCloseAccount = "token_close_account"

	InstrTypeATACreate = "ata_create"
)

type TokenAccount struct {
	address string
	amount  uint64
	owner   *solana.Wallet
	mint    string
}

type WalletsFuzzer struct {
	ctx       context.Context
	wallet    *solana.Wallet
	rpcClient *rpc.Client
	r         *rand.Rand
	interval  time.Duration

	totalRounds        atomic.Int32
	totalTxsSent       atomic.Int32
	totalTxsConfirmed  atomic.Int32
	totalTxsSuccessful atomic.Int32

	mints               map[string][]*TokenAccount
	allowedInstructions map[string]bool
}

func NewFuzzer(ctx context.Context, wallet *solana.Wallet, rpcClient *rpc.Client, interval time.Duration) *WalletsFuzzer {
	f := &WalletsFuzzer{
		ctx:                 ctx,
		wallet:              wallet,
		rpcClient:           rpcClient,
		r:                   rand.New(rand.NewSource(time.Now().UnixNano())),
		mints:               make(map[string][]*TokenAccount),
		allowedInstructions: make(map[string]bool),
		interval:            interval,
	}

	f.allowedInstructions[InstrTypeSystemTransfer] = true
	f.allowedInstructions[InstrTypeSystemTransferWithSeed] = true
	f.allowedInstructions[InstrTypeSystemCreate] = true
	f.allowedInstructions[InstrTypeSystemCreateWithSeed] = true
	f.allowedInstructions[InstrTypeTokenInitMint] = true

	return f
}

func (f *WalletsFuzzer) String() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Rounds: %d\n", f.totalRounds.Load()))

	txsConfirmed := f.totalTxsConfirmed.Load()
	txsSuccessful := f.totalTxsSuccessful.Load()
	successRate := float64(txsSuccessful) / float64(txsConfirmed)
	builder.WriteString(fmt.Sprintf(
		"Txs sent, confirmed, successful, success rate: %d, %d, %d, %.2f\n",
		f.totalTxsSent.Load(),
		txsConfirmed,
		txsSuccessful,
		successRate,
	))
	builder.WriteString(fmt.Sprintf("Wallets: %d\n", 1))
	builder.WriteString("-------------------------\n\n")
	builder.WriteString(fmt.Sprintf("Wallet 1: %s\n", f.wallet.PublicKey().String()))

	return builder.String()
}

func (f *WalletsFuzzer) airdrop() {
	balanceRes, err := f.rpcClient.GetBalance(f.ctx, f.wallet.PublicKey(), rpc.CommitmentConfirmed)
	if balanceRes.Value > 10_000_000_000 {
		return
	}

	signature, err := f.rpcClient.RequestAirdrop(f.ctx, f.wallet.PublicKey(), 1000_000_000_000, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	for {
		res, err := f.rpcClient.GetSignatureStatuses(f.ctx, false, signature)
		if errors.Is(err, rpc.ErrNotFound) {
			continue
		}
		utils.AssertNoErr(err)
		txRes := res.Value[0]
		if txRes != nil && txRes.ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
			return
		}
	}
}

func (f *WalletsFuzzer) sendAndConfirmTransaction(ixs []solana.Instruction, blockhash solana.Hash, signers Signers) (bool, error) {
	tx, err := solana.NewTransaction(
		ixs,
		blockhash,
		solana.TransactionPayer(f.wallet.PublicKey()),
	)
	utils.AssertNoErr(err)

	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		return &signers.find(key).PrivateKey
	})

	signature, err := f.rpcClient.SendTransactionWithOpts(f.ctx, tx, rpc.TransactionOpts{
		SkipPreflight: true,
	})
	utils.AssertNoErr(err)

	start := time.Now().Unix()
	ticker := time.NewTicker(1 * time.Second)

	for {
		<-ticker.C

		if time.Now().Unix() > start+100 {
			return false, errors.New("unable to confirm")
		}

		res, err := f.rpcClient.GetSignatureStatuses(f.ctx, false, signature)
		if errors.Is(err, rpc.ErrNotFound) {
			continue
		}
		utils.AssertNoErr(err)
		txRes := res.Value[0]
		if txRes.ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
			return txRes.Err == nil, nil
		}
	}
}

type Signers []*solana.Wallet

func (signers *Signers) append(newSigners ...*solana.Wallet) {
	for _, signer := range newSigners {
		idx := slices.IndexFunc(*signers, func(s *solana.Wallet) bool {
			return s.PublicKey().Equals(signer.PublicKey())
		})
		if idx == -1 {
			(*signers) = append(*signers, signer)
		}
	}
}

func (signers Signers) find(pk solana.PublicKey) *solana.Wallet {
	idx := slices.IndexFunc(signers, func(s *solana.Wallet) bool {
		return s.PublicKey().Equals(pk)
	})
	utils.Assert(idx > -1, "unable to find signer")
	return signers[idx]
}

func (f *WalletsFuzzer) runN(n int) {
	resChan := make(chan []func(), n)
	recentBlockHash, err := f.rpcClient.GetRecentBlockhash(f.ctx, rpc.CommitmentConfirmed)
	utils.AssertNoErr(err)

	go func() {
		var wg sync.WaitGroup
		wg.Add(n)

		for range n {
			select {
			case <-f.ctx.Done():
				return
			default:
				go func() {
					defer wg.Done()
					ixsCount := f.r.Intn(4 + 1)
					ixs := make([]solana.Instruction, ixsCount)
					signersUnique := Signers{}
					execIfSuccess := []func(){}
					for i := range ixsCount {
						ix, signers, fn, err := f.createRandomInstruction()
						if err != nil {
							slog.Warn("unable to create random ix", "err", err)
							resChan <- nil
							continue
						}
						execIfSuccess = append(execIfSuccess, fn)
						ixs[i] = ix
						signersUnique.append(signers...)
					}

					if len(ixs) == 0 {
						slog.Warn("unable to send transaction, ixs is empty")
						resChan <- nil
						return
					}

					f.totalTxsSent.Add(1)
					wasSuccessful, err := f.sendAndConfirmTransaction(ixs, recentBlockHash.Value.Blockhash, signersUnique)
					utils.AssertNoErr(err)
					f.totalTxsConfirmed.Add(1)

					if wasSuccessful {
						f.totalTxsSuccessful.Add(1)
						resChan <- execIfSuccess
					}
				}()
			}
		}
		wg.Wait()
		close(resChan)
	}()

	for fns := range resChan {
		for _, fn := range fns {
			if fn != nil {
				fn()
			}
		}
	}
	f.totalRounds.Add(int32(n))
}

func (f *WalletsFuzzer) Run(initRounds int) {
	f.airdrop()
	if initRounds > 0 {
		f.runN(initRounds)
		slog.Info("Executed initial rounds", "count", initRounds)
	}

	ticker := time.NewTicker(f.interval)
	for {
		select {
		case <-f.ctx.Done():
			return
		default:
			<-ticker.C
			f.airdrop()
			f.runN(5)
			slog.Info("Executed tx round", "count", f.totalRounds.Load())
		}
	}
}

func (f *WalletsFuzzer) createRandomInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func(), err error) {
	var availableInstructions []string
	for instr, allowed := range f.allowedInstructions {
		if allowed {
			availableInstructions = append(availableInstructions, instr)
		}
	}

	idx := f.r.Intn(len(availableInstructions))
	selected := availableInstructions[idx]

	switch selected {
	case InstrTypeTokenInitMint:
		slog.Info("Creating TOKEN initMint ix")
		ix, signers, onSuccess = f.createTokenInitMintInstruction()
	case InstrTypeATACreate:
		slog.Info("Creating AT initAta ix")
		ix, signers, onSuccess = f.createInitATAInstruction()
	case InstrTypeTokenMint:
		slog.Info("Creating TOKEN mint ix")
		ix, signers, onSuccess, err = f.createTokenMintInstruction()
	case InstrTypeTokenBurn:
		slog.Info("Creating TOKEN burn ix")
		ix, signers, onSuccess, err = f.createTokenBurnInstruction()
	case InstrTypeTokenTransfer:
		slog.Info("Creating TOKEN transfer ix")
		ix, signers, onSuccess = f.createTokenTransferInstruction()
	case InstrTypeTokenCloseAccount:
		slog.Info("Creating TOKEN closeAccount ix")
		ix, signers, onSuccess = f.createTokenCloseAccountInstruction()
	case InstrTypeSystemCreateWithSeed:
		fallthrough
	case InstrTypeSystemCreate:
		slog.Info("Creating SYSTEM create ix")
		ix, signers = f.createSystemCreateInstruction()
	case InstrTypeSystemTransferWithSeed:
		fallthrough
	case InstrTypeSystemTransfer:
		fallthrough
	default:
		slog.Info("Creating SYSTEM transfer ix")
		ix, signers = f.createSystemTransferInstruction()
	}

	return
}

func (f *WalletsFuzzer) createSystemTransferInstruction() (ix solana.Instruction, signers []*solana.Wallet) {
	solAmount := f.r.Intn(9) + 1
	lamports := uint64(solAmount) * uint64(math.Pow10(9))
	funder := f.wallet
	receiver := solana.NewWallet().PublicKey()

	ix = system.NewTransferInstruction(lamports, funder.PublicKey(), receiver).Build()
	signers = append(signers, funder)
	return
}

// func (f *TransactionsFuzzer) createSystemTransferWithSeedInstruction() solana.Instruction {
// 	solAmount := f.r.Intn(10)
// 	lamports := uint64(solAmount) * uint64(math.Pow10(9))
// 	base := f.wallet.PublicKey()
// 	seed := fmt.Sprintf("fuzzer%d", f.r.Int63())
// 	programID := system.ProgramID

// 	derivedAddress, err := solana.CreateWithSeed(base, seed, programID)
// 	utils.AssertNoErr(err)

// 	return system.NewTransferWithSeedInstruction(
// 		lamports,
// 		seed,
// 		f.wallet.PublicKey(),
// 		solana.NewWallet().PublicKey(),
// 		base,
// 		derivedAddress,
// 	).Build()
// }

func (f *WalletsFuzzer) createSystemCreateInstruction() (ix solana.Instruction, signers []*solana.Wallet) {
	byteSize := uint64(f.r.Intn(1000)) + 100
	lamports, err := f.rpcClient.GetMinimumBalanceForRentExemption(
		f.ctx,
		byteSize,
		rpc.CommitmentConfirmed,
	)
	utils.AssertNoErr(err)

	funder := f.wallet
	newAccount := solana.NewWallet()

	ix = system.NewCreateAccountInstruction(
		lamports,
		byteSize,
		system.ProgramID,
		funder.PublicKey(),
		newAccount.PublicKey(),
	).Build()
	signers = append(signers, funder, newAccount)
	return
}

// func (f *TransactionsFuzzer) createSystemCreateWithSeedInstruction() solana.Instruction {
// 	byteSize := uint64(f.r.Intn(1000))
// 	lamports, err := f.rpcClient.GetMinimumBalanceForRentExemption(
// 		f.ctx,
// 		byteSize,
// 		rpc.CommitmentConfirmed,
// 	)
// 	utils.AssertNoErr(err)

// 	base := f.wallet.PublicKey()
// 	seed := fmt.Sprintf("fuzzer%d", f.r.Int63())
// 	newAccount := solana.NewWallet().PublicKey()

// 	return system.NewCreateAccountWithSeedInstruction(
// 		base,
// 		seed,
// 		lamports,
// 		byteSize,
// 		f.wallet.PublicKey(),
// 		f.wallet.PublicKey(),
// 		newAccount,
// 		base,
// 	).Build()
// }

// account first has to be created
func (f *WalletsFuzzer) createTokenInitMintInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func()) {
	mintAccount := solana.NewWallet()
	ix = token.NewInitializeMintInstruction(
		9,
		f.wallet.PublicKey(),
		solana.PublicKey{},
		mintAccount.PublicKey(),
		solana.SysVarRentPubkey,
	).Build()
	signers = append(signers, f.wallet)
	onSuccess = func() {
		f.mints[mintAccount.PublicKey().String()] = make([]*TokenAccount, 0)
		f.allowedInstructions[InstrTypeATACreate] = true
	}
	return
}

func (f *WalletsFuzzer) createInitATAInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func()) {
	mintsKeys := slices.Collect(maps.Keys(f.mints))
	mintIdx := f.r.Intn(len(mintsKeys))
	mint := mintsKeys[mintIdx]
	mintPk := solana.MustPublicKeyFromBase58(mint)

	// random owner
	owner := f.wallet
	ata, _, err := solana.FindAssociatedTokenAddress(owner.PublicKey(), mintPk)
	utils.AssertNoErr(err)

	accounts := solana.AccountMetaSlice{
		{IsSigner: true, IsWritable: true, PublicKey: f.wallet.PublicKey()},
		{IsWritable: true, PublicKey: ata},
		{PublicKey: owner.PublicKey()},
		{PublicKey: mintPk},
		{PublicKey: token.ProgramID},
	}
	disc := byte(0)
	if f.r.Intn(10) < 3 {
		disc = 1
	}
	ix = solana.NewInstruction(solana.SPLAssociatedTokenAccountProgramID, accounts, []byte{disc})
	signers = append(signers, f.wallet, owner)
	onSuccess = func() {
		f.allowedInstructions[InstrTypeTokenMint] = true
		f.mints[mint] = append(f.mints[mint], &TokenAccount{
			address: ata.String(),
			owner:   owner,
			mint:    mint,
		})
	}
	return
}

func (f *WalletsFuzzer) createTokenMintInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func(), err error) {
	atas := []*TokenAccount{}
	for _, tokenAccounts := range f.mints {
		atas = append(atas, tokenAccounts...)
	}
	if len(atas) == 0 {
		err = errors.New("atas is empty")
		return
	}
	ataIdx := f.r.Intn(len(atas))
	ata := atas[ataIdx]
	amount := uint64(f.r.Intn(1000000) + 1)

	ix = token.NewMintToCheckedInstruction(
		amount,
		9,
		solana.MustPublicKeyFromBase58(ata.address),
		solana.MustPublicKeyFromBase58(ata.address),
		f.wallet.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	signers = append(signers, f.wallet)
	onSuccess = func() {
		f.allowedInstructions[InstrTypeTokenBurn] = true
		f.allowedInstructions[InstrTypeSystemTransfer] = true
		ata.amount += amount
	}

	return
}

func (f *WalletsFuzzer) createTokenBurnInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func(), err error) {
	// use empty atas too so there is a chance for error
	atas := []*TokenAccount{}
	for _, tokenAccounts := range f.mints {
		atas = append(atas, tokenAccounts...)
	}
	if len(atas) == 0 {
		err = errors.New("atas is empty")
		return
	}

	ataIdx := f.r.Intn(len(atas))
	ata := atas[ataIdx]

	amount := uint64(f.r.Intn(int(ata.amount) + 100))
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, amount)

	ix = solana.NewInstruction(token.ProgramID, []*solana.AccountMeta{
		{IsWritable: true, PublicKey: solana.MustPublicKeyFromBase58(ata.address)},
		{IsWritable: true, PublicKey: f.wallet.PublicKey()},
		{IsSigner: true, PublicKey: ata.owner.PublicKey()},
	}, data)
	signers = append(signers, f.wallet, ata.owner)
	onSuccess = func() {
		f.allowedInstructions[InstrTypeTokenCloseAccount] = true
		ata.amount -= amount
	}

	return
}

func (f *WalletsFuzzer) createTokenTransferInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func()) {
	fromAtas := []*TokenAccount{}
	for _, tokenAccounts := range f.mints {
		fromAtas = append(fromAtas, tokenAccounts...)
	}
	fromIdx := f.r.Intn(len(fromAtas))
	fromAta := fromAtas[fromIdx]

	toAtas := []*TokenAccount{}
	for _, tokenAccounts := range f.mints {
		for _, ata := range tokenAccounts {
			if ata.address != fromAta.address {
				toAtas = append(toAtas, ata)
			}
		}
	}
	toIdx := f.r.Intn(len(toAtas))
	toAta := toAtas[toIdx]

	amount := uint64(f.r.Intn(int(fromAta.amount) + 100))
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, amount)
	accounts := []*solana.AccountMeta{
		{IsWritable: true, PublicKey: solana.MustPublicKeyFromBase58(fromAta.address)},
		{IsWritable: true, PublicKey: solana.MustPublicKeyFromBase58(toAta.address)},
		{IsSigner: true, PublicKey: fromAta.owner.PublicKey()},
	}
	ix = solana.NewInstruction(token.ProgramID, accounts, data)
	signers = append(signers, f.wallet, fromAta.owner)
	onSuccess = func() {
		fromAta.amount -= amount
		toAta.amount += amount
	}

	return
}

func (f *WalletsFuzzer) createTokenCloseAccountInstruction() (ix solana.Instruction, signers []*solana.Wallet, onSuccess func()) {
	atas := []*TokenAccount{}
	for _, tokenAccounts := range f.mints {
		atas = append(atas, tokenAccounts...)
	}
	ataIdx := f.r.Intn(len(atas))
	ata := atas[ataIdx]

	accounts := []*solana.AccountMeta{
		{IsWritable: true, PublicKey: solana.MustPublicKeyFromBase58(ata.address)},
		{IsWritable: true, PublicKey: f.wallet.PublicKey()},
		{IsSigner: true, PublicKey: ata.owner.PublicKey()},
	}
	ix = solana.NewInstruction(token.ProgramID, accounts, []byte{})
	signers = append(signers, f.wallet, ata.owner)
	onSuccess = func() {
		ata = nil
	}

	return
}
