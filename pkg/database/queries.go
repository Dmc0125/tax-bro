package database

import (
	"fmt"
	"tax-bro/pkg/utils"
)

const QueryInsertAccount = "INSERT INTO \"account\" (selected_auth_provider, email) VALUES ($1, $2)"
const QueryInsertAddress = "INSERT INTO address (value) VALUES ($1)"
const QueryInsertWallet = "INSERT INTO wallet (account_id, address_id, label) VALUES ($1, $2, $3)"
const QueryInsertSyncWalletRequest = "INSERT INTO sync_wallet_request (wallet_id) VALUES ($1)"
const QueryDeleteWallet = "DELETE FROM wallet WHERE id = $1"

func QueryWithReturn(query, returning string) string {
	utils.Assert(query != "", "query can not be empty")
	utils.Assert(returning != "", "returning can not be empty")
	return fmt.Sprintf("%s RETURNING %s", query, returning)
}
