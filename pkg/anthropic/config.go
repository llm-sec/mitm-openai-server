package anthropic

// Config 定义Anthropic（Claude）服务配置
type Config struct {
	// 是否启用这个服务
	Enabled bool `json:"enabled"`

	// 模拟响应的延迟毫秒数
	ResponseDelayMs int `json:"response_delay_ms"`

	// API密钥验证设置（验证客户端发来的 x-api-key / Authorization）
	APIKeyAuth bool `json:"api_key_auth"`

	// 用于验证的API密钥（与config.Token一致时自动生效）
	APIKey string `json:"api_key"`

	// 自定义响应内容
	CustomResponse string `json:"custom_response"`

	// 模型列表文件路径
	ModelsFile string `json:"models_file"`

	// 是否启用代理模式
	ProxyMode bool `json:"proxy_mode"`

	// 目标API URL（默认 https://api.anthropic.com）
	TargetURL string `json:"target_url"`

	// 目标认证类型 (none, api_key, bearer)
	TargetAuthType string `json:"target_auth_type"`

	// 目标API密钥（x-api-key）
	TargetAPIKey string `json:"target_api_key"`

	// 目标Bearer Token（Authorization: Bearer）
	TargetToken string `json:"target_token"`

	// 使用的API版本头（anthropic-version），默认 2023-06-01
	AnthropicVersion string `json:"anthropic_version"`
}

// DefaultTargetURL Anthropic官方API地址
const DefaultTargetURL = "https://api.anthropic.com"

// DefaultAnthropicVersion 默认的API版本
const DefaultAnthropicVersion = "2023-06-01"

// DefaultConfig 返回默认的Anthropic服务配置
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		ResponseDelayMs:  100,
		APIKeyAuth:       false,
		ProxyMode:        false,
		TargetAuthType:   "none",
		AnthropicVersion: DefaultAnthropicVersion,
	}
}
