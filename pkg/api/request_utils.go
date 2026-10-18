package api

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
)

// SaveRequest 保存请求到存储
// 将API请求模型转换为存储请求模型并保存
// 参数:
//   - userID: 用户ID
//   - req: 要保存的请求
//   - storage: 存储接口
//
// 返回:
//   - error: 如果保存失败，返回错误
func SaveRequest(userID int64, req *Request, store storage.Storage) error {
	// 解析时间戳
	var timestamp time.Time
	if req.Timestamp != "" {
		var err error
		timestamp, err = time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			// 如果解析失败，使用当前时间
			timestamp = time.Now()
		}
	} else {
		timestamp = time.Now()
	}

	// 转换API请求模型为存储请求模型
	storageReq := &storage.Request{
		ID:        req.ID,
		UserID:    userID,
		Method:    req.Method,
		Path:      req.Path,
		Timestamp: timestamp,
		ClientIP:  req.IPAddress,
		IPAddress: req.IPAddress,
		Headers:   req.Headers,
		Query:     req.Query,
		Body:      req.Body,
		Metadata:  req.Metadata,
	}

	// 如果有响应，也转换响应
	if req.Response != nil {
		storageReq.Response = &storage.ProxyResponse{
			StatusCode: req.Response.StatusCode,
			Headers:    req.Response.Headers,
			Body:       req.Response.Body,
			Latency:    req.Response.Latency,
		}
	}

	// 保存到存储
	return store.SaveRequest(userID, storageReq)
}

// ConvertStorageToAPIRequest 将存储请求模型转换为API请求模型
// 参数:
//   - req: 存储请求模型
//
// 返回:
//   - *Request: API请求模型
func ConvertStorageToAPIRequest(req *storage.Request) *Request {
	// 创建API请求模型
	apiReq := &Request{
		ID:     req.ID,
		Method: req.Method,
		Path:   req.Path,
		Body:   req.Body,
	}

	// 处理时间戳
	apiReq.Timestamp = req.Timestamp.Format(time.RFC3339)

	// 处理IP地址（兼容两个字段）
	if req.ClientIP != "" {
		apiReq.IPAddress = req.ClientIP
	} else {
		apiReq.IPAddress = req.IPAddress
	}

	// 转换Headers - 现在Headers已经是map[string]string
	apiReq.Headers = req.Headers

	// 转换Query - 现在Query已经是map[string]string
	apiReq.Query = req.Query

	// 转换Metadata（如TLS ClientHello指纹等，供前端展示）
	apiReq.Metadata = req.Metadata

	// 转换Response（如果存在）
	if req.Response != nil {
		apiReq.Response = &ProxyResponse{
			StatusCode: req.Response.StatusCode,
			Headers:    req.Response.Headers,
			Body:       req.Response.Body,
			Latency:    req.Response.Latency,
		}
	}

	return apiReq
}

// GetPaginationParams 从请求中获取分页参数
// 参数:
//   - c: Gin上下文
//
// 返回:
//   - page: 页码
//   - size: 每页数量
func GetPaginationParams(c *gin.Context) (page, size int) {
	page = 1  // 默认页码为1
	size = 20 // 默认每页20条

	// 解析查询参数
	pageParam := c.Query("page")
	sizeParam := c.Query("size")

	// 转换页码参数
	if pageParam != "" {
		if parsedPage, err := strconv.Atoi(pageParam); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	// 转换每页大小参数
	if sizeParam != "" {
		if parsedSize, err := strconv.Atoi(sizeParam); err == nil && parsedSize > 0 {
			size = parsedSize
		}
	}

	return page, size
}
