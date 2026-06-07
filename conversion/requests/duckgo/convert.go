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

func ConvertAPIRequest(apiRequest officialtypes.APIRequest) duckgotypes.ApiRequest {
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
	for _, message := range apiRequest.Messages {
		role := message.Role
		if role == "system" {
			role = "user"
		}
		if role != "user" && role != "assistant" {
			continue
		}

		content := extractContent(message.Content)
		if content != "" {
			duckgoRequest.AddMessage(role, content)
		}
	}
	duckgoRequest.DurableStream = newDurableStream()
	return duckgoRequest
}

func extractContent(content interface{}) string {
	if arrayContent, ok := content.([]interface{}); ok {
		var text strings.Builder
		for _, element := range arrayContent {
			elementMap, ok := element.(map[string]interface{})
			if !ok || elementMap["type"] != "text" {
				continue
			}
			contentStr, _ := elementMap["text"].(string)
			text.WriteString(contentStr)
		}
		return text.String()
	}

	contentStr, _ := content.(string)
	return contentStr
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
