package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	ClaudeAuthURL     = "https://claude.ai/oauth/authorize"
	ClaudeTokenURL    = "https://console.anthropic.com/v1/oauth/token"
	ClaudeClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	ClaudeRedirectURI = "{baseURL}/api/oauth/cc/callback"
	ClaudeScopes      = "org:create_api_key user:profile user:inference"
)

// ClaudeTokenResponse is the response from Claude's token endpoint.
type ClaudeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// BuildClaudeAuthURL constructs the Claude OAuth authorization URL.
func BuildClaudeAuthURL(baseURL, state, codeChallenge string) string {
	redirectURI := strings.Replace(ClaudeRedirectURI, "{baseURL}", baseURL, 1)
	params := url.Values{
		"code":                  {"true"},
		"client_id":             {ClaudeClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {ClaudeScopes},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	return ClaudeAuthURL + "?" + params.Encode()
}

// ExchangeClaudeCode exchanges an authorization code for tokens.
// Claude may include a state fragment in the code (code#state).
func ExchangeClaudeCode(code, verifier, redirectURI string) (*ClaudeTokenResponse, error) {
	authCode := code
	codeState := ""
	if idx := strings.Index(code, "#"); idx != -1 {
		authCode = code[:idx]
		codeState = code[idx+1:]
	}

	payload := map[string]string{
		"code":          authCode,
		"state":         codeState,
		"grant_type":    "authorization_code",
		"client_id":     ClaudeClientID,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal claude token request: %w", err)
	}

	resp, err := http.Post(ClaudeTokenURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("claude token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("claude token exchange failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens ClaudeTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode claude token response: %w", err)
	}
	return &tokens, nil
}

// RefreshClaudeToken obtains new tokens using a refresh token.
func RefreshClaudeToken(refreshToken string) (*ClaudeTokenResponse, error) {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     ClaudeClientID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal claude refresh request: %w", err)
	}

	resp, err := http.Post(ClaudeTokenURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("claude refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("claude token refresh failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens ClaudeTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode claude refresh response: %w", err)
	}
	return &tokens, nil
}
