package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"tax-bro/pkg/logger"
	"tax-bro/pkg/utils"
	"tax-bro/view/constants"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

const (
	githubApiUrl  = "https://api.github.com"
	discordApiUrl = "https://discord.com/api"
)

var oauthConfigs = map[string]*oauth2.Config{
	"github": {
		ClientID:     utils.GetEnvVar("GITHUB_CLIENT_ID"),
		ClientSecret: utils.GetEnvVar("GITHUB_CLIENT_SECRET"),
		Endpoint:     github.Endpoint,
		Scopes:       []string{"user:email"},
		RedirectURL:  fmt.Sprintf("%s/signin/github/callback", constants.BASE_URL),
	},
	"discord": {
		ClientID:     utils.GetEnvVar("DISCORD_CLIENT_ID"),
		ClientSecret: utils.GetEnvVar("DISCORD_CLIENT_SECRET"),
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://discord.com/oauth2/authorize",
			TokenURL:  "https://discord.com/api/oauth2/token",
			AuthStyle: oauth2.AuthStyleInHeader,
		},
		Scopes:      []string{"identify email"},
		RedirectURL: fmt.Sprintf("%s/signin/discord/callback", constants.BASE_URL),
	},
	"google": {
		ClientID:     utils.GetEnvVar("GOOGLE_CLIENT_ID"),
		ClientSecret: utils.GetEnvVar("GOOGLE_CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{"openid profile email"},
		RedirectURL:  fmt.Sprintf("%s/signin/google/callback", constants.BASE_URL),
	},
}

type githubUserDataResponse struct {
	Login     string
	Id        uint64
	AvatarUrl string `json:"avatar_url"`
	Email     string
}

type githubEmailsResponse []struct {
	Email      string
	Primary    bool
	Verified   bool
	Visibility string
}

type githubOauthData struct {
	m        *sync.Mutex
	userData githubUserDataResponse
	emails   githubEmailsResponse
}

func (data *githubOauthData) headers(accessToken string) map[string]string {
	return map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", accessToken),
		"accept":        "application/vnd.github+json",
	}
}

func (data *githubOauthData) getUserData(accessToken string) error {
	userInfoRes, err := fetch("GET", fmt.Sprintf("%s/user", githubApiUrl), nil, data.headers(accessToken))
	if err != nil {
		logger.Logf("unable to fetch github user info: %s", err)
		return err
	}
	data.m.Lock()
	defer data.m.Unlock()
	if err = json.Unmarshal(userInfoRes, &data.userData); err != nil {
		logger.Logf("unable to deserialize github user info: %s", err)
		return err
	}
	return nil
}

func (data *githubOauthData) getUserEmails(accessToken string) error {
	userEmailsRes, err := fetch("GET", fmt.Sprintf("%s/user/emails", githubApiUrl), nil, data.headers(accessToken))
	if err != nil {
		logger.Logf("unable to fetch github user emails: %s", err)
		return err
	}
	data.m.Lock()
	defer data.m.Unlock()
	if err = json.Unmarshal(userEmailsRes, &data.emails); err != nil {
		logger.Logf("unable to deserialize github user emails: %s", err)
		return err
	}
	return nil
}

type googleUserInfo struct {
	Sub           string
	Email         string
	EmailVerified bool `json:"email_verified"`
	Picture       string
}

func getGoogleUserData(accessToken string) (googleUserInfo, error) {
	userInfo := googleUserInfo{}
	headers := map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", accessToken),
		"accept":        "application/json",
	}

	openIdRes, err := fetch("GET", "https://accounts.google.com/.well-known/openid-configuration", nil, headers)
	if err != nil {
		logger.Logf("unable to fetch google openid config: %s", err)
		return userInfo, err
	}
	openidConfig := struct {
		UserInfoEndpoint string `json:"userinfo_endpoint"`
	}{}
	if err := json.Unmarshal(openIdRes, &openidConfig); err != nil {
		logger.Logf("unable to deserialize google openid config: %s", err)
		return userInfo, err
	}

	userInfoRes, err := fetch("GET", openidConfig.UserInfoEndpoint, nil, headers)
	if err != nil {
		logger.Logf("unable to fetch google userinfo: %s", err)
		return userInfo, err
	}
	if err = json.Unmarshal(userInfoRes, &userInfo); err != nil {
		logger.Logf("unable to deserialize google userinfo: %s", err)
	}

	return userInfo, err
}

type discordUserDataResponse struct {
	Id       string
	Username string
	Verified bool
	Email    string
}

func getDiscordUserData(accessToken string) (discordUserDataResponse, error) {
	userDataRes, err := fetch("GET", fmt.Sprintf("%s/users/@me", discordApiUrl), nil, map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", accessToken),
		"accept":        "application/json",
	})
	if err != nil {
		logger.Logf("unable to fetch discord user data: %s", err)
		return discordUserDataResponse{}, err
	}

	userData := discordUserDataResponse{}
	if err = json.Unmarshal(userDataRes, &userData); err != nil {
		logger.Logf("unable to fetch deserialize user data: %s", err)
		return userData, err
	}

	return userData, nil
}

func fetch(method, url string, body io.Reader, headers map[string]string) (data []byte, e error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		e = fmt.Errorf("unable to create request: %w", err)
		return
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		e = fmt.Errorf("unable to fetch request: %w", err)
		return
	}
	defer res.Body.Close()
	data, err = io.ReadAll(res.Body)
	if err != nil {
		e = fmt.Errorf("unable to read body: %w", err)
		return
	}
	if res.StatusCode != 200 {
		e = fmt.Errorf("response not ok - status: %d body: %s", res.StatusCode, string(data))
	}
	return
}
