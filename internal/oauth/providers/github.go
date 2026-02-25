package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	GitHubAuthURL    = "https://github.com/login/oauth/authorize"
	GitHubTokenURL   = "https://github.com/login/oauth/access_token"
	GitHubDeviceURL  = "https://github.com/login/device/code"
	GitHubClientID   = "Iv1.b507a08c87ecfe98"
	GitHubScopes     = "read:user"
	gitHubAPIVersion = "2022-11-28"
	gitHubUserAgent  = "GitHubCopilotChat/0.26.7"
	copilotTokenURL  = "https://api.github.com/copilot_internal/v2/token"
)

// GitHubTokenResponse is the response from GitHub's token endpoint.
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// GitHubDeviceCodeResponse is the initial device code response.
type GitHubDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// BuildGitHubAuthURL constructs the GitHub OAuth authorization URL.
// GitHub uses standard OAuth app flow (no PKCE).
func BuildGitHubAuthURL(baseURL, state string) string {
	redirectURI := baseURL + "/api/oauth/gh/callback"
	params := url.Values{
		"client_id":    {GitHubClientID},
		"redirect_uri": {redirectURI},
		"scope":        {GitHubScopes},
		"state":        {state},
	}
	return GitHubAuthURL + "?" + params.Encode()
}

// ExchangeGitHubCode exchanges an authorization code for a GitHub access token.
func ExchangeGitHubCode(code, redirectURI string) (*GitHubTokenResponse, error) {
	params := url.Values{
		"client_id":    {GitHubClientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	req, err := http.NewRequest(http.MethodPost, GitHubTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build github token request: %w", err)
	}
	req.URL.RawQuery = params.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", gitHubUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github token request: %w", err)
	}
	defer resp.Body.Close()

	var tokens GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode github token response: %w", err)
	}
	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("github token exchange: no access_token in response")
	}
	return &tokens, nil
}

// FetchCopilotToken exchanges a GitHub access token for a Copilot session token.
// Returns (copilotToken, expiresAt, error).
func FetchCopilotToken(githubAccessToken string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, copilotTokenURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("build copilot token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+githubAccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-GitHub-Api-Version", gitHubAPIVersion)
	req.Header.Set("User-Agent", gitHubUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("copilot token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("copilot token request failed with status %d", resp.StatusCode)
	}

	var data struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("decode copilot token response: %w", err)
	}
	return data.Token, data.ExpiresAt, nil
}

// RequestGitHubDeviceCode requests a device code for device flow auth.
func RequestGitHubDeviceCode() (*GitHubDeviceCodeResponse, error) {
	params := url.Values{
		"client_id": {GitHubClientID},
		"scope":     {GitHubScopes},
	}

	req, err := http.NewRequest(http.MethodPost, GitHubDeviceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build github device code request: %w", err)
	}
	req.URL.RawQuery = params.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", gitHubUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github device code request: %w", err)
	}
	defer resp.Body.Close()

	var data GitHubDeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode github device code response: %w", err)
	}
	return &data, nil
}

// PollGitHubDeviceCode polls for a token once the user has approved the device.
func PollGitHubDeviceCode(deviceCode string) (*GitHubTokenResponse, error) {
	params := url.Values{
		"client_id":   {GitHubClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequest(http.MethodPost, GitHubTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build github poll request: %w", err)
	}
	req.URL.RawQuery = params.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", gitHubUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github poll request: %w", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode github poll response: %w", err)
	}

	if errStr, ok := data["error"].(string); ok {
		return nil, fmt.Errorf("%s", errStr)
	}

	accessToken, _ := data["access_token"].(string)
	if accessToken == "" {
		return nil, fmt.Errorf("no access_token in poll response")
	}
	return &GitHubTokenResponse{
		AccessToken: accessToken,
		TokenType:   stringVal(data, "token_type"),
		Scope:       stringVal(data, "scope"),
	}, nil
}

func stringVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}
