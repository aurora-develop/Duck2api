package duckgo

import (
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"strings"
)

func ConvertAPIRequest(api_request officialtypes.APIRequest) duckgotypes.ApiRequest {
	inputModel := api_request.Model
	duckgo_request := duckgotypes.NewApiRequest(inputModel)
	realModel := inputModel

	// 如果模型未进行映射，则直接使用输入模型，方便后续用户使用 duckduckgo 添加的新模型。
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
	}

	duckgo_request.Model = realModel
	content := buildContent(&api_request)
	duckgo_request.AddMessage("user", content)

	return duckgo_request
}

func buildContent(api_request *officialtypes.APIRequest) string {
	var content strings.Builder
	for _, apiMessage := range api_request.Messages {
		role := apiMessage.Role
		if role == "user" || role == "system" || role == "assistant" {
			if role == "system" {
				role = "user"
			}
			contentStr := ""
			// 判断 apiMessage.Content 是否为数组
			if arrayContent, ok := apiMessage.Content.([]interface{}); ok {
				// 如果是数组，遍历数组，查找第一个 type 为 "text" 的元素
				for _, element := range arrayContent {
					if elementMap, ok := element.(map[string]interface{}); ok {
						if elementMap["type"] == "text" {
							contentStr = elementMap["text"].(string)
							break
						}
					}
				}
			} else {
				contentStr, _ = apiMessage.Content.(string)
			}
			content.WriteString(role + ":" + contentStr + ";\r\n")
		}
	}
	return content.String()
}
