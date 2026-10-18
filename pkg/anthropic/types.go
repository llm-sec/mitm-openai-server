package anthropic

// MessagesRequest Messages API 请求体（只需解析用到的字段）
type MessagesRequest struct {
	Model         string                 `json:"model"`
	MaxTokens     int                    `json:"max_tokens"`
	Messages      []RequestMessage       `json:"messages"`
	System        interface{}            `json:"system"`
	Stream        bool                   `json:"stream"`
	Tools         interface{}            `json:"tools"`
	Thinking      interface{}            `json:"thinking"`
	Temperature   *float64               `json:"temperature"`
	TopP          *float64               `json:"top_p"`
	Metadata      map[string]interface{} `json:"metadata"`
	StopSequences []string               `json:"stop_sequences"`
	ServiceTier   string                 `json:"service_tier"`
	Speed         string                 `json:"speed"`
}

// RequestMessage 消息（content 可以是字符串或内容块数组）
type RequestMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ContentBlock 响应内容块
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// tool_use 相关字段（模拟时一般不会出现，但保持结构完整）
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// MessageResponse Messages API 标准响应
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	Content      []ContentBlock `json:"content"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// Usage Token用量
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ErrorResponse Anthropic标准错误响应
// { "type": "error", "error": { "type": "invalid_request_error", "message": "..." } }
type ErrorResponse struct {
	Type  string      `json:"type"`
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// CountTokensRequest count_tokens 端点请求体
type CountTokensRequest struct {
	Model    string           `json:"model"`
	Messages []RequestMessage `json:"messages"`
	System   interface{}      `json:"system"`
	Tools    interface{}      `json:"tools"`
}

// CountTokensResponse count_tokens 端点响应
type CountTokensResponse struct {
	InputTokens int `json:"input_tokens"`
}

// ModelInfo /v1/models 返回的模型信息（Anthropic格式）
type ModelInfo struct {
	Type            string                 `json:"type"`
	ID              string                 `json:"id"`
	DisplayName     string                 `json:"display_name"`
	CreatedAt       string                 `json:"created_at"`
	Lifecycle       string                 `json:"lifecycle,omitempty"`
	MaxInputTokens  int                    `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]interface{} `json:"capabilities,omitempty"`
}

// ListModelsResponse 模型列表响应
type ListModelsResponse struct {
	Data    []ModelInfo `json:"data"`
	HasMore bool        `json:"has_more"`
	FirstID *string     `json:"first_id"`
	LastID  *string     `json:"last_id"`
}
