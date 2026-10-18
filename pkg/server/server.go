package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/llm-sec/mitm-openai-server/pkg/anthropic"
	"github.com/llm-sec/mitm-openai-server/pkg/api"
	"github.com/llm-sec/mitm-openai-server/pkg/embed"
	"github.com/llm-sec/mitm-openai-server/pkg/openai"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
	"github.com/llm-sec/mitm-openai-server/pkg/tlsprobe"
	"github.com/llm-sec/mitm-openai-server/pkg/utils"
)

// Server 表示API服务器
// 是服务器的核心结构，包含路由引擎、存储接口和配置
type Server struct {
	router              *gin.Engine             // Gin路由引擎
	storage             storage.Storage         // 存储接口
	config              api.ServerConfig        // 服务器配置
	openaiService       openai.Service          // OpenAI服务接口
	anthropicService    anthropic.Service       // Anthropic服务接口
	configManager       *ConfigManager          // 配置管理器
	uiServer            api.UIServerInterface   // UI服务器接口
	openaiHandler       interface{}             // OpenAI API处理器，使用openai.Handler
	anthropicHandler    interface{}             // Anthropic API处理器，使用anthropic.Handler
	storagePath         string                  // 用户配置的数据存储路径
	SuppressStartupInfo bool                    // 控制是否输出启动信息
	tlsProbe            *tlsprobe.ProbeListener // TLS ClientHello 探测监听器（启用HTTPS时非nil）
}

// NewServer 创建一个新的服务器实例
// 使用默认配置创建服务器，只需提供存储接口
// 参数:
//   - storage: 用于存储请求数据的存储接口
//
// 返回:
//   - *Server: 服务器实例
func NewServer(storage storage.Storage) *Server {
	return NewServerWithConfig(api.ServerConfig{
		Storage: storage,
	})
}

// NewServerWithConfig 使用给定的配置创建服务器
// 使用指定的配置创建和初始化Server实例
// 参数:
//   - config: 服务器配置
//
// 返回:
//   - *Server: 初始化的服务器实例
func NewServerWithConfig(config api.ServerConfig) *Server {
	// 如果未设置存储，则返回错误
	if config.Storage == nil {
		log.Fatal("必须提供存储实例")
	}

	// 创建配置管理器
	configManager, err := NewConfigManager()
	if err != nil {
		fmt.Printf("警告: 无法创建配置管理器: %v\n", err)
	}

	// 创建默认路由
	router := gin.Default()

	// 创建服务器实例
	server := &Server{
		router:        router,
		storage:       config.Storage,
		config:        config,
		configManager: configManager,
	}

	// 如果配置管理器创建成功，尝试加载配置
	if configManager != nil {
		userConfig, err := configManager.LoadConfig()
		if err != nil {
			fmt.Printf("警告: 无法加载用户配置: %v\n", err)
		} else {
			// 应用用户配置
			configManager.ApplyConfig(userConfig, server)

			// 如果成功加载了密码，跳过自动生成
			if userConfig.UIPassword != "" {
				config.GenerateUIAuth = false
			}
		}
	}

	// 密码处理逻辑 - 简化，避免无限循环
	if server.configManager != nil && server.config.UIPassword == "" && config.GenerateUIAuth {
		// 只有在密码为空且启用了生成功能时才生成
		username := server.config.UIUsername
		if username == "" {
			username = "root" // 默认用户名
			server.config.UIUsername = username
		}

		// 生成随机密码
		newPassword := utils.GenerateRandomPassword(12)
		server.config.UIPassword = newPassword

		// 创建login.json文件
		loginConfig := struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}{
			Username: username,
			Password: newPassword,
		}

		// 序列化为JSON
		data, err := json.MarshalIndent(loginConfig, "", "  ")
		if err == nil {
			// 写入login.json文件
			err = os.WriteFile("login.json", data, 0644)
			if err != nil {
				fmt.Printf("警告: 无法写入login.json文件: %v\n", err)
			}
		}
	}

	// 初始化OpenAI服务
	if config.ProxyMode {
		// 创建OpenAI服务配置
		openaiConfig := openai.Config{
			Enabled:         true,
			ResponseDelayMs: 0,     // 在代理模式下不添加额外延迟
			APIKeyAuth:      false, // 不在服务器端验证API密钥
			ProxyMode:       true,
			TargetURL:       config.TargetURL,
			TargetAuthType:  config.TargetAuthType,
			TargetUsername:  config.TargetUsername,
			TargetPassword:  config.TargetPassword,
			TargetToken:     config.TargetToken,
		}
		server.openaiService = openai.NewService(openaiConfig)
	} else if !config.ProxyMode && config.EnableAuth {
		// 创建带有API密钥验证的模拟服务
		openaiConfig := openai.Config{
			Enabled:         true,
			ResponseDelayMs: 100, // 添加一些响应延迟以模拟真实API
			APIKeyAuth:      true,
			APIKey:          config.Token, // 使用同一个token作为API密钥
			ProxyMode:       false,
		}
		server.openaiService = openai.NewService(openaiConfig)
	} else {
		// 创建不需要验证的模拟服务
		openaiConfig := openai.DefaultConfig()
		openaiConfig.APIKeyAuth = false
		server.openaiService = openai.NewService(openaiConfig)
	}

	// 初始化OpenAI全局处理器，用于外部保存请求
	if server.openaiService != nil {
		openai.InitGlobalHandler(config.Storage, server.openaiService)
	}

	// 创建UI服务器
	server.uiServer = api.UIServerFactory(config.Storage, config, server.openaiService)

	// 创建OpenAI处理器 - 直接使用openai包中的Handler
	openaiHandler := openai.NewHandler(config.Storage, server.openaiService)
	server.openaiHandler = openaiHandler

	// 初始化Anthropic（Claude Code协议）服务
	server.anthropicService = newAnthropicService(config)
	if server.anthropicService != nil {
		// 创建Anthropic处理器
		anthropicHandler := anthropic.NewHandler(config.Storage, server.anthropicService)
		server.anthropicHandler = anthropicHandler
	}

	// 确保UI服务器使用最新密码
	if uiServer, ok := server.uiServer.(*api.UIServer); ok {
		if server.config.UIPassword != "" && len(server.config.UIPassword) > 0 {
			uiServer.SetConfig(server.config)
		}
	}

	// 设置路由
	server.setupRoutes()
	server.setupOpenAIRoutes()        // 设置OpenAI相关路由
	server.setupAnthropicRoutes()     // 设置Anthropic相关路由
	server.setupModelsDispatchRoute() // 设置/v1/models统一分派路由

	return server
}

// newAnthropicService 根据配置创建Anthropic服务
// 配置了目标URL则使用代理模式，否则使用内置模拟服务（Claude Code开箱即用）
func newAnthropicService(config api.ServerConfig) anthropic.Service {
	anthropicConfig := anthropic.DefaultConfig()

	if config.AnthropicTargetURL != "" {
		// 代理模式
		anthropicConfig.ProxyMode = true
		anthropicConfig.TargetURL = config.AnthropicTargetURL
		anthropicConfig.TargetAuthType = config.AnthropicTargetAuthType
		anthropicConfig.TargetAPIKey = config.AnthropicAPIKey
		anthropicConfig.TargetToken = config.AnthropicAuthToken
		anthropicConfig.ModelsFile = config.AnthropicModelsFile
	} else {
		// 模拟模式（默认，Claude Code无需任何配置即可连接）
		anthropicConfig.ProxyMode = false
		anthropicConfig.ModelsFile = config.AnthropicModelsFile
	}

	return anthropic.NewService(anthropicConfig)
}

// Run 启动服务器
// 在指定地址上启动HTTP服务器
// 参数:
//   - addr: 服务器监听地址，格式为"host:port"
//
// 返回:
//   - error: 如果服务器启动失败，返回错误
func (s *Server) Run(addr string) error {
	// 不再调用ensurePassword方法，避免循环
	// s.ensurePassword()

	// 只有在SuppressStartupInfo为false时才输出启动信息
	if !s.SuppressStartupInfo {
		fmt.Println("\n┌─────────────────────────────────────────────────┐")
		fmt.Println("│            MITM OpenAI Server 已启动            │")
		fmt.Println("└─────────────────────────────────────────────────┘")

		port := addr[1:] // 去掉":"
		scheme := "http"
		if s.config.EnableTLS || s.config.AutoTLS {
			scheme = "https"
		}
		fmt.Printf("登录地址: %s://localhost:%s/ui/login\n", scheme, port)
		fmt.Printf("用户名:   %s\n", s.config.UIUsername)

		// 显示真实密码
		if s.config.UIPassword != "" {
			fmt.Printf("密码:     %s\n", s.config.UIPassword)
		} else {
			fmt.Printf("密码:     [未设置]\n")
		}

		fmt.Println("\n请使用上述凭据登录系统，监控和分析OpenAI API请求。")
	}

	// 获取底层TCP监听器
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听地址 %s 失败: %w", addr, err)
	}

	// 如果启用了HTTPS，包装监听器以探测TLS ClientHello并加载证书
	if s.config.EnableTLS || s.config.AutoTLS {
		// 准备TLS证书
		certFile, keyFile := s.config.TLSCertFile, s.config.TLSKeyFile
		if s.config.AutoTLS {
			// 自动生成自签名证书
			certDir := "."
			if s.config.UIDir != "" {
				certDir = s.config.UIDir
			}
			certFile, keyFile = filepath.Join(certDir, "cert.pem"), filepath.Join(certDir, "key.pem")
			if _, statErr := os.Stat(certFile); os.IsNotExist(statErr) {
				certFile, keyFile, err = tlsprobe.GenerateSelfSignedCert(certDir)
				if err != nil {
					listener.Close()
					return fmt.Errorf("自动生成自签名证书失败: %w", err)
				}
				if !s.SuppressStartupInfo {
					fmt.Printf("已生成自签名证书: %s\n", certFile)
				}
			}
		}
		if certFile == "" || keyFile == "" {
			listener.Close()
			return fmt.Errorf("启用HTTPS需要提供证书: 使用 --tls-cert/--tls-key 或 --auto-tls 自动生成")
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			listener.Close()
			return fmt.Errorf("加载TLS证书失败: %w", err)
		}

		// 创建ProbeListener窥探ClientHello，再包一层TLS
		probeListener := tlsprobe.NewProbeListener(listener, 5*time.Second)
		s.tlsProbe = probeListener

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS10,
		}
		tlsListener := tls.NewListener(probeListener, tlsConfig)

		// 启动HTTPS服务器
		srv := &http.Server{
			Handler: s.router,
		}
		return srv.Serve(tlsListener)
	}

	// 普通HTTP模式，复用已创建的监听器
	// 注意：不能使用 s.router.Run(addr)，那会再次bind同一端口导致 "address already in use"
	srv := &http.Server{
		Handler: s.router,
	}
	return srv.Serve(listener)
}

// ensurePassword 确保系统有有效的密码
// 如果没有密码，则生成一个随机密码并保存
func (s *Server) ensurePassword() {
	// 如果已经有密码，则不需要再生成
	if s.config.UIPassword != "" && len(s.config.UIPassword) > 0 {
		fmt.Printf("已有密码: •••••••• (长度: %d)\n", len(s.config.UIPassword))
		return
	}

	// 生成新密码（不再尝试从配置文件加载）
	password := utils.GenerateRandomPassword(12)
	s.config.UIPassword = password
	fmt.Printf("已生成新密码 (长度: %d)\n", len(password))

	// 如果配置管理器可用，保存密码
	if s.configManager != nil {
		// 直接保存密码到LoginFile
		loginConfig := struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}{
			Username: s.config.UIUsername,
			Password: password,
		}

		// 序列化为JSON
		data, err := json.MarshalIndent(loginConfig, "", "  ")
		if err == nil {
			// 写入login.json文件
			err = os.WriteFile("login.json", data, 0644)
			if err != nil {
				fmt.Printf("警告: 无法写入login.json文件: %v\n", err)
			} else {
				fmt.Printf("成功创建login.json文件，密码已保存\n")
			}
		}
	}
}

// authMiddleware 认证中间件
// 提供API认证功能，支持basic和token两种认证方式
// 返回:
//   - gin.HandlerFunc: Gin中间件函数
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果认证被禁用，则跳过
		if !s.config.EnableAuth {
			c.Next()
			return
		}

		// 根据配置的认证类型进行认证
		var authorized bool
		switch s.config.AuthType {
		case "basic": // 基本认证
			authorized = s.validateBasicAuth(c)
		case "token": // 令牌认证
			authorized = s.validateTokenAuth(c)
		default:
			// 不支持的认证类型，拒绝访问
			c.JSON(http.StatusUnauthorized, StandardResponse{
				Code: 10001,
				Msg:  "认证配置错误",
			})
			c.Abort()
			return
		}

		// 认证失败，返回401状态码
		if !authorized {
			c.JSON(http.StatusUnauthorized, StandardResponse{
				Code: 10002,
				Msg:  "认证失败",
			})
			c.Abort()
			return
		}

		// 认证成功，继续处理请求
		c.Next()
	}
}

// validateBasicAuth 验证基本认证
// 检查HTTP Basic认证的用户名和密码是否匹配配置
// 参数:
//   - c: Gin上下文
//
// 返回:
//   - bool: 认证是否成功
func (s *Server) validateBasicAuth(c *gin.Context) bool {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		return false // 没有提供认证信息
	}

	return username == s.config.Username && password == s.config.Password
}

// validateTokenAuth 验证令牌认证
// 检查Authorization头中的令牌是否匹配配置
// 参数:
//   - c: Gin上下文
//
// 返回:
//   - bool: 认证是否成功
func (s *Server) validateTokenAuth(c *gin.Context) bool {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false // 没有提供认证头
	}

	// 支持"Bearer token"格式和直接的令牌格式
	token := authHeader
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token = authHeader[7:] // 去掉"Bearer "前缀
	}

	// 1. 检查配置中的令牌
	if token == s.config.Token {
		return true
	}

	// 2. 检查是否是UI登录生成的令牌
	if s.uiServer != nil && s.uiServer.ValidateToken(token) {
		return true
	}

	return false
}

// corsMiddleware CORS中间件
// 处理跨域资源共享，允许来自不同域的请求
// 返回:
//   - gin.HandlerFunc: Gin中间件函数
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.config.EnableCORS {
			// 设置CORS相关的HTTP头
			c.Writer.Header().Set("Access-Control-Allow-Origin", s.config.AllowOrigins)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

			// 处理预检请求
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent) // 返回204状态码
				return
			}
		}
		c.Next()
	}
}

// setupRoutes 配置服务器的所有HTTP路由和中间件
// 设置HTTP路由处理程序，包括API路由、认证中间件和CORS中间件
func (s *Server) setupRoutes() {
	// 设置全局中间件
	s.router.Use(gin.Logger())
	s.router.Use(gin.Recovery())
	s.router.Use(s.corsMiddleware())

	// 健康检查路由
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API路由组，有认证中间件保护
	apiGroup := s.router.Group("/api")
	apiGroup.Use(s.authMiddleware())
	apiGroup.Use(s.apiMiddleware())

	// 设置UI相关路由
	s.setupUIRoutes()
}

// setupUIRoutes 设置UI相关的路由
// 处理静态文件和用户登录
func (s *Server) setupUIRoutes() {
	// 确保UI路径有效
	uiPath := s.config.UIDir
	if uiPath == "" {
		uiPath = "./ui" // 默认UI目录
	}

	// 设置UI路由
	s.uiServer.SetupUIRoutes(s.router, s.authMiddleware(), s.apiMiddleware())

	// 单独处理基本UI路由，确保index.html总是被加载
	s.router.GET("/ui", func(c *gin.Context) {
		c.File(embed.ResolvePath(uiPath, "index.html"))
	})
	s.router.GET("/ui/", func(c *gin.Context) {
		c.File(embed.ResolvePath(uiPath, "index.html"))
	})

	// 根路径重定向到UI路径，让React Router处理认证逻辑
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/")
	})

	// 处理静态文件 - 确保assets和其他静态资源能被正确访问
	// 将 /ui/assets 映射到 dist/assets 目录
	s.router.Static("/ui/assets", embed.ResolvePath(uiPath, "assets"))
	s.router.Static("/ui/static", embed.ResolvePath(uiPath, "static"))

	// 根路径下的静态文件，为了兼容性
	s.router.Static("/assets", embed.ResolvePath(uiPath, "assets"))
	s.router.Static("/static", embed.ResolvePath(uiPath, "static"))

	// 任何未处理的UI路由都重定向到index.html，实现SPA路由
	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 根路径在上面已经处理了重定向，这里排除掉
		if path == "/" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		// 检查是否是未定义的API路径
		if strings.HasPrefix(path, "/ui/api/") {
			// 对于未实现的UI API路径，返回404而不是前端页面
			c.JSON(http.StatusNotFound, gin.H{"code": 10004, "msg": "API接口不存在"})
			return
		}

		// 对于UI路径或其他非API路径，返回前端应用
		if strings.HasPrefix(path, "/ui/") ||
			(!strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/v1/")) {
			c.File(embed.ResolvePath(uiPath, "index.html"))
			return
		}

		// 其他情况返回404
		c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
	})
}

// setupOpenAIRoutes 设置OpenAI API代理路由
// 配置所有与OpenAI API相关的路由
func (s *Server) setupOpenAIRoutes() {
	// 如果没有初始化openaiHandler，跳过设置OpenAI路由
	if s.openaiHandler == nil {
		fmt.Println("警告: openaiHandler未初始化，跳过OpenAI路由设置")
		return
	}

	// 使用OpenAI处理器设置路由 - 直接使用Handler的SetupRoutes方法
	if handler, ok := s.openaiHandler.(*openai.Handler); ok {
		handler.SetupRoutes(s.router, s.apiMiddleware())
	} else {
		fmt.Println("警告: openaiHandler类型错误，无法设置OpenAI路由")
	}
}

// setupAnthropicRoutes 设置Anthropic（Claude Code协议）API代理路由
func (s *Server) setupAnthropicRoutes() {
	// 如果没有初始化anthropicHandler，跳过设置Anthropic路由
	if s.anthropicHandler == nil {
		fmt.Println("警告: anthropicHandler未初始化，跳过Anthropic路由设置")
		return
	}

	// 使用Anthropic处理器设置路由
	if handler, ok := s.anthropicHandler.(*anthropic.Handler); ok {
		handler.SetupRoutes(s.router, s.apiMiddleware())
	} else {
		fmt.Println("警告: anthropicHandler类型错误，无法设置Anthropic路由")
	}
}

// setupModelsDispatchRoute 注册/v1/models统一分派路由
// OpenAI与Anthropic的模型列表格式不同，根据请求头特征判断协议：
//   - 带 anthropic-version 请求头 -> Anthropic
//   - x-api-key 或 Authorization: Bearer 以 sk-ant- 开头 -> Anthropic
//   - 其他情况 -> OpenAI
func (s *Server) setupModelsDispatchRoute() {
	s.router.GET("/v1/models", s.apiMiddleware(), func(c *gin.Context) {
		if s.isAnthropicRequest(c) {
			if handler, ok := s.anthropicHandler.(*anthropic.Handler); ok {
				handler.HandleRequest(c)
				return
			}
			// 没有Anthropic处理器时回退到OpenAI
		}
		if handler, ok := s.openaiHandler.(*openai.Handler); ok {
			handler.HandleRequest(c)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}

// isAnthropicRequest 判断请求是否属于Anthropic协议
// 依据请求头特征判断，避免与OpenAI的/v1/models格式冲突
func (s *Server) isAnthropicRequest(c *gin.Context) bool {
	// 1. 带 anthropic-version 请求头，明确是Anthropic协议
	if c.GetHeader("anthropic-version") != "" {
		return true
	}

	// 2. x-api-key 以 sk-ant- 开头（Anthropic密钥特征）
	if key := c.GetHeader("X-Api-Key"); strings.HasPrefix(key, "sk-ant-") {
		return true
	}

	// 3. Authorization: Bearer 以 sk-ant- 开头
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer sk-ant-") {
		return true
	}

	return false
}

// detectProtocol 根据请求路径和请求头判断协议类型
// 用于在请求记录中标记 openai / anthropic，便于UI区分展示
func (s *Server) detectProtocol(method, path string, headers map[string][]string) string {
	// Anthropic协议的路径特征
	if strings.HasPrefix(path, "/v1/messages") {
		return "anthropic"
	}

	// /v1/models 根据请求头特征判断
	if path == "/v1/models" {
		for k, v := range headers {
			lowerKey := strings.ToLower(k)
			if lowerKey == "anthropic-version" && len(v) > 0 && v[0] != "" {
				return "anthropic"
			}
			if lowerKey == "x-api-key" && len(v) > 0 && strings.HasPrefix(v[0], "sk-ant-") {
				return "anthropic"
			}
			if lowerKey == "authorization" && len(v) > 0 && strings.HasPrefix(v[0], "Bearer sk-ant-") {
				return "anthropic"
			}
		}
	}

	// 默认按OpenAI处理
	return "openai"
}

// apiMiddleware API中间件
// 记录所有API请求，包括请求和响应数据
// 返回:
//   - gin.HandlerFunc: Gin中间件函数
func (s *Server) apiMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取请求开始时间
		startTime := time.Now()

		// 获取客户端IP
		clientIP := c.ClientIP()

		// 捕获请求
		requestID := uuid.New().String()
		method := c.Request.Method
		path := c.Request.URL.Path
		query := make(map[string][]string)

		// 复制URL查询参数
		for k, v := range c.Request.URL.Query() {
			query[k] = v
		}

		// 复制请求头
		headers := make(map[string][]string)
		for k, v := range c.Request.Header {
			headers[k] = v
		}

		// 读取请求体
		var bodyBytes []byte
		var err error

		if c.Request.Body != nil {
			bodyBytes, err = io.ReadAll(c.Request.Body)
			if err != nil {
				// 如果读取失败，记录错误并继续
				c.JSON(http.StatusInternalServerError, api.StandardResponse{
					Code: 10005,
					Msg:  "无法读取请求体: " + err.Error(),
				})
				c.Abort()
				return
			}

			// 恢复请求体，以便后续处理程序可以再次读取
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// 解析请求体为JSON（如果可能）
		var bodyData interface{}
		if len(bodyBytes) > 0 {
			contentType := c.GetHeader("Content-Type")
			if strings.Contains(contentType, "application/json") {
				// 尝试解析JSON
				if err := json.Unmarshal(bodyBytes, &bodyData); err != nil {
					// 如果解析失败，使用原始字符串
					bodyData = string(bodyBytes)
				}
			} else {
				// 非JSON内容，使用原始字符串
				bodyData = string(bodyBytes)
			}
		}

		// 创建一个响应记录器来捕获响应
		responseRecorder := utils.NewResponseRecorder(c.Writer)
		c.Writer = responseRecorder

		// 处理请求，这将调用下一个中间件或路由处理程序
		c.Next()

		// 计算请求处理时间
		latency := time.Since(startTime).Milliseconds()

		// 捕获响应数据
		statusCode := responseRecorder.Status()
		responseHeaders := responseRecorder.Header()
		responseBodyBytes := responseRecorder.Body.Bytes()

		// 解析响应体为JSON（如果可能）
		var responseBody interface{}
		if len(responseBodyBytes) > 0 {
			contentType := responseHeaders.Get("Content-Type")
			if strings.Contains(contentType, "application/json") {
				// 尝试解析JSON
				if err := json.Unmarshal(responseBodyBytes, &responseBody); err != nil {
					// 如果解析失败，使用原始字符串
					responseBody = string(responseBodyBytes)
				}
			} else {
				// 非JSON内容，使用原始字符串
				responseBody = string(responseBodyBytes)
			}
		}

		// 创建响应对象
		response := &storage.ProxyResponse{
			StatusCode: statusCode,
			Headers:    utils.ConvertHeaderToStringMap(responseHeaders),
			Body:       responseBody,
			Latency:    latency,
		}

		// 获取用户信息（为了支持用户隔离）
		var userID int64 = 0 // 默认用户ID为0（匿名用户）
		var username string = "api_user"

		// 尝试从上下文中获取用户信息
		if uid, exists := c.Get("user_id"); exists {
			if uname, exists := c.Get("username"); exists {
				userID = uid.(int64)
				username = uname.(string)
			}
		}

		// 创建请求记录
		request := &storage.Request{
			ID:        requestID,
			UserID:    userID,
			Method:    method,
			Path:      path,
			Timestamp: time.Now(),
			Headers:   utils.ConvertToStringMap(headers),
			Query:     utils.ConvertToStringMap(query),
			Body:      bodyData,
			ClientIP:  clientIP,
			Response:  response,
			Metadata: map[string]interface{}{
				"latency_ms": latency,
				"user_id":    userID,
				"username":   username,
				"protocol":   s.detectProtocol(method, path, headers),
			},
		}

		// 如果启用了HTTPS，附加TLS ClientHello信息（SNI、JA3/JA4指纹等）
		if s.tlsProbe != nil {
			if hello := s.tlsProbe.GetClientHello(c.Request.RemoteAddr); hello != nil {
				request.Metadata["tls_client_hello"] = tlsprobe.ClientHelloToMap(hello)
			}
		}

		// 保存请求到存储
		if err := s.storage.SaveRequest(userID, request); err != nil {
			// 记录错误，但不中断请求处理
			fmt.Printf("无法保存请求: %v\n", err)
		}
	}
}

// GetConfig 返回服务器的配置
// 获取当前服务器使用的配置信息
// 返回:
//   - ServerConfig: 服务器配置
func (s *Server) GetConfig() api.ServerConfig {
	return s.config
}

// InitServer 初始化服务器并打印配置信息（不显示密码）
// 参数:
//   - cfg: 服务器配置
//
// 返回:
//   - *Server: 初始化后的服务器
//   - error: 初始化过程中的错误
func InitServer(cfg api.ServerConfig) (*Server, error) {
	server := NewServerWithConfig(cfg)
	fmt.Printf("UI服务器配置: 用户名=%s, 密码=••••••••\n", cfg.UIUsername)
	return server, nil
}
