package anthropic

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// newTestMockService 创建默认测试配置的mock服务
func newTestMockService() Service {
	return MockServiceCreator(Config{
		Enabled:          true,
		ResponseDelayMs:  0,
		AnthropicVersion: DefaultAnthropicVersion,
	})
}

func TestMockMessagesNonStream(t *testing.T) {
	svc := newTestMockService()

	body, _ := json.Marshal(MessagesRequest{
		Model:     "claude-sonnet-5-5",
		MaxTokens: 256,
		Messages: []RequestMessage{
			{Role: "user", Content: "你好，测试一下"},
		},
	})

	status, headers, respData, err := svc.HandleRequest("POST", "/messages", nil, nil, body)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("期望状态码200，实际: %d", status)
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("期望Content-Type为application/json，实际: %s", headers["Content-Type"])
	}

	msgResp, ok := respData.(MessageResponse)
	if !ok {
		t.Fatalf("响应类型错误: %T", respData)
	}
	if msgResp.Type != "message" {
		t.Errorf("期望type为message，实际: %s", msgResp.Type)
	}
	if msgResp.Role != "assistant" {
		t.Errorf("期望role为assistant，实际: %s", msgResp.Role)
	}
	if msgResp.Model != "claude-sonnet-5-5" {
		t.Errorf("期望model为claude-sonnet-5-5，实际: %s", msgResp.Model)
	}
	if msgResp.StopReason != "end_turn" {
		t.Errorf("期望stop_reason为end_turn，实际: %s", msgResp.StopReason)
	}
	if len(msgResp.Content) != 1 || msgResp.Content[0].Type != "text" {
		t.Fatalf("期望一个text内容块，实际: %+v", msgResp.Content)
	}
	if !strings.Contains(msgResp.Content[0].Text, "你好") {
		t.Errorf("响应文本应包含用户消息内容，实际: %s", msgResp.Content[0].Text)
	}
	if msgResp.Usage.InputTokens <= 0 || msgResp.Usage.OutputTokens <= 0 {
		t.Errorf("usage不应为零: %+v", msgResp.Usage)
	}
}

func TestMockMessagesContentBlocks(t *testing.T) {
	svc := newTestMockService()

	// 测试 content 为 blocks 数组的情况（Claude Code 常用格式）
	body, _ := json.Marshal(MessagesRequest{
		Model: "claude-opus-5-5",
		Messages: []RequestMessage{
			{Role: "user", Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "第一段"},
				map[string]interface{}{"type": "text", "text": "第二段"},
			}},
		},
	})

	status, _, respData, err := svc.HandleRequest("POST", "/v1/messages", nil, nil, body)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("期望状态码200，实际: %d", status)
	}

	msgResp := respData.(MessageResponse)
	if !strings.Contains(msgResp.Content[0].Text, "第一段") {
		t.Errorf("响应应包含blocks中的文本，实际: %s", msgResp.Content[0].Text)
	}
}

func TestMockMessagesValidation(t *testing.T) {
	svc := newTestMockService()

	// 缺少model
	body, _ := json.Marshal(MessagesRequest{
		Messages: []RequestMessage{{Role: "user", Content: "hi"}},
	})
	status, _, respData, err := svc.HandleRequest("POST", "/messages", nil, nil, body)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusBadRequest {
		t.Fatalf("缺少model应返回400，实际: %d", status)
	}
	if errResp, ok := respData.(ErrorResponse); ok {
		if errResp.Type != "error" || errResp.Error.Type != "invalid_request_error" {
			t.Errorf("错误响应格式不正确: %+v", errResp)
		}
	} else {
		t.Fatalf("错误响应类型错误: %T", respData)
	}

	// 缺少messages
	body, _ = json.Marshal(MessagesRequest{Model: "claude-sonnet-5-5"})
	status, _, _, err = svc.HandleRequest("POST", "/messages", nil, nil, body)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusBadRequest {
		t.Fatalf("缺少messages应返回400，实际: %d", status)
	}

	// 无效JSON
	status, _, _, err = svc.HandleRequest("POST", "/messages", nil, nil, []byte("not-json"))
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusBadRequest {
		t.Fatalf("无效JSON应返回400，实际: %d", status)
	}
}

func TestMockMessagesStream(t *testing.T) {
	svc := newTestMockService()

	body, _ := json.Marshal(MessagesRequest{
		Model:     "claude-sonnet-5-5",
		MaxTokens: 256,
		Stream:    true,
		Messages:  []RequestMessage{{Role: "user", Content: "流式测试"}},
	})

	status, headers, respData, err := svc.HandleRequest("POST", "/messages", nil, nil, body)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("期望状态码200，实际: %d", status)
	}
	if !strings.Contains(headers["Content-Type"], "text/event-stream") {
		t.Errorf("流式响应Content-Type应为text/event-stream，实际: %s", headers["Content-Type"])
	}

	reader, ok := respData.(io.Reader)
	if !ok {
		t.Fatalf("流式响应应为io.Reader，实际: %T", respData)
	}

	// 读取完整SSE流并验证事件序列
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("读取流式响应失败: %v", err)
	}
	sseText := string(data)

	// 验证关键事件都在
	requiredEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	}
	for _, ev := range requiredEvents {
		if !strings.Contains(sseText, ev) {
			t.Errorf("流式响应缺少事件: %s\n完整响应:\n%s", ev, sseText)
		}
	}

	// 验证事件顺序
	idx := 0
	for _, ev := range requiredEvents {
		pos := strings.Index(sseText[idx:], ev)
		if pos < 0 {
			t.Errorf("事件顺序错误: 未找到 %s", ev)
			break
		}
		idx += pos + len(ev)
	}

	// 验证JSON解析
	lines := strings.Split(sseText, "\n")
	eventCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload); err != nil {
				t.Errorf("SSE data行无法解析为JSON: %v\n行: %s", err, line)
			}
			eventCount++
		}
	}
	if eventCount < 5 {
		t.Errorf("期望至少5个data事件，实际: %d", eventCount)
	}
}

func TestMockCountTokens(t *testing.T) {
	svc := newTestMockService()

	body, _ := json.Marshal(CountTokensRequest{
		Model:    "claude-sonnet-5-5",
		Messages: []RequestMessage{{Role: "user", Content: "hello world"}},
	})

	status, _, respData, err := svc.HandleRequest("POST", "/messages/count_tokens", nil, nil, body)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("期望状态码200，实际: %d", status)
	}

	countResp, ok := respData.(CountTokensResponse)
	if !ok {
		t.Fatalf("响应类型错误: %T", respData)
	}
	if countResp.InputTokens <= 0 {
		t.Errorf("期望input_tokens大于0，实际: %d", countResp.InputTokens)
	}
}

func TestMockListModels(t *testing.T) {
	svc := newTestMockService()

	status, _, respData, err := svc.HandleRequest("GET", "/models", nil, nil, nil)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("期望状态码200，实际: %d", status)
	}

	models, ok := respData.(ListModelsResponse)
	if !ok {
		t.Fatalf("响应类型错误: %T", respData)
	}
	if len(models.Data) == 0 {
		t.Fatal("模型列表不应为空")
	}
	// 验证Anthropic格式字段
	if models.Data[0].Type != "model" {
		t.Errorf("期望type为model，实际: %s", models.Data[0].Type)
	}
	if models.Data[0].ID == "" {
		t.Error("模型ID不应为空")
	}
	if models.Data[0].DisplayName == "" {
		t.Error("模型DisplayName不应为空")
	}
}

func TestMockRouteNotFound(t *testing.T) {
	svc := newTestMockService()

	status, _, respData, err := svc.HandleRequest("GET", "/models/nonexistent", nil, nil, nil)
	if err != nil {
		t.Fatalf("HandleRequest返回错误: %v", err)
	}
	if status != http.StatusNotFound {
		t.Fatalf("未知路径应返回404，实际: %d", status)
	}
	if _, ok := respData.(ErrorResponse); !ok {
		t.Fatalf("404响应应为ErrorResponse，实际: %T", respData)
	}
}

func TestExtractMessageText(t *testing.T) {
	// 字符串content
	if got := extractMessageText("hello"); got != "hello" {
		t.Errorf("字符串content提取失败: %s", got)
	}

	// blocks数组content
	blocks := []interface{}{
		map[string]interface{}{"type": "text", "text": "a"},
		map[string]interface{}{"type": "text", "text": "b"},
	}
	if got := extractMessageText(blocks); got != "ab" {
		t.Errorf("blocks数组content提取失败: %s", got)
	}

	// 非文本块应被跳过
	blocks = []interface{}{
		map[string]interface{}{"type": "tool_use", "id": "x", "name": "y", "input": map[string]interface{}{}},
		map[string]interface{}{"type": "text", "text": "only"},
	}
	if got := extractMessageText(blocks); got != "only" {
		t.Errorf("非文本块应被跳过: %s", got)
	}

	// 空/nil
	if got := extractMessageText(nil); got != "" {
		t.Errorf("nil content应返回空串: %s", got)
	}
}

func TestSplitTextIntoChunks(t *testing.T) {
	text := "这是一个比较长的测试文本，用于验证拆分逻辑"
	chunks := splitTextIntoChunks(text, 5)
	if len(chunks) < 2 {
		t.Fatalf("期望至少2个chunk，实际: %d", len(chunks))
	}
	// 拼接回去应等于原文
	joined := strings.Join(chunks, "")
	if joined != text {
		t.Errorf("chunks拼接后与原文本不一致: %s != %s", joined, text)
	}
}

func TestLoadModelsFromFileStrings(t *testing.T) {
	// 字符串数组格式（OpenAI风格）
	models, err := loadModelsFromFileString(`["claude-sonnet-5-5","claude-opus-5-5"]`)
	if err != nil {
		t.Fatalf("加载字符串数组模型失败: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("期望2个模型，实际: %d", len(models))
	}
	if models[0].Type != "model" {
		t.Errorf("模型type应为model，实际: %s", models[0].Type)
	}
}

// loadModelsFromFileString 内部辅助：从字符串加载模型列表（测试用）
func loadModelsFromFileString(content string) ([]ModelInfo, error) {
	return parseModelsContent([]byte(content))
}

func TestLoadModelsFromFileObjects(t *testing.T) {
	// 对象数组格式（Anthropic风格）
	content := `[
		{"type":"model","id":"claude-sonnet-5-5","display_name":"Claude Sonnet 5.5","created_at":"2024-01-01T00:00:00Z","lifecycle":"active"}
	]`
	models, err := loadModelsFromFileString(content)
	if err != nil {
		t.Fatalf("加载对象数组模型失败: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("期望1个模型，实际: %d", len(models))
	}
	if models[0].ID != "claude-sonnet-5-5" {
		t.Errorf("模型ID错误: %s", models[0].ID)
	}
}
