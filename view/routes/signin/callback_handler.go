package signin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"tax-bro/pkg/utils"
	"tax-bro/view/middlewares"

	"github.com/labstack/echo/v4"
)

type githubAccessTokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             uint64 `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn uint64 `json:"refresh_token_expires_in"`
	Scope                 string
	TokenType             string `json:"token_type"`
}

func getGithubAccessToken(code string) (githubAccessTokenResponse, error) {
	clientId := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s",
			clientId,
			clientSecret,
			code,
		),
		nil,
	)
	utils.Assert(err == nil, fmt.Sprintf("unable to create request: %s", err))
	req.Header.Add("accept", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return githubAccessTokenResponse{}, errors.New("unable to fetch github access token")
	}
	defer res.Body.Close()

	responseData := githubAccessTokenResponse{}
	if err = json.NewDecoder(res.Body).Decode(&responseData); err != nil {
		fmt.Print(err)
		return githubAccessTokenResponse{}, errors.New("unable to deserialize github access token response")
	}

	return responseData, nil
}

type githubUserDataResponse struct {
	Login     string
	Id        uint64
	AvatarUrl string `json:"avatar_url"`
	Email     string
}

func getGithubUserData(accessToken string) (githubUserDataResponse, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	utils.Assert(err == nil, fmt.Sprintf("unable to create request: %s", err))
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("accept", "application/vnd.github+json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return githubUserDataResponse{}, errors.New("unable to fetch github user")
	}
	defer res.Body.Close()

	userData := githubUserDataResponse{}
	if err = json.NewDecoder(res.Body).Decode(&userData); err != nil {
		fmt.Print(err)
		return githubUserDataResponse{}, errors.New("unable to deserialize github user data")
	}

	return userData, nil
}

func CallbackHandler(c echo.Context) error {
	isSignedIn, err := checkIsSignedIn(c)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	if isSignedIn {
		return c.Redirect(http.StatusPermanentRedirect, "/dashboard/wallets")
	}

	provider := c.Param("provider")

	switch provider {
	case "github":
		code := c.QueryParam("code")
		if code == "" {
			// redirect to sign in with unable to sign in message
			return c.NoContent(http.StatusInternalServerError)
		}

		accessTokenData, err := getGithubAccessToken(code)
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}

		userData, err := getGithubUserData(accessTokenData.AccessToken)
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}

		// does user exist
		db := c.(*middlewares.DbContext).Db

		savedUser := struct{ Id int32 }{}
		err = db.Get(&savedUser, "SELECT user_id as id FROM github_auth_data WHERE github_id = $1", userData.Id)

		if errors.Is(err, sql.ErrNoRows) {
			if err = db.Get(&savedUser, "INSERT INTO \"user\" (auth_provider) VALUES ('github') RETURNING id"); err != nil {
				return c.NoContent(http.StatusInternalServerError)
			}
			if _, err = db.Exec(
				"INSERT INTO github_auth_data (user_id, github_id, username) VALUES ($1, $2, $3)",
				savedUser.Id,
				userData.Id,
				userData.Login,
			); err != nil {
				fmt.Print(err)
				return c.NoContent(http.StatusInternalServerError)
			}
		} else if err != nil {
			fmt.Print(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		session := struct{ Id string }{}
		if err = db.Get(&session, "INSERT INTO session (user_id) VALUES ($1) RETURNING id", savedUser.Id); err != nil {
			fmt.Print(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		cookie := &http.Cookie{
			Name:     "auth",
			Value:    session.Id,
			MaxAge:   86400,
			HttpOnly: true,
			Path:     "/",
		}
		c.SetCookie(cookie)

		return c.Redirect(http.StatusPermanentRedirect, "/dashboard/wallets")
	case "discord":
		return c.HTML(http.StatusOK, provider)
	default:
		return c.NoContent(http.StatusNotFound)
	}
}
