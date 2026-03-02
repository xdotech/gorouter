package translator

import (
	"fmt"

	"github.com/xdotech/gorouter/internal/translator/request"
	"github.com/xdotech/gorouter/internal/translator/response"
	"github.com/xdotech/gorouter/internal/translator/types"
)

// TranslateRequest translates a request body from sourceFormat to targetFormat.
// Currently supports OpenAI → Claude and OpenAI → Gemini translations.
// For same-format or unsupported pairs, the body is returned unchanged.
func TranslateRequest(body map[string]interface{}, sourceFormat, targetFormat string) (map[string]interface{}, error) {
	if body == nil {
		return body, nil
	}
	if sourceFormat == targetFormat {
		return body, nil
	}

	// Normalise to OpenAI first if source is not OpenAI (placeholder for future translators)
	openAIBody := body

	// Translate OpenAI → target
	switch targetFormat {
	case FormatClaude, FormatAntigravity:
		return request.OpenAIToClaude(openAIBody)
	case FormatGemini, FormatGeminiCLI:
		return request.OpenAIToGemini(openAIBody)
	case FormatOpenAI, FormatOpenAIResp:
		return openAIBody, nil
	default:
		return nil, fmt.Errorf("unsupported target format: %s", targetFormat)
	}
}

// NewStreamTranslator returns the appropriate StreamTranslator for converting upstream
// provider SSE events to OpenAI SSE format.
func NewStreamTranslator(targetFormat, model string) types.StreamTranslator {
	switch targetFormat {
	case FormatClaude, FormatAntigravity:
		return response.NewClaudeStreamTranslator(model)
	case FormatGemini, FormatGeminiCLI:
		return response.NewGeminiStreamTranslator(model)
	case FormatOpenAIResp:
		return response.NewCodexStreamTranslator(model)
	default:
		// OpenAI-compatible upstream: no translation needed (passthrough)
		return nil
	}
}
