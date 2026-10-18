package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// proxyService 实现代理Anthropic（Claude）服务
type proxyService struct {
	config       Config
	httpClient   *http.Client
	supportedAPI map[string]bool
}

// 初始化函数，注册代理服务创建器
func init() {
	ProxyServiceCreator = newProxyService
}

// newProxyService 创建代理Anthropic服务
func newProxyService(config Config) Service {
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 5 * time.Minute, // 流式响应需要较长的超时时间
	}

	// 初始化支持的API
	supportedAPI := map[string]bool{
		"POST /messages":              true,
		"POST /messages/count_tokens": true,
		"GET /models":                 true,
	}

	return &proxyService{
		config:       config,
		httpClient:   client,
		supportedAPI: supportedAPI,
	}
}

// Name 返回服务名称
func (s *proxyService) Name() string {
	return "Anthropic API Proxy"
}

// HandleRequest 处理API请求并代理转发
func (s *proxyService) HandleRequest(method, path string, headers, queryParams map[string]string, body []byte) (int, map[string]string, interface{}, error) {
	// 检查目标URL是否配置
	if s.config.TargetURL == "" {
		return http.StatusBadRequest, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "configuration_error",
				Message: "未配置目标Anthropic API URL",
			},
		}, nil
	}

	// 模拟延迟（如果配置了延迟）
	if s.config.ResponseDelayMs > 0 {
		time.Sleep(time.Duration(s.config.ResponseDelayMs) * time.Millisecond)
	}

	// 标准化路径，确保以/开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 检查API是否支持
	apiKey := strings.ToUpper(method) + " " + path
	if !s.isAPISupported(apiKey) {
		return http.StatusNotFound, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "invalid_request_error",
				Message: "不支持的API路径: " + path,
			},
		}, nil
	}

	// 构建目标URL - Anthropic真实API要求 /v1 前缀
	upstreamPath := path
	if upstreamPath == "" || upstreamPath == "/" {
		upstreamPath = "/v1"
	} else if upstreamPath != "/v1" && !strings.HasPrefix(upstreamPath, "/v1/") {
		upstreamPath = "/v1" + upstreamPath
	}
	targetURL := s.buildURL(upstreamPath)

	// 添加查询参数
	if len(queryParams) > 0 {
		queryStrings := make([]string, 0, len(queryParams))
		for k, v := range queryParams {
			queryStrings = append(queryStrings, fmt.Sprintf("%s=%s", k, v))
		}
		targetURL += "?" + strings.Join(queryStrings, "&")
	}

	// 创建请求
	req, err := http.NewRequest(method, targetURL, bytes.NewBuffer(body))
	if err != nil {
		return http.StatusInternalServerError, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "proxy_error",
				Message: "创建代理请求失败: " + err.Error(),
			},
		}, err
	}

	// 添加请求头
	for k, v := range headers {
		// 跳过某些特殊的头，这些头由HTTP客户端自动处理
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Host") {
			continue
		}
		req.Header.Set(k, v)
	}

	// 确保Content-Type
	if len(body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 确保 anthropic-version 头存在（Claude Code SDK 通常会自己带上，
	// 如果没带则使用配置的默认版本）
	if req.Header.Get("anthropic-version") == "" {
		version := s.config.AnthropicVersion
		if version == "" {
			version = DefaultAnthropicVersion
		}
		req.Header.Set("anthropic-version", version)
	}

	// 添加认证
	s.addAuthHeader(req)

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return http.StatusBadGateway, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "proxy_error",
				Message: "代理请求失败: " + err.Error(),
			},
		}, err
	}

	// 提取响应头
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	// 判断是否为流式响应（SSE），如果是则返回io.Reader透传
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return resp.StatusCode, respHeaders, resp.Body, nil
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusBadGateway, jsonHeaders(), ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "proxy_error",
				Message: "读取响应失败: " + err.Error(),
			},
		}, err
	}

	// 解析响应体
	var parsedBody interface{}
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		if err := json.Unmarshal(respBody, &parsedBody); err != nil {
			// 如果无法解析JSON，则作为字符串返回
			parsedBody = string(respBody)
		}
	} else {
		// 非JSON响应，返回字符串
		parsedBody = string(respBody)
	}

	// 返回响应
	return resp.StatusCode, respHeaders, parsedBody, nil
}

// 检查API是否支持
// 兼容带/不带 /v1 前缀的路径
func (s *proxyService) isAPISupported(apiKey string) bool {
	// 兼容带/不带 /v1 前缀的路径（如 "POST /v1/messages" 与 "POST /messages"）
	if idx := strings.Index(apiKey, " /v1"); idx != -1 {
		apiKey = apiKey[:idx+1] + apiKey[idx+4:]
	}

	// 精确匹配
	if s.supportedAPI[apiKey] {
		return true
	}

	// 前缀匹配（适用于带参数的路径）
	for key := range s.supportedAPI {
		parts := strings.Split(key, " ")
		if len(parts) == 2 {
			method, path := parts[0], parts[1]
			if strings.HasPrefix(apiKey, method+" "+path+"/") {
				return true
			}
		}
	}

	return false
}

// 构建完整URL
func (s *proxyService) buildURL(path string) string {
	baseURL := s.config.TargetURL

	// 处理URL斜杠，确保URL拼接正确
	if !strings.HasSuffix(baseURL, "/") && !strings.HasPrefix(path, "/") {
		baseURL += "/"
	} else if strings.HasSuffix(baseURL, "/") && strings.HasPrefix(path, "/") {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return baseURL + path
}

// 添加认证头
// Anthropic API 支持两种认证方式:
//   - x-api-key: API密钥
//   - Authorization: Bearer token
func (s *proxyService) addAuthHeader(req *http.Request) {
	switch strings.ToLower(s.config.TargetAuthType) {
	case "api_key":
		if s.config.TargetAPIKey != "" {
			req.Header.Set("x-api-key", s.config.TargetAPIKey)
		}
	case "token", "bearer":
		if s.config.TargetToken != "" {
			token := s.config.TargetToken
			if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
				token = "Bearer " + token
			}
			req.Header.Set("Authorization", token)
		}
	}
}
