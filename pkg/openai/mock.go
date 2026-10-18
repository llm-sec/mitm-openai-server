package openai

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockService 实现模拟OpenAI服务
type mockService struct {
	responseDelay   time.Duration
	handlerMapping  map[string]mockHandler // 路径->处理函数的映射
	supportedModels []string               // 支持的模型列表
}

// mockHandler 定义模拟处理函数类型
type mockHandler func(method string, pathParams map[string]string, queryParams map[string]string, body []byte) (int, interface{})

// 初始化函数，赋值给接口中定义的变量
func init() {
	MockServiceCreator = newMockService
}

// newMockService 创建模拟OpenAI服务
func newMockService(config Config) Service {
	var models []string
	if config.ModelsFile != "" {
		// 从文件加载模型列表
		if m, err := loadModelsFromFile(config.ModelsFile); err == nil {
			models = m
		} else {
			fmt.Printf("加载模型列表文件失败: %v，将使用默认模型\n", err)
			models = getDefaultModels()
		}
	} else {
		models = getDefaultModels()
	}

	delay := time.Duration(config.ResponseDelayMs) * time.Millisecond

	svc := &mockService{
		responseDelay:   delay,
		handlerMapping:  make(map[string]mockHandler),
		supportedModels: models,
	}

	// 注册路径处理函数
	svc.registerHandlers()

	return svc
}

// loadModelsFromFile 从文件加载模型列表
func loadModelsFromFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var models []string
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	return models, nil
}

// getDefaultModels 返回默认支持的模型列表
func getDefaultModels() []string {
	return []string{
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-16k",
		"gpt-4",
		"gpt-4-32k",
		"text-embedding-3-small",
		"text-embedding-3-large",
		"dall-e-3",
	}
}

// ServeOpenAISpec 提供OpenAI规范
func (s *mockService) ServeOpenAISpec(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"title":       "Fake OpenAI API",
		"description": "兼容OpenAI接口的模拟服务",
		"models":      s.supportedModels,
	})
}

// HandleRequest 处理API请求并返回模拟响应
func (s *mockService) HandleRequest(method, path string, headers, queryParams map[string]string, body []byte) (int, map[string]string, interface{}, error) {
	// 模拟延迟
	if s.responseDelay > 0 {
		time.Sleep(s.responseDelay)
	}

	// 提取路径参数
	handler, pathParams := s.matchPath(method, path)
	if handler == nil {
		// 如果没有找到处理函数，返回404
		return http.StatusNotFound, map[string]string{"Content-Type": "application/json"}, Response{
			Error: &ErrorResp{
				Message: "未找到API路径: " + path,
				Type:    "invalid_request_error",
				Code:    "resource_not_found",
			},
		}, nil
	}

	// 调用处理函数
	statusCode, respBody := handler(method, pathParams, queryParams, body)

	// 返回响应
	return statusCode, map[string]string{"Content-Type": "application/json"}, respBody, nil
}

// Name 返回服务名称
func (s *mockService) Name() string {
	return "MockOpenAIService"
}

// matchPath 匹配路径并返回对应的处理函数和路径参数
func (s *mockService) matchPath(method, path string) (mockHandler, map[string]string) {
	// 标准化路径，确保以/开头，但不以/结尾
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	// 兼容带 /v1 前缀的路径（测试和部分调用方可能直接传入 /v1/... 路径）
	if path == "/v1" || strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
		if path == "" {
			path = "/"
		}
	}

	// 首先尝试精确匹配
	key := strings.ToUpper(method) + " " + path
	if handler, ok := s.handlerMapping[key]; ok {
		return handler, make(map[string]string)
	}

	// 如果没有精确匹配，尝试匹配带参数的路径
	for pattern, handler := range s.handlerMapping {
		parts := strings.Split(pattern, " ")
		if len(parts) != 2 || parts[0] != strings.ToUpper(method) {
			continue
		}

		pathPattern := parts[1]
		if !strings.Contains(pathPattern, "{") {
			continue
		}

		// 转换路径模式为正则表达式
		regexPattern := regexp.QuoteMeta(pathPattern)
		regexPattern = regexp.MustCompile(`\{([^/]+)\}`).ReplaceAllString(regexPattern, `([^/]+)`)
		regexPattern = "^" + regexPattern + "$"

		re, err := regexp.Compile(regexPattern)
		if err != nil {
			continue
		}

		matches := re.FindStringSubmatch(path)
		if matches == nil {
			continue
		}

		// 提取路径参数
		params := make(map[string]string)
		paramNames := regexp.MustCompile(`\{([^/]+)\}`).FindAllStringSubmatch(pathPattern, -1)
		for i, name := range paramNames {
			if i+1 < len(matches) {
				params[name[1]] = matches[i+1]
			}
		}

		return handler, params
	}

	return nil, nil
}

// registerHandlers 注册路径处理函数
func (s *mockService) registerHandlers() {
	// OpenAI API 路径 - 注册时不带/v1前缀，matchPath中已经做了/v1前缀兼容处理
	s.handlerMapping["POST /chat/completions"] = s.handleChatCompletions
	s.handlerMapping["POST /completions"] = s.handleCompletions
	s.handlerMapping["POST /embeddings"] = s.handleEmbeddings
	s.handlerMapping["POST /images/generations"] = s.handleImagesGenerations
	s.handlerMapping["POST /audio/transcriptions"] = s.handleAudioTranscriptions
	s.handlerMapping["POST /moderations"] = s.handleModerations

	// 模型列表API
	s.handlerMapping["GET /models"] = s.handleListModels
}

// 处理函数实现

// handleChatCompletions 处理聊天补全API
func (s *mockService) handleChatCompletions(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	type ChatCompletionsRequest struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
		Stream      bool      `json:"stream"`
	}

	var req ChatCompletionsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "无效的请求体: " + err.Error(),
				Type:    "invalid_request_error",
			},
		}
	}

	// 验证模型
	if req.Model == "" {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须指定模型",
				Type:    "invalid_request_error",
				Param:   "model",
			},
		}
	}

	// 验证消息
	if len(req.Messages) == 0 {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须提供至少一条消息",
				Type:    "invalid_request_error",
				Param:   "messages",
			},
		}
	}

	// 模拟聊天补全响应
	responseText := "这是一个模拟的聊天补全响应。"

	// 如果最后一条消息是用户消息，我们可以基于它生成一个更加个性化的响应
	lastMsg := req.Messages[len(req.Messages)-1]
	if lastMsg.Role == "user" && lastMsg.Content != "" {
		responseText = "您的消息是: \"" + lastMsg.Content + "\"\n\n这是一个模拟的聊天响应。"
	}

	return http.StatusOK, Response{
		Object:  "chat.completion",
		ID:      "chatcmpl-" + uuid.New().String()[:8],
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: responseText,
				},
				FinishReason: "stop",
			},
		},
		Usage: &Usage{
			PromptTokens:     100,
			CompletionTokens: 30,
			TotalTokens:      130,
		},
	}
}

// handleCompletions 处理补全API（传统/Legacy）
func (s *mockService) handleCompletions(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	type CompletionsRequest struct {
		Model       string   `json:"model"`
		Prompt      string   `json:"prompt"`
		MaxTokens   int      `json:"max_tokens"`
		Temperature float64  `json:"temperature"`
		Stop        []string `json:"stop"`
	}

	var req CompletionsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "无效的请求体: " + err.Error(),
				Type:    "invalid_request_error",
			},
		}
	}

	// 验证模型和提示
	if req.Model == "" {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须指定模型",
				Type:    "invalid_request_error",
				Param:   "model",
			},
		}
	}

	// 模拟文本补全响应
	completionText := "这是对提示 \"" + req.Prompt + "\" 的模拟补全响应。"

	// 生成补全响应
	return http.StatusOK, Response{
		Object:  "text_completion",
		ID:      "cmpl-" + uuid.New().String()[:8],
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Content: completionText},
				FinishReason: "stop",
			},
		},
		Usage: &Usage{
			PromptTokens:     50,
			CompletionTokens: 20,
			TotalTokens:      70,
		},
	}
}

// handleEmbeddings 处理嵌入API
func (s *mockService) handleEmbeddings(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	type EmbeddingsRequest struct {
		Model string      `json:"model"`
		Input interface{} `json:"input"` // 可以是字符串或字符串数组
	}

	var req EmbeddingsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "无效的请求体: " + err.Error(),
				Type:    "invalid_request_error",
			},
		}
	}

	// 验证模型和输入
	if req.Model == "" {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须指定模型",
				Type:    "invalid_request_error",
				Param:   "model",
			},
		}
	}

	if req.Input == nil {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须提供输入文本",
				Type:    "invalid_request_error",
				Param:   "input",
			},
		}
	}

	// 处理输入，可能是字符串或字符串数组
	inputs := []string{}
	switch v := req.Input.(type) {
	case string:
		inputs = append(inputs, v)
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				inputs = append(inputs, str)
			}
		}
	default:
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "输入必须是字符串或字符串数组",
				Type:    "invalid_request_error",
				Param:   "input",
			},
		}
	}

	if len(inputs) == 0 {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "无有效输入文本",
				Type:    "invalid_request_error",
				Param:   "input",
			},
		}
	}

	// 创建嵌入响应
	type EmbeddingData struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	}

	rand.Seed(time.Now().UnixNano())
	embeddingData := []EmbeddingData{}

	for i, _ := range inputs {
		// 生成随机嵌入向量，长度为10的简化版本
		embedding := make([]float64, 10)
		for j := range embedding {
			embedding[j] = rand.Float64()*2 - 1 // 生成-1到1之间的随机数
		}

		embeddingData = append(embeddingData, EmbeddingData{
			Object:    "embedding",
			Embedding: embedding,
			Index:     i,
		})
	}

	totalTokens := 0
	for _, input := range inputs {
		totalTokens += len(strings.Split(input, " "))
	}

	return http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   embeddingData,
		"model":  req.Model,
		"usage": map[string]interface{}{
			"prompt_tokens": totalTokens,
			"total_tokens":  totalTokens,
		},
	}
}

// handleImagesGenerations 处理图片生成API
func (s *mockService) handleImagesGenerations(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	type ImagesRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		N      int    `json:"n"`
		Size   string `json:"size"`
	}

	var req ImagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "无效的请求体: " + err.Error(),
				Type:    "invalid_request_error",
			},
		}
	}

	// 验证提示词
	if req.Prompt == "" {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须提供提示词",
				Type:    "invalid_request_error",
				Param:   "prompt",
			},
		}
	}

	// 设置默认值
	if req.N <= 0 {
		req.N = 1
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}

	// 创建图片生成响应
	type ImageData struct {
		URL string `json:"url"`
	}

	images := []ImageData{}
	for i := 0; i < req.N; i++ {
		// 在真实实现中，这里会返回实际生成的图片URL
		// 这里我们返回一个模拟URL
		images = append(images, ImageData{
			URL: fmt.Sprintf("https://example.com/images/generated-%s-%d.png", uuid.New().String()[:8], i),
		})
	}

	return http.StatusOK, map[string]interface{}{
		"created": time.Now().Unix(),
		"data":    images,
	}
}

// handleAudioTranscriptions 处理音频转写API
func (s *mockService) handleAudioTranscriptions(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	// 注意：真实的音频转写API需要处理multipart/form-data
	// 这里我们简化处理，假设已经接收到了必要的参数

	return http.StatusOK, map[string]interface{}{
		"text": "这是一个模拟的音频转写结果。",
	}
}

// handleModerations 处理内容审核API
func (s *mockService) handleModerations(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	type ModerationRequest struct {
		Input string `json:"input"`
		Model string `json:"model"`
	}

	var req ModerationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "无效的请求体: " + err.Error(),
				Type:    "invalid_request_error",
			},
		}
	}

	// 验证输入
	if req.Input == "" {
		return http.StatusBadRequest, Response{
			Error: &ErrorResp{
				Message: "必须提供输入文本",
				Type:    "invalid_request_error",
				Param:   "input",
			},
		}
	}

	// 模拟审核结果
	return http.StatusOK, map[string]interface{}{
		"id":    "modr-" + uuid.New().String()[:8],
		"model": req.Model,
		"results": []map[string]interface{}{
			{
				"flagged": false,
				"categories": map[string]bool{
					"hate":             false,
					"hate/threatening": false,
					"self-harm":        false,
					"sexual":           false,
					"sexual/minors":    false,
					"violence":         false,
					"violence/graphic": false,
				},
				"category_scores": map[string]float64{
					"hate":             0.01,
					"hate/threatening": 0.01,
					"self-harm":        0.01,
					"sexual":           0.01,
					"sexual/minors":    0.01,
					"violence":         0.01,
					"violence/graphic": 0.01,
				},
			},
		},
	}
}

// handleListModels 处理模型列表API
func (s *mockService) handleListModels(method string, pathParams, queryParams map[string]string, body []byte) (int, interface{}) {
	type Model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	models := []Model{}
	now := time.Now().Unix()

	for _, model := range s.supportedModels {
		models = append(models, Model{
			ID:      model,
			Object:  "model",
			Created: now,
			OwnedBy: "organization-owner",
		})
	}

	return http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   models,
	}
}
