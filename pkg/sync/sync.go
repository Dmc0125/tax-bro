package sync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/utils"
	"tax-bro/pkg/walletfetcher"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, db *pgxpool.Pool, q *dbsqlc.Queries, rpcClient *rpc.Client) {
	ticker := time.NewTicker(1 * time.Second)
outer:
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req, err := q.GetLatestSyncWalletRequest(ctx)
			if errors.Is(err, pgx.ErrNoRows) {
				slog.Debug("no sync wallet request in queue")
				continue outer
			}
			slog.Info("starting sync", "req", req)
			utils.AssertNoErr(err, "unable to fetch latest sync wallet request")

			err = q.UpdateSyncWalletRequestStatus(ctx, &dbsqlc.UpdateSyncWalletRequestStatusParams{
				Status:   dbsqlc.SyncWalletRequestStatusFetchingTransactions,
				WalletID: req.WalletID,
			})
			utils.AssertNoErr(err, "unable to update sync wallet request status")

			fetcher := walletfetcher.NewFetcher(
				ctx,
				db,
				q,
				rpcClient,
				req.WalletID,
				req.Address,
				req.LastSignature.String,
			)
			err = fetcher.GetAssociatedAccounts()
			utils.AssertNoErr(err)

			var wg sync.WaitGroup
			wg.Add(1)
			msgChan := make(chan *walletfetcher.Message)
			go fetcher.HandleMessages(&wg, msgChan)

			slog.Info("starting fetching for wallet")
			fetcher.FetchTransactions(
				walletfetcher.NewFetchTransactionsRequest(false, fetcher.WalletAddress, fetcher.LastSignature),
				msgChan,
			)

			slog.Info("starting fetching for associated accounts", "associated accounts", len(fetcher.AssociatedAccounts))
			for address, associatedAccount := range fetcher.AssociatedAccounts {
				fetcher.FetchTransactions(
					walletfetcher.NewFetchTransactionsRequest(true, address, associatedAccount.LastSignature),
					msgChan,
				)
			}
			close(msgChan)
			wg.Wait()

			err = q.UpdateSyncWalletRequestStatus(ctx, &dbsqlc.UpdateSyncWalletRequestStatusParams{
				Status:   dbsqlc.SyncWalletRequestStatusParsingEvents,
				WalletID: req.WalletID,
			})
			utils.AssertNoErr(err, "unable to update sync wallet request status")
		}
	}
}
