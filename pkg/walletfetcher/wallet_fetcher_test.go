package walletfetcher

import (
	"context"
	"tax-bro/pkg/dbsqlc"
	testutils "tax-bro/pkg/test_utils"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var dummyAddresses = []string{
	"program123",
	"wallet123",
	"token123",
	"other456",
	"program789",
	"wallet789",
}
var transactions = []*OnchainTransaction{
	{
		Signature:             "123abc",
		Accounts:              dummyAddresses[:4], // First 4 addresses
		Logs:                  []string{"log1", "log2"},
		Timestamp:             time.Now(),
		TimestampGranularized: time.Now().Truncate(15 * time.Minute),
		Slot:                  1234,
		Err:                   false,
		Fee:                   5000,
		Ixs: []*onchainInstruction{
			{
				onchainInstructionBase: onchainInstructionBase{
					programAddress: "program123",
					accounts:       []string{"wallet123", "token123"},
					data:           []byte("test data"),
				},
				innerIxs: []*onchainInstructionBase{
					{
						programAddress: "other456",
						accounts:       []string{"token123"},
						data:           []byte("inner test data"),
					},
				},
			},
		},
	},
	{
		Signature:             "789xyz",
		Accounts:              []string{"program789", "wallet789", "token123"},
		Logs:                  []string{"log3", "log4", "log5"},
		Timestamp:             time.Now(),
		TimestampGranularized: time.Now().Truncate(15 * time.Minute),
		Slot:                  1235,
		Err:                   false,
		Fee:                   3000,
		Ixs: []*onchainInstruction{
			{
				onchainInstructionBase: onchainInstructionBase{
					programAddress: "program789",
					accounts:       []string{"wallet789", "token123"},
					data:           []byte("second tx data"),
				},
				innerIxs: []*onchainInstructionBase{
					{
						programAddress: "program789",
						accounts:       []string{"token123"},
						data:           []byte("second inner data"),
					},
				},
			},
		},
	},
}

func TestInsertTransactionsData(t *testing.T) {
	db, cleanup := testutils.InitDb()
	defer cleanup()

	q := dbsqlc.New(db)
	rpcClient := rpc.New("http://localhost:8899")

	fetcher := &WalletFetcher{
		ctx:           context.Background(),
		db:            db,
		q:             q,
		rpcClient:     rpcClient,
		WalletId:      1,
		WalletAddress: "123abc",
	}
	addresses, err := fetcher.insertAddresses(dummyAddresses)
	require.NoError(t, err, "Failed to insert addresses")

	_, err = fetcher.insertTransactionsData(addresses, transactions)
	require.NoError(t, err, "Failed to insert transaction data")
}

// func TestHandleTimestampsDuplicates(t *testing.T) {
// 	db, cleanup := testutils.InitDb()
// 	defer cleanup()

// 	ctx := context.Background()
// 	q := dbsqlc.New(db)

// 	addresses, err := q.InsertAddresses(ctx, []string{
// 		"addr1", "addr2", "addr3",
// 	})
// 	require.NoError(t, err)

// 	signatures, err := q.InsertSignatures(ctx, []string{
// 		"sig1", "sig2", "sig3",
// 	})
// 	require.NoError(t, err)

// 	now := time.Now()
// 	_, err = q.InsertTransactions(ctx, []*dbsqlc.InsertTransactionsParams{
// 		{
// 			SignatureID:           signatures[0].ID,
// 			AccountsIds:           []int32{addresses[0].ID},
// 			Timestamp:             pgtype.Timestamptz{Time: now, Valid: true},
// 			TimestampGranularized: pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
// 			Slot:                  100,
// 			Logs:                  []string{"log1"},
// 			Err:                   false,
// 			Fee:                   1000,
// 		},
// 		{
// 			SignatureID:           signatures[1].ID,
// 			AccountsIds:           []int32{addresses[1].ID},
// 			Timestamp:             pgtype.Timestamptz{Time: now, Valid: true},
// 			TimestampGranularized: pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
// 			Slot:                  100,
// 			Logs:                  []string{"log2"},
// 			Err:                   false,
// 			Fee:                   1000,
// 		},
// 		{
// 			SignatureID:           signatures[2].ID,
// 			AccountsIds:           []int32{addresses[2].ID},
// 			Timestamp:             pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
// 			TimestampGranularized: pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
// 			Slot:                  200,
// 			Logs:                  []string{"log3"},
// 			Err:                   false,
// 			Fee:                   1000,
// 		},
// 	})
// 	require.NoError(t, err)

// 	fetcher := &WalletFetcher{
// 		ctx: ctx,
// 		db:  db,
// 		q:   q,
// 	}

// 	err = fetcher.HandleTimestampsDuplicates()
// 	require.NoError(t, err)

// 	txs, err := q.GetTransactionsWithDuplicateTimestamps(ctx, 0)
// 	require.NoError(t, err)
// 	require.Empty(t, txs, "there should be no more transactions with duplicate timestamps")
// }
