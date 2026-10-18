package api

import (
	"github.com/gin-gonic/gin"
	"github.com/llm-sec/mitm-openai-server/pkg/openai"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
)

// ServerConfig 表示服务器配置
type ServerConfig struct {
	// 存储
	Storage storage.Storage

	// 认证
	EnableAuth bool
	AuthType   string
	Username   string
	Password   string
	Token      string

	// UI
	UIDir          string
	UIUsername     string
	UIPassword     string
	GenerateUIAuth bool

	// CORS
	EnableCORS   bool
	AllowOrigins string

	// 代理模式
	ProxyMode      bool
	TargetURL      string
	TargetAuthType string
	TargetUsername string
	TargetPassword string
	TargetToken    string

	// TLS/HTTPS
	EnableTLS   bool   // 启用HTTPS
	AutoTLS     bool   // 自动生成自签名证书
	TLSCertFile string // TLS证书文件路径
	TLSKeyFile  string // TLS私钥文件路径

	// Anthropic（Claude Code）协议支持
	// 未配置目标URL时使用内置模拟服务，配置后开启代理模式
	AnthropicTargetURL      string // 目标Anthropic API地址（如 https://api.anthropic.com）
	AnthropicTargetAuthType string // 目标认证类型 (none, api_key, bearer)
	AnthropicAPIKey         string // 目标x-api-key
	AnthropicAuthToken      string // 目标Authorization: Bearer token
	AnthropicModelsFile     string // 模型列表文件路径（mock模式）
}

// OpenAIServiceInterface 使用 openai.Service 接口
// 这样可以在 api 包中使用 openai 包的接口定义
type OpenAIServiceInterface = openai.Service

// UIServerInterface 定义了UI服务器的接口
type UIServerInterface interface {
	// SetupUIRoutes 设置UI相关的路由
	SetupUIRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc, apiMiddleware gin.HandlerFunc)

	// HandleUILogin 处理UI登录请求
	HandleUILogin(c *gin.Context)

	// GetServerInfo 返回服务器信息
	GetServerInfo(c *gin.Context)

	// GetProxyConfig 获取代理配置
	GetProxyConfig(c *gin.Context)

	// SaveProxyConfig 保存代理配置
	SaveProxyConfig(c *gin.Context)

	// GetRequestByID 获取特定ID的请求
	GetRequestByID(c *gin.Context)

	// GetRequests 获取请求列表
	GetRequests(c *gin.Context)

	// DeleteRequest 删除特定ID的请求
	DeleteRequest(c *gin.Context)

	// DeleteAllRequests 删除所有请求
	DeleteAllRequests(c *gin.Context)

	// ExportRequests 导出请求
	ExportRequests(c *gin.Context)

	// GetStorageStats 获取存储统计信息
	GetStorageStats(c *gin.Context)

	// GetAPIToken 获取API Token
	GetAPIToken(c *gin.Context)

	// HandleUIChat 处理聊天请求
	HandleUIChat(c *gin.Context)

	// SetConfig 设置或更新服务器配置
	SetConfig(config ServerConfig)

	// ValidateToken 验证令牌是否有效
	ValidateToken(token string) bool
}

// ConfigManagerInterface 定义配置管理接口
type ConfigManagerInterface interface {
	LoadConfig() (interface{}, error)
	SaveConfig(config interface{}) error
	ApplyConfig(config interface{}, server interface{}) error
}

// StandardResponse 标准API响应格式
type StandardResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// 共享的请求和响应结构体
type Request struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Timestamp string                 `json:"timestamp"`
	IPAddress string                 `json:"ip_address"`
	Headers   map[string]string      `json:"headers"`
	Query     map[string]string      `json:"query"`
	Body      interface{}            `json:"body"`
	Response  *ProxyResponse         `json:"response,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ProxyResponse 代理响应
type ProxyResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       interface{}       `json:"body"`
	Latency    int64             `json:"latency_ms,omitempty"`
}

// 提供给server包的辅助函数

// UIServerFactory 创建UI服务器
func UIServerFactory(storage storage.Storage, config ServerConfig, openaiService OpenAIServiceInterface) UIServerInterface {
	return NewUIServer(storage, config, openaiService)
}
