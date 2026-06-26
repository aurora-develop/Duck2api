package duckgo

import (
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

func capReasoningEffort(model string, effort string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	switch effort {
	case "high", "max", "xhigh":
		return "low"
	default:
		return "none"
	}
}

func ConvertAPIRequest(apiRequest officialtypes.APIRequest) duckgotypes.ApiRequest {
	return ConvertAPIRequestWithOptions(apiRequest, "", false)
}

func ConvertAPIRequestWithOptions(apiRequest officialtypes.APIRequest, reasoningEffort string, webSearch bool) duckgotypes.ApiRequest {
	inputModel := apiRequest.Model
	duckgoRequest := duckgotypes.NewApiRequest(inputModel)
	realModel := inputModel

	modelLower := strings.ToLower(inputModel)
	switch {
	case strings.HasPrefix(modelLower, "gpt-3.5"):
		realModel = "gpt-4o-mini"
	case strings.HasPrefix(modelLower, "claude-3-haiku"):
		realModel = "claude-3-haiku-20240307"
	case strings.HasPrefix(modelLower, "llama-3.3-70b"):
		realModel = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	case strings.HasPrefix(modelLower, "mixtral-8x7b"):
		realModel = "mistralai/Mixtral-8x7B-Instruct-v0.1"
	case strings.HasPrefix(modelLower, "llama-4-scout"):
		realModel = "meta-llama/Llama-4-Scout-17B-16E-Instruct"
	case strings.HasPrefix(modelLower, "mistral-small"):
		realModel = "mistralai/Mistral-Small-24B-Instruct-2501"
	}

	duckgoRequest.Model = realModel

	// Set reasoning effort (cap to model's max)
	if reasoningEffort != "" {
		duckgoRequest.ReasoningEffort = capReasoningEffort(realModel, reasoningEffort)
	} else {
		duckgoRequest.ReasoningEffort = "none" // fast mode
	}

	// Set web search
	if webSearch {
		duckgoRequest.Metadata.ToolChoice.WebSearch = true
	}

	for _, message := range apiRequest.Messages {
		role := message.Role
		if role == "system" {
			role = "user"
		}
		if role != "user" && role != "assistant" {
			continue
		}

		parts := extractContentParts(message.Content)
		if len(parts) > 0 {
			duckgoRequest.AddMessageWithParts(role, parts)
		}
	}
	duckgoRequest.DurableStream = newDurableStream()
	return duckgoRequest
}

// FileStore is a function to look up file content by ID
// Set this from the handler to enable file resolution
var FileStore func(fileID string) (filename string, mimeType string, data []byte, ok bool)

func extractContentParts(content interface{}) []duckgotypes.ContentPart {
	if content == nil {
		return nil
	}

	// String content
	if str, ok := content.(string); ok {
		if str == "" {
			return nil
		}
		return []duckgotypes.ContentPart{{Type: "text", Text: str}}
	}

	// Array content (multimodal)
	if arrayContent, ok := content.([]interface{}); ok {
		var parts []duckgotypes.ContentPart
		for _, element := range arrayContent {
			elementMap, ok := element.(map[string]interface{})
			if !ok {
				continue
			}
			partType, _ := elementMap["type"].(string)
			switch partType {
			case "text":
				if text, ok := elementMap["text"].(string); ok && text != "" {
					parts = append(parts, duckgotypes.ContentPart{Type: "text", Text: text})
				}
			case "image_url":
				// OpenAI format: {"type":"image_url","image_url":{"url":"data:image/png;base64,..."}}
				if imageURL, ok := elementMap["image_url"].(map[string]interface{}); ok {
					if url, ok := imageURL["url"].(string); ok && url != "" {
						mimeType := "image/png"
						if strings.HasPrefix(url, "data:") {
							if idx := strings.Index(url, ";"); idx > 0 {
								mimeType = url[5:idx]
							}
						}
						parts = append(parts, duckgotypes.ContentPart{
							Type:     "image",
							Image:    url,
							MimeType: mimeType,
						})
					}
				}
			case "image":
				// DuckDuckGo native format: {"type":"image","mimeType":"image/webp","image":"data:..."}
				if image, ok := elementMap["image"].(string); ok && image != "" {
					mimeType, _ := elementMap["mimeType"].(string)
					if mimeType == "" {
						mimeType = "image/png"
					}
					parts = append(parts, duckgotypes.ContentPart{
						Type:     "image",
						Image:    image,
						MimeType: mimeType,
					})
				}
			case "input_file":
				// OpenAI format: {"type":"input_file","file_id":"file-xxx"}
				if fileID, ok := elementMap["file_id"].(string); ok && fileID != "" {
					if FileStore != nil {
						if filename, mimeType, data, ok := FileStore(fileID); ok {
							b64 := base64.StdEncoding.EncodeToString(data)
							// Image files → send as image part
							if strings.HasPrefix(mimeType, "image/") {
								parts = append(parts, duckgotypes.ContentPart{
									Type:     "image",
									Image:    "data:" + mimeType + ";base64," + b64,
									MimeType: mimeType,
								})
							} else {
								// Non-image files (PDF, text, etc.) → embed as text context
								fileText := string(data)
								if len(fileText) > 50000 {
									fileText = fileText[:50000] + "\n...(truncated)"
								}
								parts = append(parts, duckgotypes.ContentPart{
									Type: "text",
									Text: "[File: " + filename + "]\n" + fileText,
								})
							}
							continue
						}
					}
					// Fallback: send file ID as text reference
					parts = append(parts, duckgotypes.ContentPart{
						Type: "text",
						Text: "[file:" + fileID + "]",
					})
				}
			}
		}
		return parts
	}

	return nil
}

// extractContent extracts plain text content (backward compatible)
func extractContent(content interface{}) string {
	parts := extractContentParts(content)
	if parts == nil {
		return ""
	}
	var text strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			text.WriteString(p.Text)
		}
	}
	return text.String()
}

func newDurableStream() duckgotypes.DurableStream {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return duckgotypes.DurableStream{}
	}

	return duckgotypes.DurableStream{
		MessageID:      uuid.NewString(),
		ConversationID: uuid.NewString(),
		PublicKey: duckgotypes.PublicKey{
			Alg:    "RSA-OAEP-256",
			E:      base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			Ext:    true,
			KeyOps: []string{"encrypt"},
			Kty:    "RSA",
			N:      base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			Use:    "enc",
		},
	}
}
