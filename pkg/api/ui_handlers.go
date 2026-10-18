package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
	"github.com/llm-sec/mitm-openai-server/pkg/utils"
)

// UIServer 表示UI API服务器
type UIServer struct {
	storage       storage.Storage
	config        ServerConfig
	openaiService OpenAIServiceInterface
	// 添加令牌验证存储
	tokenStore sync.Map
}

// UserContext 用户上下文信息
type UserContext struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	UserType string `json:"user_type"`
}

// NewUIServer 创建一个新的UI服务器实例
func NewUIServer(storage storage.Storage, config ServerConfig, openaiService OpenAIServiceInterface) *UIServer {
	return &UIServer{
		storage:       storage,
		config:        config,
		openaiService: openaiService,
	}
}

// SetupUIRoutes 设置UI相关的路由
func (s *UIServer) SetupUIRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc, apiMiddleware gin.HandlerFunc) {
	// 确保UI路径有效
	uiPath := s.config.UIDir
	if uiPath == "" {
		uiPath = "./ui" // 默认UI目录
	}

	// UI认证中间件（从token提取用户信息）
	uiAuthMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, StandardResponse{
				Code: 10002,
				Msg:  "认证失败：缺少Authorization头",
			})
			c.Abort()
			return
		}

		// 支持"Bearer token"格式和直接的令牌格式
		token := authHeader
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = authHeader[7:] // 去掉"Bearer "前缀
		}

		// 验证令牌并获取用户上下文
		userInfo, found := s.tokenStore.Load(token)
		if !found {
			c.JSON(http.StatusUnauthorized, StandardResponse{
				Code: 10002,
				Msg:  "认证失败：无效的token",
			})
			c.Abort()
			return
		}

		// 将用户上下文设置到gin.Context中
		userContext := userInfo.(UserContext)
		c.Set("user_id", userContext.UserID)
		c.Set("username", userContext.Username)
		c.Set("user_type", userContext.UserType)

		c.Next()
	}

	// UI API路由组 - 需要UI认证
	uiAPI := router.Group("/ui/api")

	// UI 登录接口 - 登录接口不需要认证
	uiAPI.POST("/login", s.HandleUILogin)

	// UI 注册接口 - 注册接口不需要认证
	uiAPI.POST("/register", s.HandleUIRegister)

	// 添加UI认证中间件
	uiAPI.Use(uiAuthMiddleware)

	// 请求相关接口（用户隔离）
	uiAPI.GET("/requests", s.GetRequests)
	uiAPI.GET("/requests/:id", s.GetRequestByID)
	uiAPI.DELETE("/requests/:id", s.DeleteRequest)
	uiAPI.DELETE("/requests", s.DeleteAllRequests)

	// 文件导出（用户隔离）
	uiAPI.GET("/export", s.ExportRequests)

	// 存储统计（用户隔离）
	uiAPI.GET("/storage-stats", s.GetStorageStats)

	// 服务器信息
	uiAPI.GET("/server-info", s.GetServerInfo)

	// 代理配置（用户隔离）
	uiAPI.GET("/proxy-config", s.GetProxyConfig)
	uiAPI.POST("/proxy-config", s.SaveProxyConfig)

	// 用户个人资料
	uiAPI.GET("/profile", s.GetUserProfile)
	uiAPI.PUT("/profile", s.UpdateUserProfile)

	// 添加API Token获取接口
	uiAPI.GET("/token", s.GetAPIToken)

	// 添加聊天测试接口
	uiAPI.POST("/chat", s.HandleUIChat)

	// 管理员专用接口
	adminAPI := uiAPI.Group("/admin")
	adminAPI.Use(s.adminOnlyMiddleware)
	adminAPI.GET("/users", s.ListUsers)
	adminAPI.GET("/all-requests", s.GetAllRequests)
	adminAPI.DELETE("/all-requests", s.DeleteAllRequests)
	adminAPI.GET("/global-stats", s.GetGlobalStats)
}

// getCurrentUser 从gin.Context中获取当前用户信息
func (s *UIServer) getCurrentUser(c *gin.Context) (int64, string, string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, "", "", fmt.Errorf("用户未认证")
	}

	username, _ := c.Get("username")
	userType, _ := c.Get("user_type")

	return userID.(int64), username.(string), userType.(string), nil
}

// adminOnlyMiddleware 管理员专用中间件
func (s *UIServer) adminOnlyMiddleware(c *gin.Context) {
	_, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		c.Abort()
		return
	}

	if userType != "root" {
		c.JSON(http.StatusForbidden, StandardResponse{
			Code: 10003,
			Msg:  "权限不足：需要管理员权限",
		})
		c.Abort()
		return
	}

	c.Next()
}

// ==================== 用户认证相关 ====================

// HandleUILogin 处理UI登录请求
func (s *UIServer) HandleUILogin(c *gin.Context) {
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "无效的请求格式",
		})
		return
	}

	// 去除用户名和密码中可能的前后空格
	username := strings.TrimSpace(loginReq.Username)
	password := strings.TrimSpace(loginReq.Password)

	var user *storage.User
	var err error

	// 1. 首先检查是否为root用户（使用配置中的凭据）
	configUsername := strings.TrimSpace(s.config.UIUsername)
	configPassword := strings.TrimSpace(s.config.UIPassword)

	if username == configUsername && password == configPassword {
		// root用户使用配置中的凭据，创建虚拟用户对象
		user = &storage.User{
			ID:       0, // root用户使用特殊ID 0
			Username: username,
			UserType: "root",
			IsActive: true,
		}
		fmt.Printf("root用户登录成功: %s\n", username)
	} else {
		// 2. 检查数据库中的普通用户
		user, err = s.storage.ValidateUserCredentials(username, password)
		if err != nil {
			fmt.Printf("登录失败: %s - %v\n", username, err)
			c.JSON(http.StatusUnauthorized, StandardResponse{
				Code: 10002,
				Msg:  "用户名或密码错误",
			})
			return
		}

		fmt.Printf("普通用户登录成功: %s\n", username)

		// 更新普通用户的最后登录时间
		s.storage.UpdateUserLastLogin(user.ID)
	}

	// 登录成功，生成令牌
	token := fmt.Sprintf("%s_%d", uuid.New().String(), time.Now().Unix())

	// 将用户上下文存储到内存中
	userContext := UserContext{
		UserID:   user.ID,
		Username: user.Username,
		UserType: user.UserType,
	}
	s.tokenStore.Store(token, userContext)

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "登录成功",
		Data: gin.H{
			"token":    token,
			"userType": user.UserType,
			"username": user.Username,
		},
	})
}

// HandleUIRegister 处理UI用户注册请求
func (s *UIServer) HandleUIRegister(c *gin.Context) {
	var registerReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&registerReq); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "无效的请求格式",
		})
		return
	}

	// 验证输入
	username := strings.TrimSpace(registerReq.Username)
	password := strings.TrimSpace(registerReq.Password)

	if username == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "用户名不能为空",
		})
		return
	}

	if password == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "密码不能为空",
		})
		return
	}

	if len(username) < 3 {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "用户名至少需要3个字符",
		})
		return
	}

	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "密码至少需要6个字符",
		})
		return
	}

	// 检查是否为保留用户名（root）
	if strings.ToLower(username) == "root" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "root是保留用户名，无法注册",
		})
		return
	}

	// 创建用户
	user := &storage.User{
		Username:          username,
		Password:          password, // 实际应用中应该加密存储
		UserType:          "user",
		IsActive:          true,
		ProxyEnabled:      false,
		MaxRequests:       10000,
		DataRetentionDays: 30,
	}

	if err := s.storage.CreateUser(user); err != nil {
		fmt.Printf("创建用户失败: %v\n", err)
		if strings.Contains(err.Error(), "用户名已存在") {
			c.JSON(http.StatusConflict, StandardResponse{
				Code: 10004,
				Msg:  "注册失败：用户名已被使用，请换一个用户名重试",
			})
		} else {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Code: 10005,
				Msg:  "注册失败：服务器内部错误，请稍后重试",
			})
		}
		return
	}

	fmt.Printf("用户注册成功: %s (ID: %d)\n", username, user.ID)

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "注册成功，请使用新账号登录",
	})
}

// ==================== 请求相关接口（用户隔离） ====================

// GetRequests 获取当前用户的请求列表
func (s *UIServer) GetRequests(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	// 处理分页参数
	page, size := GetPaginationParams(c)
	limit := size
	offset := (page - 1) * size

	var requests []*storage.Request
	var total int64

	// 根据用户类型决定数据访问范围
	if userType == "root" {
		// root用户可以看到所有数据
		requests, err = s.storage.GetAllRequests(limit, offset)
		if err == nil {
			// 获取总数
			stats, statsErr := s.storage.GetStats()
			if statsErr == nil && stats["total_requests"] != nil {
				if totalInt, ok := stats["total_requests"].(int); ok {
					total = int64(totalInt)
				}
			}
		}
	} else {
		// 普通用户只能看到自己的数据
		requests, err = s.storage.GetUserRequests(userID, limit, offset)
		if err == nil {
			// 获取用户总数
			stats, statsErr := s.storage.GetUserStats(userID)
			if statsErr == nil && stats["total_requests"] != nil {
				if totalInt, ok := stats["total_requests"].(int); ok {
					total = int64(totalInt)
				}
			}
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10012,
			Msg:  "获取请求列表失败: " + err.Error(),
		})
		return
	}

	// 转换为API请求模型
	serverRequests := make([]*Request, len(requests))
	for i, req := range requests {
		serverRequests[i] = ConvertStorageToAPIRequest(req)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取请求列表成功",
		Data: map[string]interface{}{
			"requests": serverRequests,
			"total":    total,
			"page":     page,
			"size":     size,
		},
	})
}

// GetRequestByID 获取特定ID的请求（用户隔离）
func (s *UIServer) GetRequestByID(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	// 获取请求ID
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10003,
			Msg:  "请求ID不能为空",
		})
		return
	}

	var req *storage.Request

	// 根据用户类型决定数据访问范围
	if userType == "root" {
		// root用户可以访问任何请求，但需要先获取请求以确定归属用户
		allRequests, err := s.storage.GetAllRequests(1, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Code: 10004,
				Msg:  "查询请求失败: " + err.Error(),
			})
			return
		}

		// 查找指定ID的请求
		for _, r := range allRequests {
			if r.ID == id {
				req, err = s.storage.GetRequestByID(r.UserID, id)
				break
			}
		}
		if req == nil && err == nil {
			err = fmt.Errorf("请求不存在")
		}
	} else {
		// 普通用户只能访问自己的请求
		req, err = s.storage.GetRequestByID(userID, id)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Code: 10004,
			Msg:  "请求不存在或无权限访问: " + err.Error(),
		})
		return
	}

	// 转换为API请求模型
	serverReq := ConvertStorageToAPIRequest(req)

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "请求获取成功",
		Data: serverReq,
	})
}

// DeleteRequest 删除特定ID的请求（用户隔离）
func (s *UIServer) DeleteRequest(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	// 获取请求ID
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10006,
			Msg:  "请求ID不能为空",
		})
		return
	}

	// 根据用户类型决定删除范围
	if userType == "root" {
		// root用户可以删除任何请求，但需要先获取请求以确定归属用户
		allRequests, err := s.storage.GetAllRequests(10000, 0) // 获取更多请求来查找
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Code: 10007,
				Msg:  "查询请求失败: " + err.Error(),
			})
			return
		}

		// 查找指定ID的请求
		var targetUserID int64 = -1
		for _, r := range allRequests {
			if r.ID == id {
				targetUserID = r.UserID
				break
			}
		}

		if targetUserID == -1 {
			c.JSON(http.StatusNotFound, StandardResponse{
				Code: 10007,
				Msg:  "请求不存在",
			})
			return
		}

		err = s.storage.DeleteRequest(targetUserID, id)
	} else {
		// 普通用户只能删除自己的请求
		err = s.storage.DeleteRequest(userID, id)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10007,
			Msg:  "删除请求失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "请求已成功删除",
	})
}

// DeleteAllRequests 删除当前用户的所有请求
func (s *UIServer) DeleteAllRequests(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	// 根据用户类型决定删除范围
	if userType == "root" {
		// root用户删除所有请求
		err = s.storage.DeleteAllRequests()
	} else {
		// 普通用户只删除自己的请求
		err = s.storage.DeleteAllUserRequests(userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10008,
			Msg:  "删除请求失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "请求已成功删除",
	})
}

// ExportRequests 导出当前用户的请求为JSON格式
func (s *UIServer) ExportRequests(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	var filePath string

	// 根据用户类型决定导出范围
	if userType == "root" {
		// root用户导出所有请求
		filePath, err = s.storage.ExportRequests()
	} else {
		// 普通用户只导出自己的请求
		filePath, err = s.storage.ExportUserRequests(userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10009,
			Msg:  "导出请求失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "请求已成功导出",
		Data: map[string]string{
			"file_path": filePath,
		},
	})
}

// GetStorageStats 获取当前用户的存储统计信息
func (s *UIServer) GetStorageStats(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	var stats map[string]interface{}

	// 根据用户类型决定统计范围
	if userType == "root" {
		// root用户获取全局统计
		stats, err = s.storage.GetStats()
	} else {
		// 普通用户只获取自己的统计
		stats, err = s.storage.GetUserStats(userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10010,
			Msg:  "获取存储统计信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取存储统计信息成功",
		Data: stats,
	})
}

// ==================== 配置相关接口（用户隔离） ====================

// GetProxyConfig 获取当前用户的代理配置
func (s *UIServer) GetProxyConfig(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	var config map[string]interface{}

	if userType == "root" {
		// root用户使用服务器配置
		config = map[string]interface{}{
			"enabled":   s.config.ProxyMode,
			"targetURL": s.config.TargetURL,
			"authType":  s.config.TargetAuthType,
		}

		// 根据认证类型添加相应字段
		switch s.config.TargetAuthType {
		case "basic":
			config["username"] = s.config.TargetUsername
			if s.config.TargetPassword != "" {
				config["password"] = "••••••••"
			} else {
				config["password"] = ""
			}
		case "token":
			if s.config.TargetToken != "" {
				config["token"] = "••••••••"
			} else {
				config["token"] = ""
			}
		}
	} else {
		// 普通用户从数据库获取配置
		user, err := s.storage.GetUserByID(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Code: 10011,
				Msg:  "获取用户信息失败: " + err.Error(),
			})
			return
		}

		config = map[string]interface{}{
			"enabled":   user.ProxyEnabled,
			"targetURL": user.ProxyTargetURL,
			"authType":  user.ProxyAuthType,
		}

		// 根据认证类型添加相应字段
		switch user.ProxyAuthType {
		case "basic":
			config["username"] = user.ProxyUsername
			if user.ProxyPassword != "" {
				config["password"] = "••••••••"
			} else {
				config["password"] = ""
			}
		case "token":
			if user.ProxyToken != "" {
				config["token"] = "••••••••"
			} else {
				config["token"] = ""
			}
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "代理配置获取成功",
		Data: config,
	})
}

// SaveProxyConfig 保存当前用户的代理配置
func (s *UIServer) SaveProxyConfig(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	var configReq struct {
		Enabled   bool   `json:"enabled"`
		TargetURL string `json:"targetURL"`
		AuthType  string `json:"authType"`
		Username  string `json:"username,omitempty"`
		Password  string `json:"password,omitempty"`
		Token     string `json:"token,omitempty"`
	}

	if err := c.ShouldBindJSON(&configReq); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "无效的请求格式",
		})
		return
	}

	// 如果启用了代理模式，对目标URL进行安全检查
	if configReq.Enabled && configReq.TargetURL != "" {
		// 获取服务器监听地址
		serverAddr := c.Request.Host
		
		// 检查URL安全性
		if err := utils.IsURLSafe(configReq.TargetURL, serverAddr); err != nil {
			c.JSON(http.StatusBadRequest, StandardResponse{
				Code: 10001,
				Msg:  fmt.Sprintf("目标URL不安全: %s", err.Error()),
			})
			return
		}
	}

	if userType == "root" {
		// root用户更新服务器配置（内存中，不持久化）
		s.config.ProxyMode = configReq.Enabled
		s.config.TargetURL = configReq.TargetURL
		s.config.TargetAuthType = configReq.AuthType
		if configReq.Username != "" {
			s.config.TargetUsername = configReq.Username
		}
		if configReq.Password != "" && configReq.Password != "••••••••" {
			s.config.TargetPassword = configReq.Password
		}
		if configReq.Token != "" && configReq.Token != "••••••••" {
			s.config.TargetToken = configReq.Token
		}
	} else {
		// 普通用户更新数据库配置
		userConfig := &storage.UserConfig{
			ProxyEnabled:   configReq.Enabled,
			ProxyTargetURL: configReq.TargetURL,
			ProxyAuthType:  configReq.AuthType,
		}

		if configReq.Username != "" {
			userConfig.ProxyUsername = configReq.Username
		}
		if configReq.Password != "" && configReq.Password != "••••••••" {
			userConfig.ProxyPassword = configReq.Password
		}
		if configReq.Token != "" && configReq.Token != "••••••••" {
			userConfig.ProxyToken = configReq.Token
		}

		if err := s.storage.UpdateUserConfig(userID, userConfig); err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Code: 10002,
				Msg:  "保存代理配置失败: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "代理配置已更新",
	})
}

// ==================== 用户资料相关 ====================

// GetUserProfile 获取用户个人资料
func (s *UIServer) GetUserProfile(c *gin.Context) {
	userID, username, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	if userType == "root" {
		// root用户返回配置中的信息
		profile := storage.UserProfile{
			ID:       0,
			Username: username,
			UserType: "root",
			IsActive: true,
		}

		c.JSON(http.StatusOK, StandardResponse{
			Code: 0,
			Msg:  "获取用户资料成功",
			Data: profile,
		})
		return
	}

	// 普通用户从数据库获取
	user, err := s.storage.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10011,
			Msg:  "获取用户信息失败: " + err.Error(),
		})
		return
	}

	profile := storage.UserProfile{
		ID:          user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		IsActive:    user.IsActive,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取用户资料成功",
		Data: profile,
	})
}

// UpdateUserProfile 更新用户个人资料
func (s *UIServer) UpdateUserProfile(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	if userType == "root" {
		c.JSON(http.StatusForbidden, StandardResponse{
			Code: 10003,
			Msg:  "root用户配置不能通过此接口修改",
		})
		return
	}

	var updateReq struct {
		Password string `json:"password,omitempty"`
	}

	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "无效的请求格式",
		})
		return
	}

	// 获取当前用户信息
	user, err := s.storage.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10011,
			Msg:  "获取用户信息失败: " + err.Error(),
		})
		return
	}

	// 更新字段
	updated := false

	if updateReq.Password != "" {
		if len(updateReq.Password) < 6 {
			c.JSON(http.StatusBadRequest, StandardResponse{
				Code: 10001,
				Msg:  "密码至少需要6个字符",
			})
			return
		}
		user.Password = updateReq.Password // 实际应用中应该加密
		updated = true
	}

	if !updated {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10001,
			Msg:  "没有提供需要更新的字段",
		})
		return
	}

	// 保存更新
	if err := s.storage.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10012,
			Msg:  "更新用户信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "用户资料更新成功",
	})
}

// ==================== 管理员专用接口 ====================

// ListUsers 获取用户列表（仅管理员可用）
func (s *UIServer) ListUsers(c *gin.Context) {
	page, size := GetPaginationParams(c)
	limit := size
	offset := (page - 1) * size

	users, err := s.storage.ListUsers(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10013,
			Msg:  "获取用户列表失败: " + err.Error(),
		})
		return
	}

	// 转换为UserProfile格式（不包含敏感信息）
	profiles := make([]storage.UserProfile, len(users))
	for i, user := range users {
		profiles[i] = storage.UserProfile{
			ID:          user.ID,
			Username:    user.Username,
			UserType:    user.UserType,
			IsActive:    user.IsActive,
			CreatedAt:   user.CreatedAt,
			LastLoginAt: user.LastLoginAt,
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取用户列表成功",
		Data: map[string]interface{}{
			"users": profiles,
			"page":  page,
			"size":  size,
		},
	})
}

// GetAllRequests 获取所有请求（仅管理员可用）
func (s *UIServer) GetAllRequests(c *gin.Context) {
	page, size := GetPaginationParams(c)
	limit := size
	offset := (page - 1) * size

	requests, err := s.storage.GetAllRequests(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10012,
			Msg:  "获取所有请求失败: " + err.Error(),
		})
		return
	}

	// 获取总数
	var total int64
	stats, statsErr := s.storage.GetStats()
	if statsErr == nil && stats["total_requests"] != nil {
		if totalInt, ok := stats["total_requests"].(int); ok {
			total = int64(totalInt)
		}
	}

	// 转换为API请求模型
	serverRequests := make([]*Request, len(requests))
	for i, req := range requests {
		serverRequests[i] = ConvertStorageToAPIRequest(req)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取所有请求成功",
		Data: map[string]interface{}{
			"requests": serverRequests,
			"total":    total,
			"page":     page,
			"size":     size,
		},
	})
}

// GetGlobalStats 获取全局统计信息（仅管理员可用）
func (s *UIServer) GetGlobalStats(c *gin.Context) {
	stats, err := s.storage.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10010,
			Msg:  "获取全局统计信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取全局统计信息成功",
		Data: stats,
	})
}

// ==================== 其他接口 ====================

// GetServerInfo 返回服务器信息
func (s *UIServer) GetServerInfo(c *gin.Context) {
	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取服务器信息成功",
		Data: gin.H{
			"version":     "2.0.0",
			"apiBasePath": "/",
			"openApiPath": "/openai.json",
			"features": gin.H{
				"user_isolation":   true,
				"multi_user":       true,
				"database_storage": true,
			},
			"auth": gin.H{
				"enabled": s.config.EnableAuth,
				"type":    s.config.AuthType,
			},
		},
	})
}

// GetAPIToken 返回用于OpenAI API请求的token
func (s *UIServer) GetAPIToken(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	var tokenValue string

	if userType == "root" {
		// root用户使用服务器配置的token
		if s.config.ProxyMode && s.config.TargetAuthType == "token" && s.config.TargetToken != "" {
			tokenValue = s.config.TargetToken
		} else {
			tokenValue = "mt-root-server-token"
		}
	} else {
		// 普通用户使用自己配置的token
		user, err := s.storage.GetUserByID(userID)
		if err == nil && user.ProxyEnabled && user.ProxyAuthType == "token" && user.ProxyToken != "" {
			tokenValue = user.ProxyToken
		} else {
			tokenValue = fmt.Sprintf("mt-user-%d-token", userID)
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Code: 0,
		Msg:  "获取API Token成功",
		Data: map[string]string{
			"token": tokenValue,
		},
	})
}

// HandleUIChat 处理前端UI的聊天请求
func (s *UIServer) HandleUIChat(c *gin.Context) {
	userID, _, userType, err := s.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Code: 10002,
			Msg:  "认证失败",
		})
		return
	}

	// 从请求中获取消息数据
	var chatRequest struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
	}

	if err := c.ShouldBindJSON(&chatRequest); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Code: 10020,
			Msg:  "无效的聊天请求: " + err.Error(),
		})
		return
	}

	// 使用OpenAI服务处理聊天请求
	requestBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10021,
			Msg:  "读取请求失败: " + err.Error(),
		})
		return
	}

	// 设置请求头
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   fmt.Sprintf("MITM-Server-User-%d", userID),
	}

	// 根据用户类型使用不同的认证
	if userType == "root" {
		if s.config.TargetAuthType == "token" && s.config.TargetToken != "" {
			headers["Authorization"] = "Bearer " + s.config.TargetToken
		}
	} else {
		// 获取用户配置
		user, err := s.storage.GetUserByID(userID)
		if err == nil && user.ProxyEnabled && user.ProxyAuthType == "token" && user.ProxyToken != "" {
			headers["Authorization"] = "Bearer " + user.ProxyToken
		}
	}

	// 使用OpenAI服务处理请求
	statusCode, responseHeaders, responseBody, err := s.openaiService.HandleRequest(
		"POST", "/chat/completions", headers, map[string]string{}, requestBody,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Code: 10022,
			Msg:  "聊天请求处理失败: " + err.Error(),
		})
		return
	}

	// 设置响应头
	for key, value := range responseHeaders {
		c.Header(key, value)
	}

	// 返回OpenAI的原始响应
	c.JSON(statusCode, responseBody)
}

// ValidateToken 验证令牌是否有效
func (s *UIServer) ValidateToken(token string) bool {
	_, found := s.tokenStore.Load(token)
	return found
}

// SetConfig 设置或更新服务器配置
func (s *UIServer) SetConfig(config ServerConfig) {
	if config.UIUsername != "" {
		s.config.UIUsername = config.UIUsername
	}
	if config.UIPassword != "" {
		s.config.UIPassword = config.UIPassword
	}
}
