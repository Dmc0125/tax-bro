package walletfetcher

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"tax-bro/pkg/dbsqlc"
	"tax-bro/pkg/ixparser"
	"tax-bro/pkg/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

type savedInstructionBase struct {
	ProgramAddress string `json:"program_address"`
	Accounts       []*dbsqlc.Address
	Data           string
}

func (ix *savedInstructionBase) GetProgramAddress() string {
	return ix.ProgramAddress
}

func (ix *savedInstructionBase) GetAccounts() (accounts []string) {
	for _, a := range ix.Accounts {
		accounts = append(accounts, a.Value)
	}
	return
}

func (ix *savedInstructionBase) GetData() []byte {
	bytes, err := base64.RawStdEncoding.DecodeString(ix.Data)
	utils.AssertNoErr(err)
	return bytes
}

type savedInstruction struct {
	*savedInstructionBase
	InnerIxs []*savedInstructionBase `json:"inner_ixs"`
}

func (ix *savedInstruction) GetInnerInstructions() (parsableInnerIxs []ixparser.ParsableInstructionBase) {
	for _, innerIx := range ix.InnerIxs {
		parsableInnerIxs = append(parsableInnerIxs, innerIx)
	}
	return
}

type savedTransaction struct {
	*dbsqlc.GetTransactionsFromSignaturesRow
	Ixs []*savedInstruction
}

type FetchTransactionsRequest struct {
	associatedAccountAddress string
	pubkey                   solana.PublicKey
	config                   *rpc.GetConfirmedSignaturesForAddress2Opts
}

func NewFetchTransactionsRequest(
	associatedAccountAddress, walletAddress, lastSignature string,
) *FetchTransactionsRequest {
	utils.Assert(
		associatedAccountAddress != "" || walletAddress != "",
		"either wallet address or associated account address needs to be provided",
	)
	address := walletAddress
	if associatedAccountAddress != "" {
		address = associatedAccountAddress
	}
	pk, err := solana.PublicKeyFromBase58(address)
	utils.AssertNoErr(err, "invalid wallet or associated account address")

	limit := uint64(1000)
	config := &rpc.GetConfirmedSignaturesForAddress2Opts{
		Limit:      &limit,
		Commitment: rpc.CommitmentConfirmed,
	}
	if lastSignature != "" {
		until, err := solana.SignatureFromBase58(lastSignature)
		utils.AssertNoErr(err, "invalid last signature")
		config.Until = until
	}

	return &FetchTransactionsRequest{
		config:                   config,
		pubkey:                   pk,
		associatedAccountAddress: associatedAccountAddress,
	}
}

type Message struct {
	associatedAccountAddress string
	lastSignature            string

	savedTransactions   []*savedTransaction
	unsavedTransactions []*OnchainTransaction
}

func NewMessage(associatedAccountAddress string, isLastIter bool, lastSignature string) *Message {
	msg := new(Message)
	msg.associatedAccountAddress = associatedAccountAddress
	if isLastIter {
		msg.lastSignature = lastSignature
	}
	msg.unsavedTransactions = make([]*OnchainTransaction, 0)
	return msg
}

type WalletFetcher struct {
	ctx       context.Context
	db        *pgx.Conn
	q         *dbsqlc.Queries
	rpcClient *rpc.Client

	WalletId      int32
	WalletAddress string
	LastSignature string

	AssociatedAccounts map[string]*ixparser.AssociatedAccount
}

func NewFetcher(
	ctx context.Context,
	db *pgx.Conn,
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
	if errors.Is(err, sql.ErrNoRows) {
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
		return nil, fmt.Errorf("unbale to query transactions from signatures: %w", err)
	}
	savedTransactions := make([]*savedTransaction, len(savedTransactionsRows))
	for i, tx := range savedTransactionsRows {
		ixs := []*savedInstruction{}
		if err = json.Unmarshal(tx.Ixs.([]byte), &ixs); err != nil {
			slog.Error("unmarshal error", "ixs", string(tx.Ixs.([]byte)))
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

	for {
		select {
		case <-fetcher.ctx.Done():
			return
		default:
			signatures, err := utils.CallRpcWithRetries(func() ([]*rpc.TransactionSignature, error) {
				return fetcher.rpcClient.GetConfirmedSignaturesForAddress2(fetcher.ctx, request.pubkey, config)
			}, 5)
			utils.AssertNoErr(err, "unable to fetch signatures")

			isLastIter := len(signatures) < int(*config.Limit)
			msg := NewMessage(request.associatedAccountAddress, isLastIter, newLastSignature)

			if len(signatures) == 0 {
				msgChan <- msg
				return
			}
			if newLastSignature == "" {
				newLastSignature = signatures[0].Signature.String()
			}

			msg.savedTransactions, err = fetcher.querySavedTransactions(signatures)
			utils.AssertNoErr(err)

			for _, signature := range signatures {
				if fetcher.ctx.Err() != nil {
					return
				}
				s := signature.Signature
				txIdx := slices.IndexFunc(msg.savedTransactions, func(tx *savedTransaction) bool {
					return tx.Signature == s.String()
				})
				if txIdx == -1 {
					continue
				}
				tx, err := fetcher.fetchTransaction(s)
				utils.AssertNoErr(err, "unable to fetch transaction")
				msg.unsavedTransactions = append(msg.unsavedTransactions, tx)
			}

			msgChan <- msg
			if isLastIter {
				return
			}
		}
	}
}

func (fetcher *WalletFetcher) insertAddresses(addresses []string) ([]*dbsqlc.Address, error) {
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

	out := make([]*dbsqlc.Address, len(addresses), len(addresses))
	for i, a := range savedAddresses {
		out[i] = &dbsqlc.Address{
			Value: a.Value,
			ID:    a.ID,
		}
	}
	if len(savedAddresses) == len(addresses) {
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
	offset := len(savedAddresses)
	for i, a := range insertedAddresses {
		out[i+offset] = &dbsqlc.Address{
			Value: a.Value,
			ID:    a.ID,
		}
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
	if err != nil {
		return nil, err
	}

	insertTransactionsParams := make([]*dbsqlc.InsertTransactionsParams, len(transactions))
	insertInstructionsParams := make([]*dbsqlc.InsertInstructionsParams, 0)
	insertInnerInstructionsParams := make([]*dbsqlc.InsertInnerInstructionsParams, 0)
	insertInstructionEventsParams := make([]*dbsqlc.InsertInstructionEventsParams, 0)

	for i, tx := range transactions {
		sigIdx := slices.IndexFunc(insertedSignatures, func(s *dbsqlc.InsertSignaturesRow) bool {
			return s.Value == tx.Signature
		})
		signatureId := insertedSignatures[sigIdx].ID
		insertTransactionsParams[i] = tx.intoParams(signatureId, addresses)

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

	var eg errgroup.Group
	eg.TryGo(func() error {
		_, err = qtx.InsertTransactions(fetcher.ctx, insertTransactionsParams)
		return err
	})
	if len(insertInstructionsParams) > 0 {
		eg.TryGo(func() error {
			_, err := qtx.InsertInstructions(fetcher.ctx, insertInstructionsParams)

			if len(insertInnerInstructionsParams) > 0 {
				eg.TryGo(func() error {
					_, err := qtx.InsertInnerInstructions(fetcher.ctx, insertInnerInstructionsParams)
					return err
				})
			}
			if len(insertInstructionEventsParams) > 0 {
				eg.TryGo(func() error {
					_, err := qtx.InsertInstructionEvents(fetcher.ctx, insertInstructionEventsParams)
					return err
				})
			}

			return err
		})
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
		assignSignaturesToWalletParams := make([]*dbsqlc.AssignInstructionsToWalletParams, signaturesCount, signaturesCount)
		for i, tx := range savedTransactions {
			assignSignaturesToWalletParams[i] = &dbsqlc.AssignInstructionsToWalletParams{
				WalletID:    fetcher.WalletId,
				SignatureID: tx.SignatureID,
			}
		}
		offset := len(savedTransactions)
		for i, signature := range insertedSignatures {
			assignSignaturesToWalletParams[i+offset] = &dbsqlc.AssignInstructionsToWalletParams{
				WalletID:    fetcher.WalletId,
				SignatureID: signature.ID,
			}
		}
		_, err = qtx.AssignInstructionsToWallet(fetcher.ctx, assignSignaturesToWalletParams)
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
		err = qtx.UpdateAssociatedAccountLastSignature(fetcher.ctx, &dbsqlc.UpdateAssociatedAccountLastSignatureParams{
			LastSignature:            lastSignature,
			AssociatedAccountAddress: associatedAccountAddress,
			WalletID:                 fetcher.WalletId,
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

func (fetcher *WalletFetcher) HandleMessages(msgChan chan *Message) {
	for {
		select {
		case <-fetcher.ctx.Done():
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			utils.Assert(
				len(msg.savedTransactions) > 0 || len(msg.unsavedTransactions) > 0 || msg.lastSignature != "",
				"invalid message data",
			)
			// msg can be sent with
			//
			// txs associated to wallet imported by user
			// txs associated to associated account related to wallet
			//
			// saved txs already have events parsed, so for those we only want to
			// parse associated accounts (so if it's sent by associated account we ignore saved txs)
			//
			// for unsaved txs associated to wallet we need to parse both events and associated accounts
			// otherwise only parse events

			associatedAccountsBatch := make(map[string]*ixparser.AssociatedAccount)
			if msg.associatedAccountAddress == "" {
				for _, tx := range msg.savedTransactions {
					if tx.Err {
						continue
					}
					for _, ix := range tx.Ixs {
						_, associatedAccounts := ixparser.ParseInstruction(ix, fetcher.WalletAddress, tx.Signature)
						if associatedAccounts == nil {
							continue
						}
						maps.Insert(associatedAccountsBatch, maps.All(associatedAccounts))
						maps.Insert(fetcher.AssociatedAccounts, maps.All(associatedAccounts))
					}
				}
			}
			addressesBatch := make(map[string]bool)
			for _, tx := range msg.unsavedTransactions {
				if tx.Err {
					continue
				}
				for _, address := range tx.Accounts {
					addressesBatch[address] = true
				}
				for _, ix := range tx.Ixs {
					events, associatedAccounts := ixparser.ParseInstruction(
						ix,
						// don't care if it's asscoiated account because
						// those can not have associated accounts
						fetcher.WalletAddress,
						tx.Signature,
					)
					if events != nil {
						ix.events = append(ix.events, events...)
					}
					if msg.associatedAccountAddress == "" && associatedAccounts != nil {
						maps.Insert(associatedAccountsBatch, maps.All(associatedAccounts))
						maps.Insert(fetcher.AssociatedAccounts, maps.All(associatedAccounts))
					}
				}
			}

			// Need to handle it with txs because
			// If it fails for whatever reason, we want to
			// 		- keep currently inserted accounts (actually does not matter but why not keep it)
			// 		- signatures are expected to have tx, ixs, innerixs and events
			// 		- wallet state is independent of all of these so does not matter if we don't get to it (will just be handled next time)
			insertedAddresses, err := fetcher.insertAddresses(slices.Collect(maps.Keys(addressesBatch)))
			utils.AssertNoErr(err)
			if fetcher.ctx.Err() != nil {
				return
			}

			insertedSignatures := make([]*dbsqlc.InsertSignaturesRow, 0)
			if len(msg.unsavedTransactions) > 0 {
				insertedSignatures, err = fetcher.insertTransactionsData(insertedAddresses, msg.unsavedTransactions)
				utils.AssertNoErr(err)
				if fetcher.ctx.Err() != nil {
					return
				}
			}

			err = fetcher.insertWalletData(
				msg.associatedAccountAddress,
				associatedAccountsBatch,
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
