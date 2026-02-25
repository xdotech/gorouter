package providers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	IFlowAuthURL      = "https://iflow.cn/oauth"
	IFlowTokenURL     = "https://iflow.cn/oauth/token"
	IFlowClientID     = "10009311001"
	IFlowClientSecret = "4Z3YjXycVsQvyGF1etiNlIBB4RsqSDtW"
	IFlowRedirectURI  = "{baseURL}/api/oauth/if/callback"
)

// IFlowTokenResponse is the response from iFlow's token endpoint.
type IFlowTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// BuildIFlowAuthURL constructs the iFlow OAuth authorization URL.
func BuildIFlowAuthURL(baseURL, state, _ string) string {
	redirectURI := strings.Replace(IFlowRedirectURI, "{baseURL}", baseURL, 1)
	params := url.Values{
		"loginMethod": {"phone"},
		"type":        {"phone"},
		"redirect":    {redirectURI},
		"state":       {state},
		"client_id":   {IFlowClientID},
	}
	return IFlowAuthURL + "?" + params.Encode()
}

// ExchangeIFlowCode exchanges an authorization code for tokens using Basic Auth.
func ExchangeIFlowCode(code, _, redirectURI string) (accessToken, refreshToken string, err error) {
	basicAuth := base64.StdEncoding.EncodeToString([]byte(IFlowClientID + ":" + IFlowClientSecret))

	params := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {IFlowClientID},
		"client_secret": {IFlowClientSecret},
	}

	req, err := http.NewRequest(http.MethodPost, IFlowTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("build iflow token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+basicAuth)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("iflow token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return "", "", fmt.Errorf("iflow token exchange failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens IFlowTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return "", "", fmt.Errorf("decode iflow token response: %w", err)
	}
	return tokens.AccessToken, tokens.RefreshToken, nil
}

// RefreshIFlowToken obtains new tokens using a refresh token.
func RefreshIFlowToken(refreshToken string) (accessToken, newRefreshToken string, err error) {
	basicAuth := base64.StdEncoding.EncodeToString([]byte(IFlowClientID + ":" + IFlowClientSecret))

	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {IFlowClientID},
		"client_secret": {IFlowClientSecret},
	}

	req, err := http.NewRequest(http.MethodPost, IFlowTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("build iflow refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+basicAuth)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("iflow refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return "", "", fmt.Errorf("iflow token refresh failed (%d): %v", resp.StatusCode, errBody)
	}

	var tokens IFlowTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return "", "", fmt.Errorf("decode iflow refresh response: %w", err)
	}
	return tokens.AccessToken, tokens.RefreshToken, nil
}
