package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"tax-bro/pkg/logger"
	"tax-bro/view/constants"
	"tax-bro/view/middlewares"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/errgroup"
)

const insertAuthQuery = "INSERT INTO auth (account_id, p_type, provider_id, username, avatar_url) VALUES ($1, $2, $3, $4, $5)"

var isSecure = strings.Contains(constants.BASE_URL, "https")

func redirectErr(c echo.Context, msg string) error {
	return c.Redirect(http.StatusPermanentRedirect, fmt.Sprintf("/signin?error=%s", msg))
}

func createAuthCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "auth",
		HttpOnly: true,
		Path:     "/",
		Value:    value,
		MaxAge:   maxAge,
		Secure:   isSecure,
	}
}

type auth struct {
	ProviderType string `db:"p_type"`
	ProviderId   string `db:"provider_id"`
}

func HandleSignInCallback(c echo.Context) error {
	ac, err := authenticate(c)
	if err != nil {
		return redirectErr(c, constants.ClientErrInternal)
	}
	if ac.AccountId != 0 {
		return c.Redirect(http.StatusPermanentRedirect, "/wallets")
	}

	state := c.QueryParam("state")
	if state != constants.OAUTH_STATE {
		return redirectErr(c, constants.ClientErrInternal)
	}

	code := c.QueryParam("code")
	if code == "" {
		return redirectErr(c, constants.ClientErrInternal)
	}

	provider := c.Param("provider")

	oauthConfig := oauthConfigs[provider]
	if oauthConfig == nil {
		return redirectErr(c, constants.ClientErrInvalidProvider)
	}

	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Log(err)
		return redirectErr(c, constants.ClientErrInternal)
	}

	providerData := struct {
		username  string
		email     string
		id        string
		avatarUrl string
	}{}

	switch provider {
	case "github":
		var eg errgroup.Group
		githubOAuthData := githubOauthData{
			m: &sync.Mutex{},
		}

		eg.TryGo(func() error {
			return githubOAuthData.getUserData(token.AccessToken)
		})
		eg.TryGo(func() error {
			return githubOAuthData.getUserEmails(token.AccessToken)
		})

		err = eg.Wait()
		if err != nil {
			return redirectErr(c, constants.ClientErrInternal)
		}

		email := ""
		for i := 0; i < len(githubOAuthData.emails); i++ {
			e := &githubOAuthData.emails[i]
			if e.Primary && e.Verified {
				email = e.Email
				break
			}
		}
		if len(email) == 0 {
			return redirectErr(c, constants.ClientErrEmailNotVerified)
		}

		providerData.email = email
		providerData.avatarUrl = githubOAuthData.userData.AvatarUrl
		providerData.username = githubOAuthData.userData.Login
		providerData.id = fmt.Sprintf("%d", githubOAuthData.userData.Id)
	case "discord":
		userData, err := getDiscordUserData(token.AccessToken)
		if err != nil {
			return redirectErr(c, constants.ClientErrInternal)
		}

		if !userData.Verified {
			return redirectErr(c, constants.ClientErrEmailNotVerified)
		}

		providerData.email = userData.Email
		providerData.id = userData.Id
		providerData.username = userData.Username
		providerData.avatarUrl = fmt.Sprintf("https://api.dicebear.com/9.x/pixel-art/svg?seed=%s", userData.Username)
	case "google":
		userInfo, err := getGoogleUserData(token.AccessToken)
		if err != nil {
			return redirectErr(c, constants.ClientErrInternal)
		}
		if !userInfo.EmailVerified {
			return redirectErr(c, constants.ClientErrEmailNotVerified)
		}

		providerData.username = generateRandomName(time.Now().Unix())
		providerData.email = userInfo.Email
		providerData.id = userInfo.Sub
		providerData.avatarUrl = userInfo.Picture
	default:
		return redirectErr(c, constants.ClientErrInvalidProvider)
	}

	db := c.(*middlewares.DbContext).Db

	// TODO: maybe search based on provider id, if account changes their email on provider
	account := struct {
		Id    int32
		Auths []byte
	}{}
	err = db.Get(
		&account,
		`SELECT
			account.id, (
				SELECT coalesce(json_agg(agg), '[]') FROM (
					SELECT
						auth.p_type,
						auth.provider_id
					FROM
						auth
					WHERE
						auth.account_id = account.id
				) AS agg
			) AS auths
		FROM
			account
		WHERE
			account.email = $1`,
		providerData.email,
	)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	} else if err != nil {
		logger.Log(err)
		return redirectErr(c, constants.ClientErrInternal)
	}

	accountAuths := []auth{}
	if account.Id != 0 {
		if err = json.Unmarshal(account.Auths, &accountAuths); err != nil {
			logger.Log(err)
			return redirectErr(c, constants.ClientErrInternal)
		}
	}

	currentProviderIdx := slices.IndexFunc(accountAuths, func(a auth) bool {
		return a.ProviderId == providerData.id && a.ProviderType == provider
	})

	if len(accountAuths) == 0 {
		// account does not exist
		tx, err := db.Beginx()
		if err != nil {
			logger.Log(err)
			return redirectErr(c, constants.ClientErrInternal)
		}

		newAccount := struct{ Id int32 }{}
		err = tx.Get(&newAccount, "INSERT INTO \"account\" (selected_auth_provider, email) VALUES ($1, $2) RETURNING id", provider, providerData.email)
		if err != nil {
			logger.Log(err)
			tx.Rollback()
			return redirectErr(c, constants.ClientErrInternal)
		}

		account.Id = newAccount.Id
		if _, err = tx.Exec(insertAuthQuery, account.Id, provider, providerData.id, providerData.username, providerData.avatarUrl); err != nil {
			logger.Log(err)
			tx.Rollback()
			return redirectErr(c, constants.ClientErrInternal)
		}

		err = tx.Commit()
	} else if currentProviderIdx == -1 {
		// account exists but with different provider
		_, err = db.Exec(insertAuthQuery, account.Id, provider, providerData.id, providerData.username, providerData.avatarUrl)
	}

	if err != nil {
		logger.Log(err)
		return redirectErr(c, constants.ClientErrInternal)
	}

	maxAge := 86400
	expiresAt := time.Unix(time.Now().Unix()+int64(maxAge), 0)

	session := struct{ Id string }{}
	err = db.Get(&session, "INSERT INTO session (expires_at, account_id) VALUES ($1, $2) RETURNING id", expiresAt, account.Id)

	if err != nil && strings.HasPrefix(err.Error(), "pq: duplicate key value violates unique constraint") {
		err = db.Get(&session, "UPDATE session SET expires_at = $1 WHERE account_id = $2", expiresAt, account.Id)
		if err != nil {
			logger.Log(err)
			return redirectErr(c, constants.ClientErrInternal)
		}
	} else if err != nil {
		logger.Log(err)
		return redirectErr(c, constants.ClientErrInternal)
	}

	c.SetCookie(createAuthCookie(session.Id, maxAge))
	return c.Redirect(http.StatusPermanentRedirect, "/wallets")
}
