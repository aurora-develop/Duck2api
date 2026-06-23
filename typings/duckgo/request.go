package duckgo

type DurableStream struct {
	MessageID      string    `json:"messageId"`
	ConversationID string    `json:"conversationId"`
	PublicKey      PublicKey `json:"publicKey"`
}

type Metadata struct {
	ToolChoice ToolChoice `json:"toolChoice"`
}

type PublicKey struct {
	Alg    string   `json:"alg"`
	E      string   `json:"e"`
	Ext    bool     `json:"ext"`
	KeyOps []string `json:"key_ops"`
	Kty    string   `json:"kty"`
	N      string   `json:"n"`
	Use    string   `json:"use"`
}

type ToolChoice struct {
	NewsSearch      bool `json:"NewsSearch"`
	VideosSearch    bool `json:"VideosSearch"`
	LocalSearch     bool `json:"LocalSearch"`
	WeatherForecast bool `json:"WeatherForecast"`
}

type ApiRequest struct {
	Model                      string        `json:"model"`
	Metadata                   Metadata      `json:"metadata"`
	Messages                   []messages    `json:"messages"`
	CanUseTools                bool          `json:"canUseTools"`
	ReasoningEffort            string        `json:"reasoningEffort"`
	CanUseApproxLocation       *bool          `json:"canUseApproxLocation"`
	CanDelegateImageGeneration *bool          `json:"canDelegateImageGeneration"`
	DurableStream              DurableStream `json:"durableStream"`
}

type messages struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (a *ApiRequest) AddMessage(role string, content string) {
	a.Messages = append(a.Messages, messages{
		Role:    role,
		Content: content,
	})
}

func NewApiRequest(model string) ApiRequest {
	return ApiRequest{
		Model:                      model,
		CanUseTools:                true,
		ReasoningEffort:            "none",
		CanUseApproxLocation:       nil,
		CanDelegateImageGeneration: nil,
		Metadata: Metadata{
			ToolChoice: ToolChoice{
				NewsSearch:      false,
				VideosSearch:    false,
				LocalSearch:     false,
				WeatherForecast: false,
			},
		},
	}
}
