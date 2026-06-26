package duckgo

import "encoding/json"

type ImagePartData struct {
	B64Image string `json:"b64Image"`
	Format   string `json:"format"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Status   string `json:"status,omitempty"`
	Title    string `json:"title,omitempty"`
	Type     string `json:"type,omitempty"`
}

type ImagePart struct {
	Type   string        `json:"type"`
	Result string        `json:"result,omitempty"`
	Format string        `json:"format,omitempty"`
	Width  int           `json:"width,omitempty"`
	Height int           `json:"height,omitempty"`
	Data   *ImagePartData `json:"data,omitempty"`
}

type ApiResponse struct {
	Message    string          `json:"message"`
	Created    int             `json:"created"`
	Id         string          `json:"id"`
	Action     string          `json:"action"`
	Model      string          `json:"model"`
	Role       string          `json:"role,omitempty"`
	State      string          `json:"state,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	ToolCallId string          `json:"toolCallId,omitempty"`
	Result     string          `json:"result,omitempty"`
	Parts      []ImagePart     `json:"parts,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// GetImageData extracts b64Image from the data field
func (r *ApiResponse) GetImageData() *ImagePartData {
	if r.Data == nil {
		return nil
	}
	var d ImagePartData
	if err := json.Unmarshal(r.Data, &d); err != nil {
		return nil
	}
	if d.B64Image == "" {
		return nil
	}
	return &d
}
