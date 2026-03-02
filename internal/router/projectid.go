package router

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// fetchProjectID calls Cloud Code loadCodeAssist to discover the project ID
// bound to the authenticated user. Matches 9router's approach.
func fetchProjectID(accessToken string) string {
	payload := `{"metadata":{"ideType":9,"platform":1,"pluginType":2},"mode":1}`
	req, err := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		strings.NewReader(payload))
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Name", "antigravity")
	req.Header.Set("X-Client-Version", "1.107.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		CloudAICompanionProject string `json:"cloudaicompanionProject"`
	}
	if json.Unmarshal(body, &result) != nil {
		return ""
	}
	return result.CloudAICompanionProject
}
