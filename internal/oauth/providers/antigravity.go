package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Antigravity uses Google OAuth with its own client credentials.
	AntigravityAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	AntigravityTokenURL     = "https://oauth2.googleapis.com/token"
	AntigravityClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	AntigravityClientSecret = "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"
	AntigravityRedirectURI  = "{baseURL}/api/oauth/ag/callback"
	AntigravityScopes       = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"
)

// BuildAntigravityAuthURL constructs the Antigravity/Google OAuth authorization URL.
func BuildAntigravityAuthURL(baseURL, state, _ string) string {
	redirectURI := strings.Replace(AntigravityRedirectURI, "{baseURL}", baseURL, 1)
	params := url.Values{
		"client_id":     {AntigravityClientID},
		"response_type": {"code"},
		"redirect_uri":  {redirectURI},
		"scope":         {AntigravityScopes},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	return AntigravityAuthURL + "?" + params.Encode()
}

// ExchangeAntigravityCode exchanges an authorization code for tokens.
func ExchangeAntigravityCode(code, _, redirectURI string) (*GeminiTokenResponse, error) {
	params := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {AntigravityClientID},
		"client_secret": {AntigravityClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	resp, err := http.PostForm(AntigravityTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("antigravity token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("antigravity token exchange failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens GeminiTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode antigravity token response: %w", err)
	}
	return &tokens, nil
}

// RefreshAntigravityToken obtains new tokens using a refresh token.
func RefreshAntigravityToken(refreshToken string) (*GeminiTokenResponse, error) {
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {AntigravityClientID},
		"client_secret": {AntigravityClientSecret},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(AntigravityTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("antigravity refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("antigravity token refresh failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens GeminiTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode antigravity refresh response: %w", err)
	}
	return &tokens, nil
}
