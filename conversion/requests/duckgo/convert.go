package duckgo

import (
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"strings"
)

func ConvertAPIRequest(api_request officialtypes.APIRequest) duckgotypes.ApiRequest {
	// 默认模型3.5
	duckgo_request := duckgotypes.NewApiRequest("gpt-4o-mini")
	// 检查并更新模型为 claude- 开头的情况
	if strings.HasPrefix(strings.ToLower(api_request.Model), "claude") {
		duckgo_request.Model = "claude-3-haiku-20240307"
	}
	if strings.HasPrefix(strings.ToLower(api_request.Model), "Llama-3-70b") {

		duckgo_request.Model = "meta-llama/Llama-3-70b-chat-hf"
	}
	if strings.HasPrefix(strings.ToLower(api_request.Model), "Mixtral-8x7B") {

		duckgo_request.Model = "mistralai/Mixtral-8x7B-Instruct-v0.1"
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
			content.WriteString(role + ":" + apiMessage.Content + ";\r\n")
		}
	}
	return content.String()
}
