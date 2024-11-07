package wallets

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"tax-bro/pkg/database"
	"tax-bro/pkg/logger"
	"tax-bro/view/auth"
	"tax-bro/view/components"
	"tax-bro/view/constants"
	"tax-bro/view/utils"

	"github.com/a-h/templ"
	"github.com/gagliardetto/solana-go"
	"github.com/labstack/echo/v4"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const selectWalletsQuery = `
	SELECT
		wallet.id, wallet.label, address.value AS address, sync_wallet_request.status
	FROM
		wallet
	INNER JOIN
		address ON address.id = wallet.address_id
	LEFT JOIN
		sync_wallet_request ON sync_wallet_request.wallet_id = wallet.id
	WHERE
		wallet.account_id = $1
	ORDER BY
		wallet.created_at DESC
`

type wallet struct {
	Id      int32
	Label   string
	Address string
	Status  string
}

func (w *wallet) component(includeForm bool) templ.Component {
	status := strings.ReplaceAll(cases.Title(language.English).String(w.Status), "_", " ")
	if w.Status == "" {
		status = "Synced"
	}
	return walletComponent(int(w.Id), w.Label, w.Address, status, includeForm)
}

func GET(c echo.Context) error {
	ac := c.(*auth.AuthenticatedContext)

	wallets := []wallet{}
	if err := ac.Db.Select(&wallets, selectWalletsQuery, ac.AccountId); err != nil {
		logger.Log(err)
	}

	walletsComponents := []templ.Component{}
	for _, w := range wallets {
		walletsComponents = append(walletsComponents, w.component(false))
	}

	return utils.RenderOk(c, WalletsView(walletsComponents))
}

func POST(c echo.Context) error {
	req := c.Request()

	err := req.ParseForm()
	if err != nil {
		return components.GlobalErrorResponse(c, constants.ClientErrUnprocessableContent, http.StatusUnprocessableEntity)
	}

	address := req.PostFormValue("wallet_address")
	_, err = solana.PublicKeyFromBase58(address)
	if err != nil {
		logger.Log(err)
		c.Response().Header().Add("hx-retarget", "#wallet_address_err")
		c.Response().Header().Add("hx-reswap", "innerHTML")
		return utils.RenderWithStatus(c, inputErrorComponent("Invalid public key"), http.StatusUnprocessableEntity)
	}

	label := req.PostFormValue("wallet_label")
	if label == "" {
		label = fmt.Sprintf("Wallet %s", address[len(address)-4:])
	}

	ac := c.(*auth.AuthenticatedContext)
	db := ac.Db

	tx, err := db.Beginx()
	if err != nil {
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}
	defer tx.Rollback()

	savedAddress := struct{ Id int32 }{}
	err = tx.Get(&savedAddress, "SELECT id FROM address WHERE value = $1", address)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.Get(&savedAddress, database.QueryWithReturn(database.QueryInsertAddress, "id"), address)
	}
	if err != nil {
		logger.Log(err)
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}

	insertedWallet := struct{ Id int32 }{}
	err = tx.Get(
		&insertedWallet,
		database.QueryWithReturn(database.QueryInsertWallet, "id"),
		ac.AccountId,
		savedAddress.Id,
		label,
	)
	if err != nil && strings.HasPrefix(err.Error(), constants.PostgresDuplicateUnique) {
		c.Response().Header().Add("hx-retarget", "#wallet_address_err")
		c.Response().Header().Add("hx-reswap", "innerHTML")
		return utils.RenderWithStatus(c, inputErrorComponent("Wallet with this address already exists"), http.StatusConflict)
	}
	if err != nil {
		logger.Log(err)
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}

	if _, err = tx.Exec(database.QueryInsertSyncWalletRequest, insertedWallet.Id); err != nil {
		logger.Log(err)
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}

	if err = tx.Commit(); err != nil {
		logger.Log(err)
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}

	// TODO: if its first wallet, need to return #wallets
	return utils.RenderOk(c, walletComponent(int(insertedWallet.Id), label, address, "Queued", true))
}

func DELETE(c echo.Context) error {
	walletId, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		return components.GlobalErrorResponse(c, "Invalid wallet id (wallet id needs to be 32 bit int and > 0)", http.StatusUnprocessableEntity)
	}
	if walletId == 0 {
		return components.GlobalErrorResponse(c, "Invalid wallet id (wallet id needs to be > 0)", http.StatusUnprocessableEntity)
	}

	ac := c.(*auth.AuthenticatedContext)
	_, err = ac.Db.Exec(database.QueryDeleteWallet, int32(walletId))
	if errors.Is(err, sql.ErrNoRows) {
		return components.GlobalErrorResponse(c, "Wallet does not exist", http.StatusNotFound)
	}
	if err != nil {
		logger.Log(err)
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}

	remainingWallets := []wallet{}
	if err = ac.Db.Select(&remainingWallets, selectWalletsQuery, ac.AccountId); err != nil {
		logger.Log(err)
		return components.GlobalErrorResponse(c, constants.ClientErrInternal, http.StatusInternalServerError)
	}

	walletsComponents := []templ.Component{}
	for _, w := range remainingWallets {
		walletsComponents = append(walletsComponents, w.component(false))
	}

	return utils.RenderOk(c, walletsListComponent(walletsComponents))
}
