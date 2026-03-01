package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	QwenDeviceCodeURL = "https://chat.qwen.ai/api/v1/oauth2/device/code"
	QwenTokenURL      = "https://chat.qwen.ai/api/v1/oauth2/token"
	QwenClientID      = "f0304373b74a44d2b584a3fb70ca9e56"
	QwenScope         = "openid profile email model.completion"
)

// QwenDeviceCodeResponse is the response from Qwen's device code endpoint.
type QwenDeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// StartQwenDeviceCode initiates a Qwen device code flow with PKCE.
func StartQwenDeviceCode(codeChallenge string) (*QwenDeviceCodeResponse, error) {
	params := url.Values{
		"client_id":             {QwenClientID},
		"scope":                 {QwenScope},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	req, err := http.NewRequest(http.MethodPost, QwenDeviceCodeURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build qwen device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qwen device code request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("qwen device code request failed (%d): %v", resp.StatusCode, errBody)
	}

	var data QwenDeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode qwen device code response: %w", err)
	}
	return &data, nil
}

// PollQwenDeviceCode polls Qwen's token endpoint for a completed device authorization.
// Returns (accessToken, refreshToken, error).
// Caller should check for "authorization_pending" error and retry.
func PollQwenDeviceCode(deviceCode, codeVerifier string) (accessToken, refreshToken string, err error) {
	params := url.Values{
		"grant_type":    {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":     {QwenClientID},
		"device_code":   {deviceCode},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequest(http.MethodPost, QwenTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("build qwen poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("qwen poll request: %w", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("decode qwen poll response: %w", err)
	}

	if errStr, ok := data["error"].(string); ok {
		return "", "", fmt.Errorf("%s", errStr)
	}

	at, _ := data["access_token"].(string)
	rt, _ := data["refresh_token"].(string)
	if at == "" {
		return "", "", fmt.Errorf("no access_token in qwen poll response")
	}
	return at, rt, nil
}

// RefreshQwenToken refreshes Qwen tokens using a refresh token.
func RefreshQwenToken(refreshToken string) (accessToken, newRefreshToken string, err error) {
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {QwenClientID},
	}

	req, err := http.NewRequest(http.MethodPost, QwenTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("build qwen refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("qwen refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return "", "", fmt.Errorf("qwen token refresh failed (%d): %v", resp.StatusCode, errBody)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("decode qwen refresh response: %w", err)
	}

	at, _ := data["access_token"].(string)
	rt, _ := data["refresh_token"].(string)
	if at == "" {
		return "", "", fmt.Errorf("no access_token in qwen refresh response")
	}
	return at, rt, nil
}
