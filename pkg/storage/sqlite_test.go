package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestRequest 创建用于测试的请求对象
func createTestRequest(id string) *Request {
	return &Request{
		ID:        id,
		Method:    "POST",
		Path:      "/api/test/" + id,
		Timestamp: time.Now(),
		Headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "Test-Agent",
		},
		Query: map[string]string{
			"param1": "value1",
			"param2": "value2",
		},
		Body:     map[string]interface{}{"test": "data"},
		ClientIP: "127.0.0.1",
		Response: &ProxyResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:    map[string]interface{}{"status": "success"},
			Latency: 50,
		},
	}
}

// createTestUser 创建一个测试用户并返回其ID（requests表有外键约束，必须先有用户）
func createTestUser(t *testing.T, storage Storage) int64 {
	user := &User{
		Username: fmt.Sprintf("testuser-%d", time.Now().UnixNano()),
		Password: "testpass",
		UserType: "user",
		IsActive: true,
	}
	err := storage.CreateUser(user)
	require.NoError(t, err)
	return user.ID
}

// testStorageReopen 测试存储重新打开的功能
func testStorageReopen(t *testing.T, createStorageFn func() (Storage, error), cleanup func()) {
	// 创建存储实例
	storage, err := createStorageFn()
	require.NoError(t, err)

	// 创建测试用户
	userID := createTestUser(t, storage)

	// 添加测试数据
	req := createTestRequest("reopen-test")
	req.UserID = userID
	err = storage.SaveRequest(userID, req)
	require.NoError(t, err)

	// 关闭存储
	err = storage.Close()
	require.NoError(t, err)

	// 重新打开存储
	storage, err = createStorageFn()
	require.NoError(t, err)
	defer storage.Close()

	// 验证数据仍然存在
	retrievedReq, err := storage.GetRequestByID(userID, "reopen-test")
	require.NoError(t, err)
	assert.Equal(t, "reopen-test", retrievedReq.ID)
	assert.Equal(t, "/api/test/reopen-test", retrievedReq.Path)

	// 清理测试数据
	err = storage.DeleteRequest(userID, "reopen-test")
	assert.NoError(t, err)
}

func TestSQLiteStorage(t *testing.T) {
	// 创建临时SQLite数据库目录
	tempDir, err := os.MkdirTemp("", "sqlite-storage-test-")
	if err != nil {
		t.Fatalf("创建临时数据库目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir) // 测试完成后清理

	// 创建SQLiteStorage实例
	storage, err := NewSQLiteStorage(tempDir)
	if err != nil {
		t.Fatalf("创建SQLiteStorage失败: %v", err)
	}
	defer storage.Close()

	// 创建测试用户（requests表有外键约束）
	userID := createTestUser(t, storage)

	// 测试保存请求
	t.Run("SaveRequest", func(t *testing.T) {
		req := &Request{
			ID:        "test-id-1",
			Method:    "GET",
			Path:      "/api/test",
			Timestamp: time.Now(),
			Headers: map[string]string{
				"User-Agent": "Test-Agent",
			},
			ClientIP: "127.0.0.1",
			Response: &ProxyResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body:    `{"status":"ok"}`,
				Latency: 50,
			},
		}

		if err := storage.SaveRequest(userID, req); err != nil {
			t.Fatalf("保存请求失败: %v", err)
		}
	})

	// 测试获取单个请求
	t.Run("GetRequestByID", func(t *testing.T) {
		req, err := storage.GetRequestByID(userID, "test-id-1")
		if err != nil {
			t.Fatalf("获取请求失败: %v", err)
		}

		if req.ID != "test-id-1" || req.Method != "GET" || req.Path != "/api/test" {
			t.Fatalf("获取的请求数据不匹配: %+v", req)
		}

		if req.Response == nil || req.Response.StatusCode != 200 {
			t.Fatalf("响应数据不匹配或缺失: %+v", req.Response)
		}
	})

	// 测试获取不存在的请求
	t.Run("GetRequestByID_NotFound", func(t *testing.T) {
		_, err := storage.GetRequestByID(userID, "non-existent-id")
		if err == nil {
			t.Fatalf("应该返回错误，但返回了nil")
		}
	})

	// 保存更多请求用于测试分页
	t.Run("SaveMultipleRequests", func(t *testing.T) {
		// 保存10个请求
		for i := 2; i <= 10; i++ {
			req := &Request{
				ID:        "test-id-" + string([]byte{byte('0') + byte(i)}),
				Method:    "POST",
				Path:      "/api/test" + string([]byte{byte('0') + byte(i)}),
				Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				ClientIP:  "127.0.0.1",
				Response: &ProxyResponse{
					StatusCode: 201,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body:    `{"created":true}`,
					Latency: 30 + int64(i),
				},
			}
			if err := storage.SaveRequest(userID, req); err != nil {
				t.Fatalf("保存请求失败: %v", err)
			}
		}
	})

	// 测试获取所有请求
	t.Run("GetAllRequests", func(t *testing.T) {
		// 不分页，获取所有
		requests, err := storage.GetAllRequests(0, 0)
		if err != nil {
			t.Fatalf("获取所有请求失败: %v", err)
		}

		if len(requests) != 10 {
			t.Fatalf("期望获取10条请求，实际获取 %d 条", len(requests))
		}

		// 测试分页
		pageRequests, err := storage.GetAllRequests(5, 0)
		if err != nil {
			t.Fatalf("获取分页请求失败: %v", err)
		}

		if len(pageRequests) != 5 {
			t.Fatalf("期望获取5条请求，实际获取 %d 条", len(pageRequests))
		}

		// 测试偏移
		offsetRequests, err := storage.GetAllRequests(5, 5)
		if err != nil {
			t.Fatalf("获取偏移请求失败: %v", err)
		}

		if len(offsetRequests) != 5 {
			t.Fatalf("期望获取5条请求，实际获取 %d 条", len(offsetRequests))
		}

		// 超出范围的偏移
		beyondRequests, err := storage.GetAllRequests(5, 20)
		if err != nil {
			t.Fatalf("获取超出范围请求失败: %v", err)
		}

		if len(beyondRequests) != 0 {
			t.Fatalf("期望获取0条请求，实际获取 %d 条", len(beyondRequests))
		}
	})

	// 测试删除单个请求
	t.Run("DeleteRequest", func(t *testing.T) {
		if err := storage.DeleteRequest(userID, "test-id-1"); err != nil {
			t.Fatalf("删除请求失败: %v", err)
		}

		// 尝试再次获取已删除的请求
		_, err := storage.GetRequestByID(userID, "test-id-1")
		if err == nil {
			t.Fatalf("应该返回错误，但返回了nil")
		}
	})

	// 测试删除不存在的请求
	t.Run("DeleteRequest_NotFound", func(t *testing.T) {
		err := storage.DeleteRequest(userID, "non-existent-id")
		if err == nil {
			t.Fatalf("应该返回错误，但返回了nil")
		}
	})

	// 测试导出请求
	t.Run("ExportRequests", func(t *testing.T) {
		exportPath, err := storage.ExportRequests()
		if err != nil {
			t.Fatalf("导出请求失败: %v", err)
		}

		// 验证导出文件是否存在
		if _, err := os.Stat(exportPath); os.IsNotExist(err) {
			t.Fatalf("导出文件不存在: %s", exportPath)
		}

		// 清理导出文件
		os.Remove(exportPath)
	})

	// 测试获取统计信息
	t.Run("GetStats", func(t *testing.T) {
		stats, err := storage.GetStats()
		if err != nil {
			t.Fatalf("获取统计信息失败: %v", err)
		}

		totalRequests, ok := stats["total_requests"].(int)
		if !ok {
			t.Fatalf("统计信息中没有total_requests字段或类型错误")
		}

		// 我们已删除一个请求，所以现在应该有9个
		if totalRequests != 9 {
			t.Fatalf("期望有9个请求，但统计结果为 %d", totalRequests)
		}

		methods, ok := stats["method_counts"].(map[string]int)
		if !ok {
			t.Fatalf("统计信息中没有method_counts字段或类型错误")
		}

		// 验证方法统计
		if methods["POST"] != 9 {
			t.Fatalf("期望有9个POST请求，但统计结果为 %d", methods["POST"])
		}
	})

	// 测试删除所有请求
	t.Run("DeleteAllRequests", func(t *testing.T) {
		if err := storage.DeleteAllRequests(); err != nil {
			t.Fatalf("删除所有请求失败: %v", err)
		}

		// 确认获取所有请求返回空结果
		requests, err := storage.GetAllRequests(100, 0)
		if err != nil {
			t.Fatalf("获取所有请求失败: %v", err)
		}

		if len(requests) != 0 {
			t.Fatalf("期望获取0条请求，实际获取 %d 条", len(requests))
		}
	})
}

func TestSQLiteStorageReopen(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "sqlite-storage-reopen-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	createStorage := func() (Storage, error) {
		return NewSQLiteStorage(tempDir)
	}

	t.Run("SQLiteStorageReopen", func(t *testing.T) {
		testStorageReopen(t, createStorage, func() {
			os.RemoveAll(tempDir)
		})
	})
}

func TestSQLiteStorageEdgeCases(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "sqlite-storage-edge-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("DefaultLocation", func(t *testing.T) {
		// 测试不提供路径时的默认位置
		storage, err := NewSQLiteStorage("")
		if err != nil {
			t.Skipf("默认位置测试失败: %v", err)
		}
		defer storage.Close()

		// 创建测试用户
		userID := createTestUser(t, storage)

		// 验证可以正常工作
		req := createTestRequest("default-sqlite-location-test")
		req.UserID = userID
		err = storage.SaveRequest(userID, req)
		require.NoError(t, err)

		// 获取存储统计信息，确认工作正常
		stats, err := storage.GetStats()
		require.NoError(t, err)
		require.NotNil(t, stats)

		// 清理
		err = storage.DeleteRequest(userID, "default-sqlite-location-test")
		assert.NoError(t, err)
	})

	t.Run("InvalidPath", func(t *testing.T) {
		// 尝试在一个无法创建目录的路径上创建存储
		// 使用根目录下的一个位置，通常普通用户没有权限创建文件夹
		if os.Getuid() == 0 {
			t.Skip("以root用户运行测试时跳过此测试")
		}

		storage, err := NewSQLiteStorage("/root/forbidden/path")
		if err == nil {
			storage.Close() // 清理资源
			t.Error("应该无法在无权限路径创建存储")
		} else {
			t.Logf("预期错误: %v", err)
		}
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		storage, err := NewSQLiteStorage(tempDir)
		require.NoError(t, err)
		defer storage.Close()

		// 清理存储
		err = storage.DeleteAllRequests()
		require.NoError(t, err)

		// 创建测试用户
		userID := createTestUser(t, storage)

		// 并发保存10个请求（不要太多导致超时）
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(idx int) {
				id := fmt.Sprintf("concurrent-test-%d", idx)
				req := createTestRequest(id)
				req.UserID = userID
				err := storage.SaveRequest(userID, req)
				if err != nil {
					t.Errorf("保存请求 %s 失败: %v", id, err)
				}
				done <- true
			}(i)
		}

		// 等待所有请求完成
		for i := 0; i < 10; i++ {
			<-done
		}

		// 验证所有请求都已保存
		requests, err := storage.GetAllRequests(20, 0)
		require.NoError(t, err)
		require.Len(t, requests, 10)
	})
}
