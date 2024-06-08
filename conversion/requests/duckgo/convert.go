package duckgo

import (
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"strings"
)

func ConvertAPIRequest(api_request officialtypes.APIRequest) duckgotypes.ApiRequest {
	// 默认模型3.5
	duckgo_request := duckgotypes.NewApiRequest("gpt-3.5-turbo-0125")
	// 检查并更新模型为 claude- 开头的情况
	if strings.HasPrefix(strings.ToLower(api_request.Model), "claude") {
		duckgo_request.Model = "claude-3-haiku-20240307"
	}
	content := buildContent(&api_request)
	duckgo_request.AddMessage("user", content)
	return duckgo_request
}

func buildContent(api_request *officialtypes.APIRequest) string {
	var content strings.Builder
	for _, apiMessage := range api_request.Messages {
		role := apiMessage.Role
		if role == "user" || role == "system" || role == "assistant" {
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
