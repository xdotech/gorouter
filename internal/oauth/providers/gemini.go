package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	GeminiAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GeminiTokenURL    = "https://oauth2.googleapis.com/token"
	GeminiClientID    = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	GeminiClientSecret = "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"
	GeminiRedirectURI = "{baseURL}/api/oauth/gc/callback"
	GeminiScopes      = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"
)

// GeminiTokenResponse is the response from Google's token endpoint.
type GeminiTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// BuildGeminiAuthURL constructs the Gemini/Google OAuth authorization URL.
// Gemini does not use PKCE — code_challenge params are omitted.
func BuildGeminiAuthURL(baseURL, state, _ string) string {
	redirectURI := strings.Replace(GeminiRedirectURI, "{baseURL}", baseURL, 1)
	params := url.Values{
		"client_id":     {GeminiClientID},
		"response_type": {"code"},
		"redirect_uri":  {redirectURI},
		"scope":         {GeminiScopes},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	return GeminiAuthURL + "?" + params.Encode()
}

// ExchangeGeminiCode exchanges an authorization code for tokens.
func ExchangeGeminiCode(code, _, redirectURI string) (*GeminiTokenResponse, error) {
	params := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {GeminiClientID},
		"client_secret": {GeminiClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	resp, err := http.PostForm(GeminiTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("gemini token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("gemini token exchange failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens GeminiTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode gemini token response: %w", err)
	}
	return &tokens, nil
}

// RefreshGeminiToken obtains new tokens using a refresh token.
func RefreshGeminiToken(refreshToken string) (*GeminiTokenResponse, error) {
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {GeminiClientID},
		"client_secret": {GeminiClientSecret},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(GeminiTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("gemini refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("gemini token refresh failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens GeminiTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode gemini refresh response: %w", err)
	}
	return &tokens, nil
}
