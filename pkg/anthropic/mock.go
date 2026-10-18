package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// mockService 实现模拟Anthropic（Claude）服务
type mockService struct {
	responseDelay    time.Duration
	handlerMapping   map[string]mockHandler // 路径->处理函数的映射
	supportedModels  []ModelInfo            // 支持的模型列表
	anthropicVersion string                 // 使用的API版本
}

// mockHandler 定义模拟处理函数类型
// 返回 (状态码, 响应头, 响应体)
// 对于流式响应，响应体会是 io.Reader
type mockHandler func(method string, pathParams, queryParams map[string]string, body []byte, headers map[string]string) (int, map[string]string, interface{})

// 初始化函数，赋值给接口中定义的变量
func init() {
	MockServiceCreator = newMockService
}

// newMockService 创建模拟Anthropic服务
func newMockService(config Config) Service {
	var models []ModelInfo
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

	version := config.AnthropicVersion
	if version == "" {
		version = DefaultAnthropicVersion
	}

	svc := &mockService{
		responseDelay:    delay,
		handlerMapping:   make(map[string]mockHandler),
		supportedModels:  models,
		anthropicVersion: version,
	}

	// 注册路径处理函数
	svc.registerHandlers()

	return svc
}

// loadModelsFromFile 从文件加载模型列表
// 文件格式与OpenAI的models文件兼容（字符串数组），
// 也支持Anthropic格式的模型对象数组
func loadModelsFromFile(filePath string) ([]ModelInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return parseModelsContent(data)
}

// parseModelsContent 解析模型列表内容
// 支持字符串数组（OpenAI风格）和模型对象数组（Anthropic风格）两种格式
func parseModelsContent(data []byte) ([]ModelInfo, error) {
	// 先尝试解析为字符串数组（OpenAI风格）
	var ids []string
	if err := json.Unmarshal(data, &ids); err == nil {
		models := make([]ModelInfo, 0, len(ids))
		for _, id := range ids {
			models = append(models, modelInfoFromID(id))
		}
		return models, nil
	}

	// 再尝试解析为模型对象数组（Anthropic风格）
	var models []ModelInfo
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}
	return models, nil
}

// modelInfoFromID 根据模型ID生成ModelInfo
func modelInfoFromID(id string) ModelInfo {
	displayName := id
	for _, m := range getDefaultModels() {
		if m.ID == id {
			displayName = m.DisplayName
			break
		}
	}
	return ModelInfo{
		Type:        "model",
		ID:          id,
		DisplayName: displayName,
		CreatedAt:   "2024-01-01T00:00:00Z",
		Lifecycle:   "active",
		Capabilities: map[string]interface{}{
			"vision":   true,
			"tool_use": true,
			"thinking": true,
		},
	}
}

// getDefaultModels 返回默认支持的模型列表（Anthropic风格）
func getDefaultModels() []ModelInfo {
	now := "2024-01-01T00:00:00Z"
	return []ModelInfo{
		{
			Type: "model", ID: "claude-opus-5-5", DisplayName: "Claude Opus 5.5",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
		{
			Type: "model", ID: "claude-opus-5-1", DisplayName: "Claude Opus 5.1",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
		{
			Type: "model", ID: "claude-sonnet-5-5", DisplayName: "Claude Sonnet 5.5",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
		{
			Type: "model", ID: "claude-sonnet-5-1", DisplayName: "Claude Sonnet 5.1",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
		{
			Type: "model", ID: "claude-haiku-5-5", DisplayName: "Claude Haiku 5.5",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
		{
			Type: "model", ID: "claude-haiku-5-1", DisplayName: "Claude Haiku 5.1",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
		{
			Type: "model", ID: "claude-fable-5-1", DisplayName: "Claude Fable 5.1",
			CreatedAt: now, Lifecycle: "active",
			MaxInputTokens: 200000, MaxOutputTokens: 64000,
			Capabilities: map[string]interface{}{"vision": true, "tool_use": true, "thinking": true},
		},
	}
}

// Name 返回服务名称
func (s *mockService) Name() string {
	return "MockAnthropicService"
}

// HandleRequest 处理API请求并返回模拟响应
func (s *mockService) HandleRequest(method, path string, headers, queryParams map[string]string, body []byte) (int, map[string]string, interface{}, error) {
	// 模拟延迟
	if s.responseDelay > 0 {
		time.Sleep(s.responseDelay)
	}

	// 提取路径处理函数
	handler, pathParams := s.matchPath(method, path)
	if handler == nil {
		// 如果没有找到处理函数，返回404
		return http.StatusNotFound, map[string]string{"Content-Type": "application/json"}, ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "invalid_request_error",
				Message: "未找到API路径: " + path,
			},
		}, nil
	}

	// 调用处理函数
	statusCode, respHeaders, respBody := handler(method, pathParams, queryParams, body, headers)

	// 返回响应
	return statusCode, respHeaders, respBody, nil
}

// matchPath 匹配路径并返回对应的处理函数和路径参数
// 与openai.mockService.matchPath逻辑一致
func (s *mockService) matchPath(method, path string) (mockHandler, map[string]string) {
	// 标准化路径，确保以/开头，但不以/结尾
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	// 兼容带 /v1 前缀的路径
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
// 注册时不带/v1前缀，matchPath中已经做了/v1前缀兼容处理
func (s *mockService) registerHandlers() {
	s.handlerMapping["POST /messages"] = s.handleMessages
	s.handlerMapping["POST /messages/count_tokens"] = s.handleCountTokens
	s.handlerMapping["GET /models"] = s.handleListModels
}

// handleMessages 处理 Messages API
func (s *mockService) handleMessages(method string, pathParams, queryParams map[string]string, body []byte, headers map[string]string) (int, map[string]string, interface{}) {
	var req MessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "invalid_request_error",
				Message: "无效的请求体: " + err.Error(),
			},
		}
	}

	// 验证模型
	if req.Model == "" {
		return http.StatusBadRequest, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "invalid_request_error",
				Message: "必须指定模型",
			},
		}
	}

	// 验证消息
	if len(req.Messages) == 0 {
		return http.StatusBadRequest, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "invalid_request_error",
				Message: "必须提供至少一条消息",
			},
		}
	}

	// 生成模拟响应文本
	responseText := s.buildResponseText(req)

	// 根据是否流式返回不同格式
	if req.Stream {
		reader := s.streamMessageResponse(req.Model, responseText)
		return http.StatusOK, map[string]string{
			"Content-Type":  "text/event-stream",
			"Cache-Control": "no-cache",
			"Connection":    "keep-alive",
		}, reader
	}

	// 非流式响应
	return http.StatusOK, jsonHeaders(), MessageResponse{
		ID:           "msg_" + uuid.New().String(),
		Type:         "message",
		Role:         "assistant",
		Model:        req.Model,
		Content:      []ContentBlock{{Type: "text", Text: responseText}},
		StopReason:   "end_turn",
		StopSequence: nil,
		Usage:        Usage{InputTokens: 100, OutputTokens: 30},
	}
}

// buildResponseText 根据请求生成模拟响应文本
// 提取最后一条user消息的文本内容（支持string和blocks数组两种content格式）
func (s *mockService) buildResponseText(req MessagesRequest) string {
	lastMsg := req.Messages[len(req.Messages)-1]

	// 提取用户消息文本
	userText := extractMessageText(lastMsg.Content)

	if userText != "" {
		return "您的消息是: \"" + userText + "\"\n\n这是一个模拟的Claude响应。"
	}
	return "这是一个模拟的Claude响应。"
}

// extractMessageText 从消息content中提取纯文本
// content可以是字符串，也可以是内容块数组（[{type:"text",text:"..."},...]）
func extractMessageText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var sb strings.Builder
		for _, block := range v {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, _ := blockMap["type"].(string); blockType == "text" {
					if text, ok := blockMap["text"].(string); ok {
						sb.WriteString(text)
					}
				}
			}
		}
		return sb.String()
	case []ContentBlock:
		var sb strings.Builder
		for _, block := range v {
			if block.Type == "text" {
				sb.WriteString(block.Text)
			}
		}
		return sb.String()
	default:
		return ""
	}
}

// streamMessageResponse 生成Messages API的SSE流式响应
// 事件顺序:
//
//	message_start -> content_block_start -> content_block_delta(多次) -> content_block_stop
//	-> message_delta -> message_stop
func (s *mockService) streamMessageResponse(model, text string) io.Reader {
	messageID := "msg_" + uuid.New().String()

	// 将文本拆分为若干小块，模拟流式输出
	chunks := splitTextIntoChunks(text, 8)

	// 构建完整的事件序列
	var buf bytes.Buffer

	// message_start: 完整的message对象，stop_reason为null
	startMessage := MessageResponse{
		ID:           messageID,
		Type:         "message",
		Role:         "assistant",
		Model:        model,
		Content:      []ContentBlock{{Type: "text", Text: ""}},
		StopReason:   "",
		StopSequence: nil,
		Usage:        Usage{InputTokens: 100, OutputTokens: 0},
	}
	startJSON, _ := json.Marshal(startMessage)
	writeSSEEvent(&buf, "message_start", map[string]interface{}{
		"type":    "message_start",
		"message": json.RawMessage(startJSON),
	})

	// content_block_start
	writeSSEEvent(&buf, "content_block_start", map[string]interface{}{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]interface{}{"type": "text", "text": ""},
	})

	// content_block_delta（多次，模拟token流式输出）
	outputTokens := 0
	for _, chunk := range chunks {
		writeSSEEvent(&buf, "content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{"type": "text_delta", "text": chunk},
		})
		outputTokens += len(strings.Fields(chunk)) + 2 // 粗略估算
	}
	if outputTokens == 0 {
		outputTokens = 30
	}

	// content_block_stop
	writeSSEEvent(&buf, "content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	})

	// message_delta: stop_reason和累计usage
	writeSSEEvent(&buf, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
		},
		"usage": map[string]interface{}{
			"input_tokens":  100,
			"output_tokens": outputTokens,
		},
	})

	// message_stop
	writeSSEEvent(&buf, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})

	return &eventStreamReader{reader: bufio.NewReader(bytes.NewReader(buf.Bytes()))}
}

// writeSSEEvent 将事件写入SSE缓冲区
// 格式: data: {json}\n\n （可选的 event: 行放在data之前）
func writeSSEEvent(buf *bytes.Buffer, eventName string, payload map[string]interface{}) {
	if eventName != "" {
		buf.WriteString("event: " + eventName + "\n")
	}
	jsonData, _ := json.Marshal(payload)
	buf.WriteString("data: ")
	buf.Write(jsonData)
	buf.WriteString("\n\n")
}

// splitTextIntoChunks 将文本拆分为小块，模拟流式输出
func splitTextIntoChunks(text string, chunkSize int) []string {
	if text == "" {
		return []string{""}
	}
	// 按中文字符和单词边界粗略拆分
	runes := []rune(text)
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

// eventStreamReader 包装io.Reader，在读取结束时发出EOF
// 实际数据已经完整写入buffer，这里直接透传即可
type eventStreamReader struct {
	reader io.Reader
}

func (r *eventStreamReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

// handleCountTokens 处理 token 计数 API
func (s *mockService) handleCountTokens(method string, pathParams, queryParams map[string]string, body []byte, headers map[string]string) (int, map[string]string, interface{}) {
	var req CountTokensRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "invalid_request_error",
				Message: "无效的请求体: " + err.Error(),
			},
		}
	}

	// 粗略估算输入token数：字符数/4（英文）或/2（中文）
	inputTokens := 0
	for _, msg := range req.Messages {
		inputTokens += estimateTokens(extractMessageText(msg.Content))
	}
	if inputTokens == 0 {
		inputTokens = 1 // 避免返回0
	}

	return http.StatusOK, jsonHeaders(), CountTokensResponse{InputTokens: inputTokens}
}

// estimateTokens 粗略估算token数
func estimateTokens(text string) int {
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}
	// 简单估算：中文字符1个token，英文单词平均1.3个token
	chineseChars := 0
	for _, r := range text {
		if r > 0x2E7F { // CJK字符范围
			chineseChars++
		}
	}
	englishPart := runes - chineseChars
	return chineseChars + englishPart/4 + 1
}

// handleListModels 处理模型列表API（Anthropic格式）
func (s *mockService) handleListModels(method string, pathParams, queryParams map[string]string, body []byte, headers map[string]string) (int, map[string]string, interface{}) {
	models := s.supportedModels
	if len(models) == 0 {
		models = getDefaultModels()
	}

	data := make([]ModelInfo, len(models))
	copy(data, models)

	firstID := ""
	lastID := ""
	if len(data) > 0 {
		firstID = data[0].ID
		lastID = data[len(data)-1].ID
	}

	return http.StatusOK, jsonHeaders(), ListModelsResponse{
		Data:    data,
		HasMore: false,
		FirstID: &firstID,
		LastID:  &lastID,
	}
}

// jsonHeaders 返回标准JSON响应头
func jsonHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/json"}
}
