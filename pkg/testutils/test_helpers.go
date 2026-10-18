package testutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/llm-sec/mitm-openai-server/pkg/storage"
)

// CreateTestStorage 创建一个用于测试的存储实例
func CreateTestStorage(t *testing.T) (storage.Storage, func()) {
	tempDir, err := os.MkdirTemp("", "server_test")
	assert.NoError(t, err, "应能创建临时目录")

	// 创建SQLite存储
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := storage.NewSQLiteStorage(dbPath)
	assert.NoError(t, err, "应能创建SQLite存储")

	// 清理函数
	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

// CreateTestGinContext 创建一个用于测试的Gin上下文
func CreateTestGinContext(method string, path string, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

// AssertStatusCode 断言状态码
func AssertStatusCode(t *testing.T, expected int, actual int, message string) {
	assert.Equal(t, expected, actual, message)
}

// AssertJSONResponse 断言JSON响应
func AssertJSONResponse(t *testing.T, responseBody []byte, assertFunc func(map[string]interface{})) {
	var response map[string]interface{}
	err := json.Unmarshal(responseBody, &response)
	assert.NoError(t, err, "响应应为有效JSON")
	assertFunc(response)
}

// AssertJSONArrayResponse 断言JSON数组响应
func AssertJSONArrayResponse(t *testing.T, responseBody []byte, assertFunc func([]interface{})) {
	var response []interface{}
	err := json.Unmarshal(responseBody, &response)
	assert.NoError(t, err, "响应应为有效JSON数组")
	assertFunc(response)
}

// CreateTestRequestWithParams 创建带路径参数的测试请求
func CreateTestRequestWithParams(method string, path string, params gin.Params, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = params
	return c, w
}

// CreateTestRequest 创建一个测试请求
func CreateTestRequest(id string) *storage.Request {
	if id == "" {
		id = fmt.Sprintf("test-%d", time.Now().UnixNano())
	}
	return &storage.Request{
		ID:        id,
		Method:    "GET",
		Path:      "/api/test",
		Timestamp: time.Now(),
		Headers: map[string]string{
			"User-Agent": "Test Agent",
			"Accept":     "application/json",
		},
		Query: map[string]string{
			"param1": "value1",
			"param2": "value2",
		},
		Body:     `{"test": "data"}`,
		ClientIP: "127.0.0.1",
		Response: &storage.ProxyResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:    `{"result": "success"}`,
			Latency: 123,
		},
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}
}

// TestStorageImplementation 测试Storage接口的实现
// 此方法可以被其他包引用，用于测试存储实现
func TestStorageImplementation(t *testing.T, storage storage.Storage) {
	// 清理存储
	err := storage.DeleteAllRequests()
	require.NoError(t, err)

	// 保存请求
	t.Run("SaveRequest", func(t *testing.T) {
		req := CreateTestRequest("test1")
		req.UserID = 1
		err := storage.SaveRequest(1, req)
		assert.NoError(t, err)
	})

	// 获取请求
	t.Run("GetRequestByID", func(t *testing.T) {
		// 先保存请求
		req := CreateTestRequest("test2")
		req.UserID = 1
		err := storage.SaveRequest(1, req)
		require.NoError(t, err)

		// 获取请求
		retrievedReq, err := storage.GetRequestByID(1, "test2")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedReq)
		assert.Equal(t, "test2", retrievedReq.ID)
		assert.Equal(t, "GET", retrievedReq.Method)
		assert.Equal(t, "/api/test", retrievedReq.Path)
		assert.NotNil(t, retrievedReq.Response)
		assert.Equal(t, 200, retrievedReq.Response.StatusCode)
	})

	// 获取不存在的请求
	t.Run("GetNonExistentRequest", func(t *testing.T) {
		_, err := storage.GetRequestByID(1, "non-existent")
		assert.Error(t, err)
	})

	// 获取所有请求
	t.Run("GetAllRequests", func(t *testing.T) {
		// 先清理
		err := storage.DeleteAllRequests()
		require.NoError(t, err)

		// 保存多个请求
		for i := 1; i <= 5; i++ {
			id := fmt.Sprintf("batch-test-%d", i)
			req := CreateTestRequest(id)
			req.UserID = 1
			err := storage.SaveRequest(1, req)
			require.NoError(t, err)
		}

		// 获取所有请求
		allRequests, err := storage.GetAllRequests(10, 0)
		assert.NoError(t, err)
		assert.Len(t, allRequests, 5)
	})

	// 分页测试
	t.Run("Pagination", func(t *testing.T) {
		// 先清理
		err := storage.DeleteAllRequests()
		require.NoError(t, err)

		// 保存10个请求
		for i := 1; i <= 10; i++ {
			id := fmt.Sprintf("pagination-test-%d", i)
			req := CreateTestRequest(id)
			// 设置时间为递增，确保排序一致
			req.Timestamp = time.Now().Add(time.Duration(i) * time.Minute)
			req.UserID = 1
			err := storage.SaveRequest(1, req)
			require.NoError(t, err)
		}

		// 第一页
		page1, err := storage.GetAllRequests(3, 0)
		assert.NoError(t, err)
		assert.Len(t, page1, 3)

		// 第二页
		page2, err := storage.GetAllRequests(3, 3)
		assert.NoError(t, err)
		assert.Len(t, page2, 3)

		// 确保页面没有重叠
		ids1 := make(map[string]bool)
		for _, req := range page1 {
			ids1[req.ID] = true
		}
		for _, req := range page2 {
			assert.False(t, ids1[req.ID], "请求ID不应该在两个页面中重复出现")
		}
	})

	// 删除请求
	t.Run("DeleteRequest", func(t *testing.T) {
		// 先保存请求
		req := CreateTestRequest("delete-test")
		req.UserID = 1
		err := storage.SaveRequest(1, req)
		require.NoError(t, err)

		// 确认请求存在
		_, err = storage.GetRequestByID(1, "delete-test")
		require.NoError(t, err)

		// 删除请求
		err = storage.DeleteRequest(1, "delete-test")
		assert.NoError(t, err)

		// 确认请求已删除
		_, err = storage.GetRequestByID(1, "delete-test")
		assert.Error(t, err)
	})

	// 删除所有请求
	t.Run("DeleteAllRequests", func(t *testing.T) {
		// 先保存几个请求
		for i := 1; i <= 3; i++ {
			id := fmt.Sprintf("delete-all-test-%d", i)
			req := CreateTestRequest(id)
			req.UserID = 1
			err := storage.SaveRequest(1, req)
			require.NoError(t, err)
		}

		// 删除所有请求
		err := storage.DeleteAllRequests()
		assert.NoError(t, err)

		// 确认所有请求已删除
		allRequests, err := storage.GetAllRequests(100, 0)
		assert.NoError(t, err)
		assert.Empty(t, allRequests)
	})

	// 导出请求
	t.Run("ExportRequests", func(t *testing.T) {
		// 先清理
		err := storage.DeleteAllRequests()
		require.NoError(t, err)

		// 保存几个请求
		for i := 1; i <= 3; i++ {
			id := fmt.Sprintf("export-test-%d", i)
			req := CreateTestRequest(id)
			req.UserID = 1
			err := storage.SaveRequest(1, req)
			require.NoError(t, err)
		}

		// 导出请求
		exportPath, err := storage.ExportRequests()
		assert.NoError(t, err)
		assert.NotEmpty(t, exportPath)

		// 确认导出文件存在
		_, err = os.Stat(exportPath)
		assert.NoError(t, err)
	})
}

// TestStorageReopen 测试重新打开存储后能否恢复数据
func TestStorageReopen(t *testing.T, createStorage func() (storage.Storage, error), cleanupFn func()) {
	var storage storage.Storage
	var err error

	// 清理上一个测试的数据
	if cleanupFn != nil {
		defer cleanupFn()
	}

	// 第一次打开存储
	storage, err = createStorage()
	require.NoError(t, err)

	// 清理数据
	err = storage.DeleteAllRequests()
	require.NoError(t, err)

	// 保存数据
	req := CreateTestRequest("reopen-test")
	req.UserID = 1
	err = storage.SaveRequest(1, req)
	require.NoError(t, err)

	// 关闭存储
	err = storage.Close()
	require.NoError(t, err)

	// 重新打开存储
	storage, err = createStorage()
	require.NoError(t, err)
	defer storage.Close()

	// 验证数据是否还存在
	retrievedReq, err := storage.GetRequestByID(1, "reopen-test")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedReq)
	assert.Equal(t, "reopen-test", retrievedReq.ID)
}
