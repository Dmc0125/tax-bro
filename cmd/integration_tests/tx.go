package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func forceSendTx(
	rpcClient *rpc.Client,
	wallet *solana.Wallet,
	ixs []solana.Instruction,
) {
	var wg sync.WaitGroup
	var lock sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	var signature solana.Signature

	wg.Add(2)
	go func() {
		defer wg.Done()
		blockhash, err := rpcClient.GetLatestBlockhash(
			context.Background(),
			rpc.CommitmentConfirmed,
		)
		if err != nil {
			log.Fatalf("unable to get blockhash: %s", err)
		}
		startTime := time.Now().UnixMilli()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if startTime+int64(90*time.Second*time.Millisecond) > time.Now().UnixMilli() {
					blockhash, err = rpcClient.GetLatestBlockhash(
						context.Background(),
						rpc.CommitmentConfirmed,
					)
					if err != nil {
						log.Fatalf("unable to get blockhash: %s", err)
					}
				}

				tx, err := solana.NewTransaction(
					ixs,
					blockhash.Value.Blockhash,
					solana.TransactionPayer(wallet.PublicKey()),
				)
				if err != nil {
					log.Fatalf("unable to create tx: %s", err)
				}
				sigs, _ := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
					return &wallet.PrivateKey
				})

				retries := uint(0)
				_, err = rpcClient.SendTransactionWithOpts(
					context.Background(),
					tx,
					rpc.TransactionOpts{
						SkipPreflight: true,
						MaxRetries:    &retries,
					},
				)
				if err != nil {
					log.Fatalf("unable to send tx: %s", err)
				}

				lock.Lock()
				if signature == (solana.Signature{}) {
					signature = sigs[0]
				}
				lock.Unlock()
				time.Sleep(2 * time.Second)
			}
		}
	}()

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lock.Lock()
				s := signature
				lock.Unlock()

				if s == (solana.Signature{}) {
					continue
				}

				res, err := rpcClient.GetSignatureStatuses(context.Background(), true, s)
				if errors.Is(err, rpc.ErrNotFound) {
					continue
				}
				if err != nil {
					log.Fatalf("unable to get signature status: %s", err)
				}

				if res.Value[0] == nil {
					continue
				}

				status := res.Value[0].ConfirmationStatus

				if status == rpc.ConfirmationStatusConfirmed {
					cancel()
					return
				}
			}
		}
	}()

	wg.Wait()
}

func forceSendTxs(
	rpcClient *rpc.Client,
	wallet *solana.Wallet,
	txs [][]solana.Instruction,
) {
	var wg sync.WaitGroup
	for _, ixs := range txs {
		wg.Add(1)
		go func() {
			forceSendTx(rpcClient, wallet, ixs)
			wg.Done()
		}()
	}
	wg.Wait()
}
