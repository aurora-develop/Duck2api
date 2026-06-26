package official

import (
	"fmt"
	"strings"
)

type APIRequest struct {
	Messages  []ApiMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	Model     string        `json:"model"`
	PluginIDs []string      `json:"plugin_ids"`
	// Extra fields for Duck.ai features (not standard OpenAI)
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // "none", "low", "medium", "high"
	WebSearch       *bool  `json:"web_search,omitempty"`       // enable web search
}

type ApiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ResponseAPIRequest struct {
	Model              string      `json:"model"`
	Input              interface{} `json:"input"`
	Instructions       string      `json:"instructions"`
	Stream             bool        `json:"stream"`
	PreviousResponseID string      `json:"previous_response_id"`
	MaxOutputTokens    int         `json:"max_output_tokens"`
	Tools              interface{} `json:"tools"`
	ToolChoice         interface{} `json:"tool_choice"`
}

func (r ResponseAPIRequest) ToChatCompletionRequest() APIRequest {
	request := APIRequest{
		Model:  r.Model,
		Stream: r.Stream,
	}

	if strings.TrimSpace(request.Model) == "" {
		request.Model = "gpt-5-mini"
	}
	if strings.TrimSpace(r.Instructions) != "" {
		request.Messages = append(request.Messages, ApiMessage{
			Role:    "system",
			Content: r.Instructions,
		})
	}

	request.Messages = append(request.Messages, responseInputMessages(r.Input)...)
	return request
}

func responseInputMessages(input interface{}) []ApiMessage {
	switch value := input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []ApiMessage{{Role: "user", Content: value}}
	case []interface{}:
		messages := make([]ApiMessage, 0, len(value))
		for _, item := range value {
			messages = append(messages, responseInputItemToMessages(item)...)
		}
		return messages
	default:
		return nil
	}
}

func responseInputItemToMessages(item interface{}) []ApiMessage {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}

	itemType, _ := itemMap["type"].(string)
	switch itemType {
	case "message", "":
		role, _ := itemMap["role"].(string)
		if role == "" {
			role = "user"
		}
		content := responseContentText(itemMap["content"])
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []ApiMessage{{Role: role, Content: content}}
	case "function_call_output":
		output := responseContentText(itemMap["output"])
		if output == "" {
			return nil
		}
		callID, _ := itemMap["call_id"].(string)
		if callID != "" {
			output = fmt.Sprintf("Tool output for %s:\n%s", callID, output)
		}
		return []ApiMessage{{Role: "user", Content: output}}
	default:
		return nil
	}
}

func responseContentText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	case []interface{}:
		var text strings.Builder
		for _, part := range value {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			partType, _ := partMap["type"].(string)
			switch partType {
			case "input_text", "output_text", "text", "":
				if partText, ok := partMap["text"].(string); ok {
					text.WriteString(partText)
				}
			}
		}
		return text.String()
	default:
		return ""
	}
}

type OpenAISessionToken struct {
	SessionToken string `json:"session_token"`
}

type OpenAIRefreshToken struct {
	RefreshToken string `json:"refresh_token"`
}
