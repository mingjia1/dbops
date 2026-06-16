package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OAuth2Config struct {
	Provider     string `json:"provider"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	UserInfoURL  string `json:"user_info_url"`
	RedirectURL  string `json:"redirect_url"`
	Scopes       string `json:"scopes"`
}

type OAuth2Authenticator struct {
	config OAuth2Config
	client *http.Client
}

func NewOAuth2Authenticator(config OAuth2Config) *OAuth2Authenticator {
	if config.Scopes == "" {
		config.Scopes = "openid,profile,email"
	}
	return &OAuth2Authenticator{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *OAuth2Authenticator) AuthURL(state string) string {
	return fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		a.config.AuthURL, a.config.ClientID, a.config.RedirectURL, a.config.Scopes, state)
}

func (a *OAuth2Authenticator) Exchange(ctx context.Context, code string) (string, error) {
	body := fmt.Sprintf("code=%s&client_id=%s&client_secret=%s&redirect_uri=%s&grant_type=authorization_code",
		code, a.config.ClientID, a.config.ClientSecret, a.config.RedirectURL)
	req, err := http.NewRequestWithContext(ctx, "POST", a.config.TokenURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("oauth2 token request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2 token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		IDToken     string `json:"id_token"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("oauth2 token parse failed: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("oauth2 token response missing access_token")
	}
	return result.AccessToken, nil
}

func (a *OAuth2Authenticator) GetUserInfo(ctx context.Context, accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.config.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth2 userinfo request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth2 userinfo failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var info map[string]interface{}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("oauth2 userinfo parse failed: %w", err)
	}
	return info, nil
}
