package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	CodexAuthURL     = "https://auth.openai.com/oauth/authorize"
	CodexTokenURL    = "https://auth.openai.com/oauth/token"
	CodexClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	CodexRedirectURI = "{baseURL}/api/oauth/cx/callback"
	CodexScope       = "openid profile email offline_access"
)

// CodexTokenResponse is the response from OpenAI's token endpoint.
type CodexTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// BuildCodexAuthURL constructs the OpenAI/Codex OAuth authorization URL with PKCE.
func BuildCodexAuthURL(baseURL, state, codeChallenge string) string {
	redirectURI := strings.Replace(CodexRedirectURI, "{baseURL}", baseURL, 1)
	params := url.Values{
		"client_id":                  {CodexClientID},
		"response_type":              {"code"},
		"redirect_uri":               {redirectURI},
		"scope":                      {CodexScope},
		"code_challenge":             {codeChallenge},
		"code_challenge_method":      {"S256"},
		"state":                      {state},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"originator":                 {"codex_cli_rs"},
	}
	return CodexAuthURL + "?" + params.Encode()
}

// ExchangeCodexCode exchanges an authorization code for tokens.
func ExchangeCodexCode(code, verifier, redirectURI string) (*CodexTokenResponse, error) {
	params := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {CodexClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	resp, err := http.PostForm(CodexTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("codex token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("codex token exchange failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens CodexTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode codex token response: %w", err)
	}
	return &tokens, nil
}

// RefreshCodexToken obtains new tokens using a refresh token.
func RefreshCodexToken(refreshToken string) (*CodexTokenResponse, error) {
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {CodexClientID},
		"scope":         {CodexScope},
	}

	resp, err := http.PostForm(CodexTokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("codex refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("codex token refresh failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens CodexTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode codex refresh response: %w", err)
	}
	return &tokens, nil
}
