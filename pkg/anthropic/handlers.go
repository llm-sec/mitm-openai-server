package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
)

// Handler 处理Anthropic API请求的处理器
type Handler struct {
	storage          storage.Storage
	anthropicService Service
}

// NewHandler 创建一个新的Anthropic处理器
func NewHandler(storage storage.Storage, anthropicService Service) *Handler {
	return &Handler{
		storage:          storage,
		anthropicService: anthropicService,
	}
}

// SetupRoutes 设置Anthropic API路由
// 注意: /v1/models 由 server 包统一注册（区分OpenAI/Anthropic格式），不在这里注册
func (h *Handler) SetupRoutes(router *gin.Engine, apiMiddleware gin.HandlerFunc) {
	// 如果没有初始化anthropicService，跳过设置Anthropic路由
	if h.anthropicService == nil {
		fmt.Println("警告: anthropicService未初始化，跳过Anthropic路由设置")
		return
	}

	// Anthropic API路由组
	anthropicGroup := router.Group("/v1")

	// 应用API中间件记录请求
	anthropicGroup.Use(apiMiddleware)

	// 使用更宽松的CORS设置，确保跨域请求能正常工作
	anthropicGroup.Use(func(c *gin.Context) {
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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization, X-API-Key, X-Requested-With, Anthropic-Version")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24小时

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 注册Anthropic API路径
	anthropicGroup.POST("/messages", h.HandleRequest)
	anthropicGroup.POST("/messages/count_tokens", h.HandleRequest)
}

// HandleRequest 处理Anthropic API请求
// 根据配置，将请求代理到真实API或返回模拟响应
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Type: "error",
				Error: ErrorDetail{
					Type:    "server_error",
					Message: "无法读取请求体: " + err.Error(),
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

	// 使用Anthropic服务处理请求
	statusCode, responseHeaders, responseData, err := h.anthropicService.HandleRequest(
		method, path, headers, queryParams, bodyBytes,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Type: "error",
			Error: ErrorDetail{
				Type:    "server_error",
				Message: fmt.Sprintf("处理请求失败: %v", err),
			},
		})
		return
	}

	// 设置响应头
	for key, value := range responseHeaders {
		c.Header(key, value)
	}

	// 处理流式响应：responseData 为 io.Reader 时直接透传
	if reader, ok := responseData.(io.Reader); ok {
		c.Status(statusCode)
		if closer, ok := reader.(io.Closer); ok {
			defer closer.Close()
		}
		io.Copy(c.Writer, reader)
		c.Writer.Flush()
		return
	}

	// 处理普通响应
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

	// 返回响应
	c.JSON(statusCode, responseBody)
}
