package official

import "encoding/json"

type ChatCompletionChunk struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
}

func (chunk *ChatCompletionChunk) String() string {
	resp, _ := json.Marshal(chunk)
	return string(resp)
}

type Choices struct {
	Delta        Delta       `json:"delta"`
	Index        int         `json:"index"`
	FinishReason interface{} `json:"finish_reason"`
}

type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

func NewChatCompletionChunk(text string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "gpt-4o-mini",
		Choices: []Choices{
			{
				Index: 0,
				Delta: Delta{
					Content: text,
				},
				FinishReason: nil,
			},
		},
	}
}

func NewChatCompletionChunkWithModel(text string, model string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []Choices{
			{
				Index: 0,
				Delta: Delta{
					Content: text,
				},
				FinishReason: nil,
			},
		},
	}
}

func StopChunkWithModel(reason string, model string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []Choices{
			{
				Index:        0,
				FinishReason: reason,
			},
		},
	}
}

func StopChunk(reason string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "gpt-4o-mini",
		Choices: []Choices{
			{
				Index:        0,
				FinishReason: reason,
			},
		},
	}
}

type ChatCompletion struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Usage   usage    `json:"usage"`
	Choices []Choice `json:"choices"`
}
type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Choice struct {
	Index        int         `json:"index"`
	Message      Msg         `json:"message"`
	FinishReason interface{} `json:"finish_reason"`
}
type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ResponseAPI struct {
	ID                 string                 `json:"id"`
	Object             string                 `json:"object"`
	CreatedAt          int64                  `json:"created_at"`
	Status             string                 `json:"status"`
	Model              string                 `json:"model"`
	Output             []ResponseOutput       `json:"output"`
	OutputText         string                 `json:"output_text"`
	Usage              ResponseUsage          `json:"usage"`
	ParallelToolCalls  bool                   `json:"parallel_tool_calls"`
	PreviousResponseID interface{}            `json:"previous_response_id"`
	Error              interface{}            `json:"error"`
	IncompleteDetails  interface{}            `json:"incomplete_details"`
	Metadata           map[string]interface{} `json:"metadata"`
}

type ResponseOutput struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Status  string                  `json:"status"`
	Role    string                  `json:"role"`
	Content []ResponseOutputContent `json:"content"`
}

type ResponseOutputContent struct {
	Type        string        `json:"type"`
	Text        string        `json:"text"`
	Annotations []interface{} `json:"annotations"`
}

type ResponseUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	InputTokensDetails  any `json:"input_tokens_details,omitempty"`
	OutputTokensDetails any `json:"output_tokens_details,omitempty"`
}

type ResponseStreamEvent struct {
	Type         string          `json:"type"`
	Sequence     int             `json:"sequence_number,omitempty"`
	Response     *ResponseAPI    `json:"response,omitempty"`
	Item         *ResponseOutput `json:"item,omitempty"`
	ItemID       string          `json:"item_id,omitempty"`
	Part         any             `json:"part,omitempty"`
	OutputIndex  int             `json:"output_index,omitempty"`
	ContentIndex int             `json:"content_index,omitempty"`
	Delta        string          `json:"delta,omitempty"`
	Text         string          `json:"text,omitempty"`
}

func (event ResponseStreamEvent) String() string {
	resp, _ := json.Marshal(event)
	return string(resp)
}

func NewResponseAPIWithModel(text string, model string) ResponseAPI {
	if model == "" {
		model = "gpt-5-mini"
	}
	return ResponseAPI{
		ID:        "resp_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:    "response",
		CreatedAt: 0,
		Status:    "completed",
		Model:     model,
		Output: []ResponseOutput{
			NewResponseOutput(text),
		},
		OutputText:        text,
		Usage:             ResponseUsage{},
		ParallelToolCalls: true,
		Metadata:          map[string]interface{}{},
	}
}

func NewResponseOutput(text string) ResponseOutput {
	return ResponseOutput{
		ID:     "msg_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Type:   "message",
		Status: "completed",
		Role:   "assistant",
		Content: []ResponseOutputContent{
			{
				Type:        "output_text",
				Text:        text,
				Annotations: []interface{}{},
			},
		},
	}
}

func NewChatCompletionWithModel(text string, model string) ChatCompletion {
	return ChatCompletion{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: int64(0),
		Model:   model,
		Usage: usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
		Choices: []Choice{
			{
				Message: Msg{
					Content: text,
					Role:    "assistant",
				},
				Index: 0,
			},
		},
	}
}

func NewChatCompletion(full_test string, input_tokens, output_tokens int) ChatCompletion {
	return ChatCompletion{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: int64(0),
		Model:   "gpt-4o-mini",
		Usage: usage{
			PromptTokens:     input_tokens,
			CompletionTokens: output_tokens,
			TotalTokens:      input_tokens + output_tokens,
		},
		Choices: []Choice{
			{
				Message: Msg{
					Content: full_test,
					Role:    "assistant",
				},
				Index: 0,
			},
		},
	}
}
