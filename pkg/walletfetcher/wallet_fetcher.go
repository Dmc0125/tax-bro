package walletfetcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/ixparser"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SavedInstructionBase struct {
	ProgramAddress string `json:"program_address"`
	Accounts       []*dbsqlc.Address
	Data           string
}

func (ix *SavedInstructionBase) GetProgramAddress() string {
	return ix.ProgramAddress
}

func (ix *SavedInstructionBase) GetAccounts() (accounts []string) {
	for _, a := range ix.Accounts {
		accounts = append(accounts, a.Value)
	}
	return
}

func (ix *SavedInstructionBase) GetData() []byte {
	bytes, err := base64.RawStdEncoding.DecodeString(ix.Data)
	utils.AssertNoErr(err)
	return bytes
}

type savedInstruction struct {
	*SavedInstructionBase
	InnerIxs []*SavedInstructionBase `json:"inner_ixs"`
}

func (ix *savedInstruction) GetInnerInstructions() (parsableInnerIxs []ixparser.ParsableInstructionBase) {
	for _, innerIx := range ix.InnerIxs {
		parsableInnerIxs = append(parsableInnerIxs, innerIx)
	}
	return
}

// func (ix *savedInstruction) parseAssociatedAccounts(walletAddress, signature string) map[string]*ixparser.AssociatedAccount {
// 	// TODO: if ix is not known, we can attempt to parse events
// 	_, associatedAccounts := ixparser.ParseInstruction(ix, walletAddress, signature)
// 	return associatedAccounts
// }

type savedTransaction struct {
	*dbsqlc.GetTransactionsFromSignaturesRow
	Ixs []*savedInstruction
}

type FetchTransactionsRequest struct {
	isAssociatedAccount bool
	address             string
	pubkey              solana.PublicKey
	config              *rpc.GetSignaturesForAddressOpts
}

func NewFetchTransactionsRequest(
	isAssociatedAccount bool, address, lastSignature string,
) *FetchTransactionsRequest {
	req := new(FetchTransactionsRequest)
	req.isAssociatedAccount = isAssociatedAccount
	req.address = address

	pk, err := solana.PublicKeyFromBase58(address)
	utils.AssertNoErr(err, "invalid wallet or associated account address")
	req.pubkey = pk

	limit := int(1000)
	config := &rpc.GetSignaturesForAddressOpts{
		Limit:      &limit,
		Commitment: rpc.CommitmentConfirmed,
	}
	if lastSignature != "" {
		until, err := solana.SignatureFromBase58(lastSignature)
		utils.AssertNoErr(err, "invalid last signature")
		config.Until = until
	}
	req.config = config

	return req
}

type Message struct {
	associatedAccountAddress string
	lastSignature            string

	associatedAccounts  map[string]*ixparser.AssociatedAccount
	savedTransactions   []*savedTransaction
	unsavedTransactions []*OnchainTransaction
}

func NewMessage(
	isAssociatedAccount bool,
	associatedAccountAddress string,
	isLastIter bool,
	lastSignature string,
) *Message {
	msg := new(Message)
	if isAssociatedAccount {
		msg.associatedAccountAddress = associatedAccountAddress
	}
	if isLastIter {
		msg.lastSignature = lastSignature
	}
	msg.associatedAccounts = make(map[string]*ixparser.AssociatedAccount)
	msg.unsavedTransactions = make([]*OnchainTransaction, 0)
	msg.savedTransactions = make([]*savedTransaction, 0)
	return msg
}

type WalletFetcher struct {
	ctx       context.Context
	db        *pgxpool.Pool
	q         *dbsqlc.Queries
	rpcClient *rpc.Client

	WalletId      int32
	WalletAddress string
	LastSignature string

	AssociatedAccounts map[string]*ixparser.AssociatedAccount
}

func NewFetcher(
	ctx context.Context,
	db *pgxpool.Pool,
	q *dbsqlc.Queries,
	rpcClient *rpc.Client,
	WalletId int32,
	WalletAddress, LastSignature string,
) *WalletFetcher {
	return &WalletFetcher{
		ctx:                ctx,
		db:                 db,
		q:                  q,
		rpcClient:          rpcClient,
		WalletId:           WalletId,
		WalletAddress:      WalletAddress,
		LastSignature:      LastSignature,
		AssociatedAccounts: make(map[string]*ixparser.AssociatedAccount),
	}
}

func (fetcher *WalletFetcher) GetAssociatedAccounts() error {
	associatedAccounts, err := fetcher.q.GetAssociatedAccountsForWallet(fetcher.ctx, fetcher.WalletId)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, associatedAccount := range associatedAccounts {
		fetcher.AssociatedAccounts[associatedAccount.Address] = &ixparser.AssociatedAccount{
			Address:       associatedAccount.Address,
			LastSignature: associatedAccount.LastSignature.String,
		}
	}
	return nil
}

func (fetcher *WalletFetcher) querySavedTransactions(signatures []*rpc.TransactionSignature) ([]*savedTransaction, error) {
	qSignatures := make([]string, len(signatures))
	for i, signature := range signatures {
		qSignatures[i] = signature.Signature.String()
	}
	savedTransactionsRows, err := fetcher.q.GetTransactionsFromSignatures(fetcher.ctx, qSignatures)
	if err != nil {
		return nil, fmt.Errorf("unable to query transactions from signatures: %w", err)
	}
	savedTransactions := make([]*savedTransaction, len(savedTransactionsRows))
	for i, tx := range savedTransactionsRows {
		ixs := []*savedInstruction{}
		if err = json.Unmarshal(tx.Ixs, &ixs); err != nil {
			slog.Error("unmarshal error", "ixs", string(tx.Ixs))
			return nil, fmt.Errorf("unable to unmarshal instructions: %w", err)
		}
		savedTransactions[i] = &savedTransaction{
			GetTransactionsFromSignaturesRow: tx,
			Ixs:                              ixs,
		}
	}
	return savedTransactions, nil
}

func (fetcher *WalletFetcher) fetchTransaction(signature solana.Signature) (*OnchainTransaction, error) {
	txRes, err := utils.CallRpcWithRetries(func() (*rpc.GetTransactionResult, error) {
		v := uint64(0)
		return fetcher.rpcClient.GetTransaction(fetcher.ctx, signature, &rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &v,
		})
	}, 5)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch transactions: %w", err)
	}
	tx, err := txRes.Transaction.GetTransaction()
	if err != nil {
		return nil, fmt.Errorf("unable to get transaction: %w", err)
	}
	msg := &tx.Message
	meta := txRes.Meta
	return DecompileOnchainTransaction(signature.String(), txRes.Slot, txRes.BlockTime.Time(), msg, meta), nil
}

func (fetcher *WalletFetcher) FetchTransactions(request *FetchTransactionsRequest, msgChan chan *Message) {
	config := request.config
	newLastSignature := ""
	address := request.address

	for {
		select {
		case <-fetcher.ctx.Done():
			return
		default:
			signatures, err := utils.CallRpcWithRetries(func() ([]*rpc.TransactionSignature, error) {
				return fetcher.rpcClient.GetSignaturesForAddressWithOpts(fetcher.ctx, request.pubkey, config)
			}, 5)
			utils.AssertNoErr(err, "unable to fetch signatures")
			slog.Info("fetched signatures", "signatures", len(signatures))

			if len(signatures) == 0 && newLastSignature != "" {
				slog.Debug("sending last message")
				msg := NewMessage(request.isAssociatedAccount, address, true, newLastSignature)
				msgChan <- msg
				return
			} else if len(signatures) == 0 {
				slog.Debug("last msg is empty")
				return
			}
			if newLastSignature == "" {
				newLastSignature = signatures[0].Signature.String()
			}

			isLastIter := len(signatures) < int(*config.Limit)
			msg := NewMessage(request.isAssociatedAccount, address, isLastIter, newLastSignature)

			msg.savedTransactions, err = fetcher.querySavedTransactions(signatures)
			utils.AssertNoErr(err)

			if !request.isAssociatedAccount {
				for _, tx := range msg.savedTransactions {
					if tx.Err {
						continue
					}
					for _, ix := range tx.Ixs {
						_, _, associatedAccounts := ixparser.ParseInstruction(ix, address, tx.Signature)
						maps.Insert(msg.associatedAccounts, maps.All(associatedAccounts))
					}
				}
				slog.Info("fetched and parsed saved transactions", "saved transactions", len(msg.savedTransactions))
			}

			for i, signature := range signatures {
				if fetcher.ctx.Err() != nil {
					return
				}
				s := signature.Signature
				txIdx := slices.IndexFunc(msg.savedTransactions, func(tx *savedTransaction) bool {
					return tx.Signature == s.String()
				})
				if txIdx > -1 {
					continue
				}
				slog.Debug("fetching onchain transaction", "idx", i+1)
				tx, err := fetcher.fetchTransaction(s)
				utils.AssertNoErr(err, "unable to fetch transaction")

				if !tx.Err {
					for _, ix := range tx.Ixs {
						associatedAccounts := ix.parse(address, tx.Signature)
						if !request.isAssociatedAccount {
							maps.Insert(msg.associatedAccounts, maps.All(associatedAccounts))
						}
					}
				}

				msg.unsavedTransactions = append(msg.unsavedTransactions, tx)
			}
			slog.Info("fetched and parsed onchain transactions", "onchain transactions", len(msg.unsavedTransactions))

			maps.Insert(fetcher.AssociatedAccounts, maps.All(msg.associatedAccounts))
			msgChan <- msg
			if isLastIter {
				slog.Info("sending last message")
				return
			}
		}
	}
}

func (fetcher *WalletFetcher) insertAddresses(addresses []string) ([]*dbsqlc.Address, error) {
	if len(addresses) == 0 {
		return make([]*dbsqlc.Address, 0), nil
	}

	tx, err := fetcher.db.Begin(fetcher.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	qtx := fetcher.q.WithTx(tx)
	savedAddresses, err := qtx.GetAddressesFromAddresses(fetcher.ctx, addresses)
	if err != nil {
		return nil, err
	}

	out := make([]*dbsqlc.Address, len(savedAddresses))
	for i, a := range savedAddresses {
		out[i] = &dbsqlc.Address{
			Value: a.Value,
			ID:    a.ID,
		}
	}
	if len(out) == len(addresses) {
		return out, nil
	}

	insertable := make([]string, 0)
	for _, address := range addresses {
		contains := slices.ContainsFunc(savedAddresses, func(a *dbsqlc.GetAddressesFromAddressesRow) bool {
			return a.Value == address
		})
		if !contains {
			insertable = append(insertable, address)
		}
	}

	insertedAddresses, err := qtx.InsertAddresses(fetcher.ctx, insertable)
	if err != nil {
		return nil, err
	}
	for _, a := range insertedAddresses {
		out = append(out, &dbsqlc.Address{
			Value: a.Value,
			ID:    a.ID,
		})
	}

	err = tx.Commit(fetcher.ctx)
	return out, err
}

func (fetcher *WalletFetcher) insertTransactionsData(
	addresses []*dbsqlc.Address,
	transactions []*OnchainTransaction,
) ([]*dbsqlc.InsertSignaturesRow, error) {
	utils.Assert(len(transactions) > 0, "transactions must not be empty")

	signatures := make([]string, len(transactions))
	for i, tx := range transactions {
		signatures[i] = tx.Signature
	}

	tx, err := fetcher.db.Begin(fetcher.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	qtx := fetcher.q.WithTx(tx)
	insertedSignatures, err := qtx.InsertSignatures(fetcher.ctx, signatures)
	// if data is not yet inserted for main wallet,
	// associated accounts may have only the same txs
	// in that case we would get no rows because conflict
	if errors.Is(err, pgx.ErrNoRows) {
		return make([]*dbsqlc.InsertSignaturesRow, 0), nil
	}
	if err != nil {
		return nil, err
	}

	insertTransactionsParams := make([]*dbsqlc.InsertTransactionsParams, 0)
	insertInstructionsParams := make([]*dbsqlc.InsertInstructionsParams, 0)
	insertInnerInstructionsParams := make([]*dbsqlc.InsertInnerInstructionsParams, 0)
	insertInstructionEventsParams := make([]*dbsqlc.InsertInstructionEventsParams, 0)

	for _, tx := range transactions {
		sigIdx := slices.IndexFunc(insertedSignatures, func(s *dbsqlc.InsertSignaturesRow) bool {
			return s.Value == tx.Signature
		})
		// signature can be missing because of pk conflict
		// line 380
		if sigIdx == -1 {
			continue
		}
		signatureId := insertedSignatures[sigIdx].ID
		insertTransactionsParams = append(insertTransactionsParams, tx.intoParams(signatureId, addresses))

		for ixIndex, ix := range tx.Ixs {
			programAddressId, accountsIds, data := ix.intoParams(addresses)
			insertInstructionsParams = append(insertInstructionsParams, &dbsqlc.InsertInstructionsParams{
				SignatureID:      signatureId,
				Index:            int16(ixIndex),
				ProgramAccountID: programAddressId,
				AccountsIds:      accountsIds,
				Data:             data,
			})

			for innerIxIndex, innerIx := range ix.innerIxs {
				programAddressId, accountsIds, data = innerIx.intoParams(addresses)
				insertInnerInstructionsParams = append(insertInnerInstructionsParams, &dbsqlc.InsertInnerInstructionsParams{
					SignatureID:      signatureId,
					Index:            int16(innerIxIndex),
					IxIndex:          int16(ixIndex),
					ProgramAccountID: programAddressId,
					AccountsIds:      accountsIds,
					Data:             data,
				})
			}

			for eventIndex, event := range ix.events {
				compiledEvent, err := event.Compile(addresses)
				if err != nil {
					return nil, err
				}
				eventType := compiledEvent.Type()
				serializedEvent, err := json.Marshal(compiledEvent)
				if err != nil {
					return nil, err
				}
				insertInstructionEventsParams = append(insertInstructionEventsParams, &dbsqlc.InsertInstructionEventsParams{
					SignatureID: signatureId,
					IxIndex:     int16(ixIndex),
					Index:       int16(eventIndex),
					Type:        eventType,
					Data:        serializedEvent,
				})
			}
		}
	}

	_, err = qtx.InsertTransactions(fetcher.ctx, insertTransactionsParams)
	if err != nil {
		return nil, fmt.Errorf("unable to insert transactions: %w", err)
	}
	_, err = qtx.InsertInstructions(fetcher.ctx, insertInstructionsParams)
	if err != nil {
		return nil, fmt.Errorf("unable to insert instructions: %w", err)
	}
	_, err = qtx.InsertInnerInstructions(fetcher.ctx, insertInnerInstructionsParams)
	if err != nil {
		return nil, fmt.Errorf("unable to insert innner instructions: %w", err)
	}
	_, err = qtx.InsertInstructionEvents(fetcher.ctx, insertInstructionEventsParams)
	if err != nil {
		return nil, fmt.Errorf("unable to insert instructions events: %w", err)
	}

	err = tx.Commit(fetcher.ctx)
	return insertedSignatures, err
}

func (fetcher *WalletFetcher) insertWalletData(
	associatedAccountAddress string,
	associatedAccountsBatch map[string]*ixparser.AssociatedAccount,
	insertedAddresses []*dbsqlc.Address,
	insertedSignatures []*dbsqlc.InsertSignaturesRow,
	savedTransactions []*savedTransaction,
	lastSignature string,
) error {
	signaturesCount := len(savedTransactions) + len(insertedSignatures)
	associatedAccountsCount := len(associatedAccountsBatch)

	utils.Assert(
		lastSignature != "" || signaturesCount > 0 || associatedAccountsCount > 0,
		"invalid wallet data",
	)
	utils.Assert(
		(associatedAccountAddress != "" && associatedAccountsCount == 0) || associatedAccountAddress == "",
		"associated account must not have associated accounts",
	)

	tx, err := fetcher.db.Begin(fetcher.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	qtx := fetcher.q.WithTx(tx)
	if signaturesCount > 0 {
		params := &dbsqlc.AssignInstructionsToWalletParams{}
		for _, tx := range savedTransactions {
			params.WalletID = append(params.WalletID, fetcher.WalletId)
			params.SignatureID = append(params.SignatureID, tx.SignatureID)
		}
		for _, signature := range insertedSignatures {
			params.WalletID = append(params.WalletID, fetcher.WalletId)
			params.SignatureID = append(params.SignatureID, signature.ID)
		}
		_, err = qtx.AssignInstructionsToWallet(fetcher.ctx, params)
		if err != nil {
			return err
		}
	}

	if associatedAccountsCount > 0 {
		params := make([]*dbsqlc.InsertAssociatedAccountsParams, 0)
		for address, associatedAccount := range associatedAccountsBatch {
			idx := slices.IndexFunc(insertedAddresses, func(a *dbsqlc.Address) bool {
				return a.Value == address
			})
			utils.Assert(idx > -1, "unable to find address")
			params = append(params, &dbsqlc.InsertAssociatedAccountsParams{
				WalletID:  fetcher.WalletId,
				AddressID: insertedAddresses[idx].ID,
				Type:      associatedAccount.Type,
			})
		}

		_, err := qtx.InsertAssociatedAccounts(fetcher.ctx, params)
		if err != nil {
			return err
		}
	}

	if associatedAccountAddress != "" && lastSignature != "" {
		if err = qtx.UpdateAssociatedAccountLastSignature(fetcher.ctx, &dbsqlc.UpdateAssociatedAccountLastSignatureParams{
			LastSignature:            lastSignature,
			AssociatedAccountAddress: associatedAccountAddress,
			WalletID:                 fetcher.WalletId,
		}); err != nil {
			return err
		}
		err = qtx.UpdateWalletAggregateCounts(fetcher.ctx, &dbsqlc.UpdateWalletAggregateCountsParams{
			SignaturesCount: int32(signaturesCount),
			WalletID:        fetcher.WalletId,
		})
	} else if lastSignature != "" {
		err = qtx.UpdateWalletAggregateCountsAndLastSignature(fetcher.ctx, &dbsqlc.UpdateWalletAggregateCountsAndLastSignatureParams{
			SignaturesCount:         int32(signaturesCount),
			AssociatedAccountsCount: int32(associatedAccountsCount),
			WalletID:                fetcher.WalletId,
			LastSignature:           lastSignature,
		})
	} else {
		err = qtx.UpdateWalletAggregateCounts(fetcher.ctx, &dbsqlc.UpdateWalletAggregateCountsParams{
			SignaturesCount:         int32(signaturesCount),
			AssociatedAccountsCount: int32(associatedAccountsCount),
			WalletID:                fetcher.WalletId,
		})
	}
	if err != nil {
		return err
	}

	return tx.Commit(fetcher.ctx)
}

func (fetcher *WalletFetcher) HandleMessages(wg *sync.WaitGroup, msgChan chan *Message) {
	defer wg.Done()
	for {
		select {
		case <-fetcher.ctx.Done():
			return
		case msg, ok := <-msgChan:
			if !ok {
				slog.Debug("msg channel closed")
				return
			}
			utils.Assert(
				!(msg.associatedAccountAddress != "" && len(msg.associatedAccounts) > 0),
				"associated account must not have associated accounts",
			)
			utils.Assert(
				len(msg.savedTransactions) > 0 || len(msg.associatedAccounts) > 0 || len(msg.unsavedTransactions) > 0 || msg.lastSignature != "",
				"msg must not be empty",
			)
			slog.Debug("received valid msg", "message", msg, "last sinature", msg.lastSignature)

			uniqueAddresses := make(map[string]bool)
			for _, tx := range msg.unsavedTransactions {
				for _, account := range tx.Accounts {
					uniqueAddresses[account] = true
				}
			}
			for address := range msg.associatedAccounts {
				uniqueAddresses[address] = true
			}

			slog.Debug("inserting addresses")
			insertedAddresses, err := fetcher.insertAddresses(slices.Collect(maps.Keys(uniqueAddresses)))
			utils.AssertNoErr(err)
			if fetcher.ctx.Err() != nil {
				return
			}

			insertedSignatures := make([]*dbsqlc.InsertSignaturesRow, 0)
			if len(msg.unsavedTransactions) > 0 {
				slog.Debug("inserting transactions data")
				insertedSignatures, err = fetcher.insertTransactionsData(insertedAddresses, msg.unsavedTransactions)
				utils.AssertNoErr(err)
				if fetcher.ctx.Err() != nil {
					return
				}
			}

			err = fetcher.insertWalletData(
				msg.associatedAccountAddress,
				msg.associatedAccounts,
				insertedAddresses,
				insertedSignatures,
				msg.savedTransactions,
				msg.lastSignature,
			)
			utils.AssertNoErr(err)
		}
	}
}

func (fetcher *WalletFetcher) HandleTimestampsDuplicates() error {
	startId := int32(0)
	type transaction struct {
		signature string
		id        int32
	}

	for {
		txs, err := fetcher.q.GetTransactionsWithDuplicateTimestamps(fetcher.ctx, startId)
		if err != nil {
			return err
		}
		if len(txs) == 0 {
			return nil
		}

		slots := make(map[int64][]transaction)
		for _, tx := range txs {
			if slots[tx.Slot] == nil {
				slots[tx.Slot] = []transaction{{signature: tx.Signature, id: tx.ID}}
			} else {
				slots[tx.Slot] = append(slots[tx.Slot], transaction{signature: tx.Signature, id: tx.ID})
			}
		}

		updateParams := &dbsqlc.UpdateTransactionsBlockIndexesParams{
			TransactionsIds: make([]int32, 0),
			BlockIndexes:    make([]int32, 0),
		}
		for slot, transactions := range slots {
			blockResult, err := utils.CallRpcWithRetries(func() (*rpc.GetBlockResult, error) {
				return fetcher.rpcClient.GetBlock(fetcher.ctx, uint64(slot))
			}, 5)
			if err != nil {
				return err
			}
			for _, tx := range transactions {
				blockIndex := slices.IndexFunc(blockResult.Signatures, func(s solana.Signature) bool {
					return s.String() == tx.signature
				})
				if blockIndex == -1 {
					slog.Error(
						"unable to find signature in block",
						"signature",
						tx.signature,
						"blockhash",
						blockResult.Blockhash.String(),
						"signatures",
						blockResult.Signatures,
					)
					return errors.New("unable to find signature in block")
				}
				updateParams.TransactionsIds = append(updateParams.TransactionsIds, tx.id)
				updateParams.BlockIndexes = append(updateParams.BlockIndexes, int32(blockIndex))
			}
		}

		err = fetcher.q.UpdateTransactionsBlockIndexes(fetcher.ctx, updateParams)
		if err != nil {
			return err
		}

		if len(txs) < 500 {
			break
		}
		startId = txs[len(txs)-1].ID
	}

	return nil
}
