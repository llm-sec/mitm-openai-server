package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
	"github.com/llm-sec/mitm-openai-server/pkg/utils"
)

// Handler 处理OpenAI API请求的处理器
type Handler struct {
	storage       storage.Storage
	openaiService Service
}

// NewHandler 创建一个新的OpenAI处理器
func NewHandler(storage storage.Storage, openaiService Service) *Handler {
	return &Handler{
		storage:       storage,
		openaiService: openaiService,
	}
}

// SetupRoutes 设置OpenAI API代理路由
// 配置所有与OpenAI API相关的路由
func (h *Handler) SetupRoutes(router *gin.Engine, apiMiddleware gin.HandlerFunc) {
	// 如果没有初始化openaiService，跳过设置OpenAI路由
	if h.openaiService == nil {
		fmt.Println("警告: openaiService未初始化，跳过OpenAI路由设置")
		return
	}

	// 添加OpenAI API规范路由 - 这个路由只添加一次
	router.GET("/openai.json", h.openaiService.ServeOpenAISpec)

	// OpenAI API路由组
	openaiGroup := router.Group("/v1")

	// 应用API中间件记录请求
	openaiGroup.Use(apiMiddleware)

	// 使用更宽松的CORS设置，确保跨域请求能正常工作
	openaiGroup.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			// 如果没有Origin头，允许所有源
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// 有Origin头，设置为请求的源
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24小时

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 注册所有需要的OpenAI API路径
	openaiGroup.POST("/chat/completions", h.HandleRequest)
	openaiGroup.POST("/completions", h.HandleRequest)
	openaiGroup.POST("/embeddings", h.HandleRequest)
	// 注意: GET /v1/models 不在openai包中注册，由server包统一分派
	// （OpenAI与Anthropic的模型列表格式不同，需要根据请求头判断协议）

	// 以下是更多可能需要的OpenAI API路径
	openaiGroup.GET("/models/:model", h.HandleRequest)
	openaiGroup.POST("/images/generations", h.HandleRequest)
	openaiGroup.POST("/audio/transcriptions", h.HandleRequest)
	openaiGroup.POST("/audio/translations", h.HandleRequest)
	openaiGroup.POST("/fine-tuning/jobs", h.HandleRequest)
	openaiGroup.GET("/fine-tuning/jobs", h.HandleRequest)
	openaiGroup.GET("/fine-tuning/jobs/:job_id", h.HandleRequest)
}

// HandleRequest 处理OpenAI API请求
// 根据配置，将请求代理到真实API或返回模拟响应
// 参数:
//   - c: Gin上下文
func (h *Handler) HandleRequest(c *gin.Context) {
	// 获取请求方法和路径
	method := c.Request.Method
	path := c.Request.URL.Path

	// 处理路径，移除前缀"/v1"（服务层已兼容带/不带/v1前缀）
	if strings.HasPrefix(path, "/v1") {
		path = path[3:]
	}

	// 读取请求体
	var bodyBytes []byte
	var err error
	if c.Request.Body != nil {
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": map[string]interface{}{
					"message": "无法读取请求体: " + err.Error(),
					"type":    "server_error",
					"code":    "internal_server_error",
				},
			})
			return
		}
		// 恢复请求体，以便其他中间件可以再次读取
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// 收集请求头
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// 收集查询参数
	queryParams := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// 使用OpenAI服务处理请求
	statusCode, responseHeaders, responseData, err := h.openaiService.HandleRequest(
		method, path, headers, queryParams, bodyBytes,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("处理请求失败: %v", err),
		})
		return
	}

	// 处理响应数据
	var responseBody interface{}
	if respBodyBytes, ok := responseData.([]byte); ok {
		// 如果是字节数组，尝试解析JSON
		if err := json.Unmarshal(respBodyBytes, &responseBody); err != nil {
			// 解析失败则作为字符串处理
			responseBody = string(respBodyBytes)
		}
	} else {
		// 否则直接使用
		responseBody = responseData
	}

	// 注意：请求/响应记录由 apiMiddleware 统一保存（SetupRoutes 中已应用该中间件），
	// 这里不再重复保存，避免每条请求产生两条记录

	// 设置响应头
	for key, value := range responseHeaders {
		c.Header(key, value)
	}

	// 返回响应
	c.JSON(statusCode, responseBody)
}

// SaveRequest 保存请求记录到存储（支持用户隔离）
func (h *Handler) SaveRequest(userID int64, request *storage.Request) error {
	if h.storage == nil {
		return fmt.Errorf("存储未初始化")
	}

	// 确保请求记录包含用户ID
	request.UserID = userID

	// 使用新的存储接口保存请求
	return h.storage.SaveRequest(userID, request)
}

// getUserFromContext 从gin.Context中获取用户信息
// 如果无法获取用户信息，返回默认用户ID（用于向后兼容）
func (h *Handler) getUserFromContext(c *gin.Context) (int64, string) {
	// 尝试从上下文中获取用户信息
	if userID, exists := c.Get("user_id"); exists {
		if username, exists := c.Get("username"); exists {
			return userID.(int64), username.(string)
		}
	}

	// 如果没有用户上下文，检查是否有Authorization头
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// 简单的token解析，实际应用中需要更严格的验证
		if strings.HasPrefix(authHeader, "Bearer ") {
			// 为了向后兼容，暂时使用默认用户
			return 0, "api_user"
		}
	}

	// 默认情况：使用匿名用户（ID为0，username为api_user）
	return 0, "api_user"
}

// HandleOpenAIRequest 处理OpenAI API请求的通用处理器
func (h *Handler) HandleOpenAIRequest(c *gin.Context) {
	// 记录开始时间，用于计算延迟
	startTime := time.Now()

	// 获取用户信息
	userID, username := h.getUserFromContext(c)

	// 获取请求信息
	method := c.Request.Method
	path := c.Request.URL.Path

	// 获取查询参数
	queryParams := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// 获取请求头
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// 读取请求体
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		// 重新创建一个可读的Body
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	// 解析请求体为JSON（如果可能）
	var bodyJSON interface{}
	if len(bodyBytes) > 0 {
		json.Unmarshal(bodyBytes, &bodyJSON)
	}

	// 添加用户标识到元数据
	metadata := map[string]interface{}{
		"source":       "openai_handler",
		"user_id":      userID,
		"username":     username,
		"request_time": startTime.Format(time.RFC3339),
	}

	// 使用服务处理请求
	statusCode, responseHeaders, responseBody, err := h.openaiService.HandleRequest(
		method, path, headers, queryParams, bodyBytes,
	)

	// 计算处理延迟
	latency := time.Since(startTime).Milliseconds()
	metadata["latency_ms"] = latency

	if err != nil {
		// 处理失败的情况
		metadata["error"] = err.Error()

		// 创建错误响应
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("处理请求失败: %v", err),
				"type":    "internal_error",
				"code":    "processing_failed",
			},
		}

		// 保存失败的请求记录
		request := &storage.Request{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    method,
			Path:      path,
			Timestamp: time.Now(),
			Headers:   headers,
			Query:     queryParams,
			Body:      bodyJSON,
			ClientIP:  utils.GetClientIP(c.Request),
			Response: &storage.ProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       errorResponse,
				Latency:    latency,
			},
			Metadata: metadata,
		}

		h.SaveRequest(userID, request)

		c.JSON(http.StatusInternalServerError, errorResponse)
		return
	}

	// 处理成功的情况
	response := &storage.ProxyResponse{
		StatusCode: statusCode,
		Headers:    responseHeaders,
		Body:       responseBody,
		Latency:    latency,
	}

	// 保存成功的请求和响应
	request := &storage.Request{
		ID:        uuid.New().String(),
		UserID:    userID,
		Method:    method,
		Path:      path,
		Timestamp: time.Now(),
		Headers:   headers,
		Query:     queryParams,
		Body:      bodyJSON,
		ClientIP:  utils.GetClientIP(c.Request),
		Response:  response,
		Metadata:  metadata,
	}

	h.SaveRequest(userID, request)

	// 设置响应头
	for key, value := range responseHeaders {
		c.Header(key, value)
	}

	// 返回响应
	c.JSON(statusCode, responseBody)
}

// HandleChatCompletions 处理聊天完成请求
func (h *Handler) HandleChatCompletions(c *gin.Context) {
	// 获取用户信息
	userID, username := h.getUserFromContext(c)

	// 读取请求体
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	// 解析聊天请求
	var chatRequest ChatCompletionRequest
	if err := json.Unmarshal(bodyBytes, &chatRequest); err != nil {
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid JSON in request body",
				"type":    "invalid_request_error",
				"code":    "json_invalid",
			},
		}

		// 保存错误请求记录
		request := &storage.Request{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    "POST",
			Path:      "/v1/chat/completions",
			Timestamp: time.Now(),
			Headers:   map[string]string{"Content-Type": "application/json"},
			Query:     map[string]string{},
			Body:      string(bodyBytes),
			ClientIP:  utils.GetClientIP(c.Request),
			Metadata: map[string]interface{}{
				"source":    "chat_completions_handler",
				"user_id":   userID,
				"username":  username,
				"error":     "invalid_json",
				"error_msg": err.Error(),
			},
		}

		h.SaveRequest(userID, request)

		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// 验证必要字段
	if len(chatRequest.Messages) == 0 {
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "At least one message is required",
				"type":    "invalid_request_error",
				"code":    "missing_messages",
			},
		}

		// 保存错误请求记录
		request := &storage.Request{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    "POST",
			Path:      "/v1/chat/completions",
			Timestamp: time.Now(),
			Headers:   map[string]string{"Content-Type": "application/json"},
			Query:     map[string]string{},
			Body:      chatRequest,
			ClientIP:  utils.GetClientIP(c.Request),
			Metadata: map[string]interface{}{
				"source":   "chat_completions_handler",
				"user_id":  userID,
				"username": username,
				"error":    "missing_messages",
			},
		}

		h.SaveRequest(userID, request)

		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// 委托给通用处理器
	h.HandleOpenAIRequest(c)
}

// HandleEmbeddings 处理嵌入请求
func (h *Handler) HandleEmbeddings(c *gin.Context) {
	// 获取用户信息
	userID, username := h.getUserFromContext(c)

	// 读取请求体
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	// 解析嵌入请求
	var embeddingRequest EmbeddingRequest
	if err := json.Unmarshal(bodyBytes, &embeddingRequest); err != nil {
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid JSON in request body",
				"type":    "invalid_request_error",
				"code":    "json_invalid",
			},
		}

		// 保存错误请求记录
		request := &storage.Request{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    "POST",
			Path:      "/v1/embeddings",
			Timestamp: time.Now(),
			Headers:   map[string]string{"Content-Type": "application/json"},
			Query:     map[string]string{},
			Body:      string(bodyBytes),
			ClientIP:  utils.GetClientIP(c.Request),
			Metadata: map[string]interface{}{
				"source":    "embeddings_handler",
				"user_id":   userID,
				"username":  username,
				"error":     "invalid_json",
				"error_msg": err.Error(),
			},
		}

		h.SaveRequest(userID, request)

		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// 验证必要字段
	if embeddingRequest.Input == nil || embeddingRequest.Input == "" {
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Input text is required",
				"type":    "invalid_request_error",
				"code":    "missing_input",
			},
		}

		// 保存错误请求记录
		request := &storage.Request{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    "POST",
			Path:      "/v1/embeddings",
			Timestamp: time.Now(),
			Headers:   map[string]string{"Content-Type": "application/json"},
			Query:     map[string]string{},
			Body:      embeddingRequest,
			ClientIP:  utils.GetClientIP(c.Request),
			Metadata: map[string]interface{}{
				"source":   "embeddings_handler",
				"user_id":  userID,
				"username": username,
				"error":    "missing_input",
			},
		}

		h.SaveRequest(userID, request)

		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// 委托给通用处理器
	h.HandleOpenAIRequest(c)
}

// HandleCompletions 处理文本完成请求
func (h *Handler) HandleCompletions(c *gin.Context) {
	// 获取用户信息
	userID, username := h.getUserFromContext(c)

	// 读取请求体
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	// 解析完成请求
	var completionRequest CompletionRequest
	if err := json.Unmarshal(bodyBytes, &completionRequest); err != nil {
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid JSON in request body",
				"type":    "invalid_request_error",
				"code":    "json_invalid",
			},
		}

		// 保存错误请求记录
		request := &storage.Request{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    "POST",
			Path:      "/v1/completions",
			Timestamp: time.Now(),
			Headers:   map[string]string{"Content-Type": "application/json"},
			Query:     map[string]string{},
			Body:      string(bodyBytes),
			ClientIP:  utils.GetClientIP(c.Request),
			Metadata: map[string]interface{}{
				"source":    "completions_handler",
				"user_id":   userID,
				"username":  username,
				"error":     "invalid_json",
				"error_msg": err.Error(),
			},
		}

		h.SaveRequest(userID, request)

		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// 委托给通用处理器
	h.HandleOpenAIRequest(c)
}

// HandleModels 处理模型列表请求
func (h *Handler) HandleModels(c *gin.Context) {
	// 获取用户信息
	userID, username := h.getUserFromContext(c)

	// 模型列表请求通常没有请求体，直接委托给通用处理器
	// 添加用户信息到上下文中
	c.Set("user_id", userID)
	c.Set("username", username)

	h.HandleOpenAIRequest(c)
}

// HandleModelDetails 处理单个模型详情请求
func (h *Handler) HandleModelDetails(c *gin.Context) {
	// 获取用户信息
	userID, username := h.getUserFromContext(c)

	// 模型详情请求通常没有请求体，直接委托给通用处理器
	// 添加用户信息到上下文中
	c.Set("user_id", userID)
	c.Set("username", username)

	h.HandleOpenAIRequest(c)
}

// 添加请求和响应的数据结构定义

// ChatCompletionRequest 聊天完成请求结构
type ChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []ChatMessage          `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	N                *int                   `json:"n,omitempty"`
	Stream           *bool                  `json:"stream,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]interface{} `json:"logit_bias,omitempty"`
	User             *string                `json:"user,omitempty"`
}

// ChatMessage 聊天消息结构
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// EmbeddingRequest 嵌入请求结构
type EmbeddingRequest struct {
	Input          interface{} `json:"input"`
	Model          string      `json:"model"`
	EncodingFormat *string     `json:"encoding_format,omitempty"`
	User           *string     `json:"user,omitempty"`
}

// CompletionRequest 文本完成请求结构
type CompletionRequest struct {
	Model            string                 `json:"model"`
	Prompt           interface{}            `json:"prompt"`
	Suffix           *string                `json:"suffix,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	N                *int                   `json:"n,omitempty"`
	Stream           *bool                  `json:"stream,omitempty"`
	Logprobs         *int                   `json:"logprobs,omitempty"`
	Echo             *bool                  `json:"echo,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	BestOf           *int                   `json:"best_of,omitempty"`
	LogitBias        map[string]interface{} `json:"logit_bias,omitempty"`
	User             *string                `json:"user,omitempty"`
}
