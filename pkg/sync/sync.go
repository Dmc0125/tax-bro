package sync

import (
	"context"
	"database/sql"
	"errors"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/utils"
	"tax-bro/pkg/walletfetcher"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
)

func Run(ctx context.Context, db *pgx.Conn, q *dbsqlc.Queries, rpcClient *rpc.Client) {
	ticker := time.NewTicker(1 * time.Second)
outer:
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req, err := q.GetLatestSyncWalletRequest(ctx)
			if errors.Is(err, sql.ErrNoRows) {
				continue outer
			}
			utils.AssertNoErr(err, "unable to fetch latest sync wallet request")

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

			msgChan := make(chan *walletfetcher.Message)
			go fetcher.HandleMessages(msgChan)

			fetcher.FetchTransactions(
				walletfetcher.NewFetchTransactionsRequest("", fetcher.WalletAddress, fetcher.LastSignature),
				msgChan,
			)
			for address, associatedAccount := range fetcher.AssociatedAccounts {
				fetcher.FetchTransactions(
					walletfetcher.NewFetchTransactionsRequest(address, "", associatedAccount.LastSignature),
					msgChan,
				)
			}
			close(msgChan)
		}
	}
}
