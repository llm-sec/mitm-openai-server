package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/llm-sec/mitm-openai-server/pkg/server"
	"github.com/llm-sec/mitm-openai-server/pkg/storage"
	"github.com/spf13/cobra"
)

var (
	// API服务器特定的标志
	port               int
	enableAuth         bool
	authUsername       string
	authPassword       string
	authToken          string
	authType           string
	enableCors         bool
	corsAllowedOrigins string

	// 前端UI相关标志
	uiUsername     string
	uiPassword     string
	generateUIAuth bool
	uiDir          string

	// 中间人代理相关标志
	proxyMode      bool
	targetURL      string
	targetAuthType string
	targetUsername string
	targetPassword string
	targetToken    string

	// 存储相关标志
	useSQL     bool
	sqlitePath string

	// TLS/HTTPS相关标志
	enableTLS   bool
	autoTLS     bool
	tlsCertFile string
	tlsKeyFile  string

	// Anthropic（Claude Code协议）相关标志
	anthropicTargetURL      string
	anthropicTargetAuthType string
	anthropicAPIKey         string
	anthropicAuthToken      string
	anthropicModelsFile     string
)

// serverCmd 表示server子命令
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动API服务器",
	Long: `启动一个中间人OpenAPI服务器，用于接收、转发和记录请求。

例如:
  # 启动独立模式服务器（返回模拟数据）
  mitm-openai-server server --port 8080 --data ./data
  
  # API认证默认开启，使用basic认证方式和--auth-username/--auth-password参数提供的凭据
  # 而UI认证使用login.json中存储的凭据（默认用户名root，初次运行自动生成密码）
  # 如需禁用API认证，请使用 --enable-auth=false
  
  # 独立模式（默认带认证）
  mitm-openai-server server --auth-username admin --auth-password secret
  
  # 代理模式（转发到真实API）
  mitm-openai-server server --proxy-mode --target-url https://api.example.com
  
  # 带认证的代理模式
  mitm-openai-server server --proxy-mode --target-url https://api.example.com --target-auth-type basic --target-username user --target-password pass

  # HTTPS模式（自动生成自签名证书，捕获TLS ClientHello指纹）
  mitm-openai-server server --auto-tls

  # HTTPS模式（使用自定义证书）
  mitm-openai-server server --tls --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem

  # Claude Code协议（Anthropic）支持：默认使用内置模拟服务，无需任何配置即可连接
  # Claude Code 配置: ANTHROPIC_BASE_URL=http://localhost:8080 ANTHROPIC_AUTH_TOKEN=任意值

  # 代理模式（转发Claude Code请求到真实Anthropic API）
  mitm-openai-server server --anthropic-target-url https://api.anthropic.com --anthropic-api-key sk-ant-xxx`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// API服务器特定的标志
	serverCmd.Flags().IntVar(&port, "port", 8080, "服务器监听端口")
	serverCmd.Flags().BoolVar(&enableAuth, "enable-auth", true, "启用API认证")
	serverCmd.Flags().StringVar(&authType, "auth-type", "basic", "认证类型 (basic 或 token)")
	serverCmd.Flags().StringVar(&authUsername, "auth-username", "admin", "API Basic认证用户名")
	serverCmd.Flags().StringVar(&authPassword, "auth-password", "", "API Basic认证密码")
	serverCmd.Flags().StringVar(&authToken, "auth-token", "", "API Token认证令牌")
	serverCmd.Flags().BoolVar(&enableCors, "enable-cors", true, "启用CORS")
	serverCmd.Flags().StringVar(&corsAllowedOrigins, "cors-allowed-origins", "*", "允许的源，用于CORS")

	// 前端UI相关标志
	serverCmd.Flags().BoolVar(&generateUIAuth, "generate-ui-auth", true, "生成随机UI认证凭证")
	serverCmd.Flags().StringVar(&uiUsername, "ui-username", "root", "前端UI用户名")
	serverCmd.Flags().StringVar(&uiPassword, "ui-password", "", "前端UI密码 (留空则自动生成)")
	serverCmd.Flags().StringVar(&uiDir, "ui-dir", "./react-ui/dist", "前端UI文件目录")

	// 中间人代理相关标志
	serverCmd.Flags().BoolVar(&proxyMode, "proxy-mode", false, "启用代理模式（转发到真实API）")
	serverCmd.Flags().StringVar(&targetURL, "target-url", "", "目标API服务器地址")
	serverCmd.Flags().StringVar(&targetAuthType, "target-auth-type", "none", "目标API认证类型 (none, basic, token)")
	serverCmd.Flags().StringVar(&targetUsername, "target-username", "", "目标API基本认证用户名")
	serverCmd.Flags().StringVar(&targetPassword, "target-password", "", "目标API基本认证密码")
	serverCmd.Flags().StringVar(&targetToken, "target-token", "", "目标API令牌")

	// 存储相关标志
	serverCmd.Flags().BoolVar(&useSQL, "use-sql", false, "启用SQLite存储")
	serverCmd.Flags().StringVar(&sqlitePath, "sqlite-path", "", "SQLite数据库路径")

	// TLS/HTTPS相关标志
	serverCmd.Flags().BoolVar(&enableTLS, "tls", false, "启用HTTPS（需提供证书，或用 --auto-tls 自动生成）")
	serverCmd.Flags().BoolVar(&autoTLS, "auto-tls", false, "自动生成自签名证书并启用HTTPS")
	serverCmd.Flags().StringVar(&tlsCertFile, "tls-cert", "", "TLS证书文件路径（PEM格式）")
	serverCmd.Flags().StringVar(&tlsKeyFile, "tls-key", "", "TLS私钥文件路径（PEM格式）")

	// Anthropic（Claude Code协议）相关标志
	serverCmd.Flags().StringVar(&anthropicTargetURL, "anthropic-target-url", "", "目标Anthropic API地址（配置后启用Claude Code代理模式，默认使用内置模拟服务）")
	serverCmd.Flags().StringVar(&anthropicTargetAuthType, "anthropic-target-auth-type", "none", "目标Anthropic API认证类型 (none, api_key, bearer)")
	serverCmd.Flags().StringVar(&anthropicAPIKey, "anthropic-api-key", "", "目标Anthropic API密钥（x-api-key）")
	serverCmd.Flags().StringVar(&anthropicAuthToken, "anthropic-auth-token", "", "目标Anthropic Bearer Token（Authorization: Bearer）")
	serverCmd.Flags().StringVar(&anthropicModelsFile, "anthropic-models-file", "", "Anthropic模型列表文件路径（mock模式）")
}

func runServer() {
	// 设置Gin为发布模式，减少控制台输出
	gin.SetMode(gin.ReleaseMode)

	// 确定最终使用的数据目录
	finalDataDir := dataDir
	if finalDataDir == "" {
		// 如果没有指定路径，使用当前目录下的data文件夹
		currentDir, err := os.Getwd()
		if err == nil {
			finalDataDir = filepath.Join(currentDir, "data")
		} else {
			finalDataDir = "./data"
		}
	}

	// 确保数据目录存在
	absDataDir, err := filepath.Abs(finalDataDir)
	if err != nil {
		log.Fatalf("无法获取数据目录的绝对路径: %v", err)
	}

	if err := os.MkdirAll(absDataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 确保UI目录存在
	absUIDir, err := filepath.Abs(uiDir)
	if err != nil {
		log.Fatalf("无法获取UI目录的绝对路径: %v", err)
	}

	if err := ensureUIDirectory(absUIDir); err != nil {
		log.Fatalf("UI目录准备失败: %v", err)
	}

	if verbose {
		log.Printf("数据将存储在: %s", absDataDir)
		log.Printf("UI文件目录: %s", absUIDir)
	}

	// 强制使用SQLite存储（用户隔离需要数据库支持）
	if verbose {
		log.Printf("使用SQLite数据库存储，数据库路径: %s/mitm_server.db", absDataDir)
	}

	storageImpl, storageErr := storage.NewSQLiteStorage(absDataDir)
	if storageErr != nil {
		log.Fatalf("初始化SQLite存储失败: %v", storageErr)
	}

	// 创建服务器配置
	config := server.ServerConfig{
		Storage:        storageImpl,
		EnableAuth:     enableAuth,
		AuthType:       authType,
		Username:       authUsername,
		Password:       authPassword,
		Token:          authToken,
		EnableCORS:     enableCors,
		AllowOrigins:   corsAllowedOrigins,
		UIUsername:     uiUsername,
		UIPassword:     uiPassword,
		GenerateUIAuth: generateUIAuth,
		UIDir:          absUIDir,

		// 中间人代理相关配置
		ProxyMode:      proxyMode,
		TargetURL:      targetURL,
		TargetAuthType: targetAuthType,
		TargetUsername: targetUsername,
		TargetPassword: targetPassword,
		TargetToken:    targetToken,

		// TLS/HTTPS相关配置
		EnableTLS:   enableTLS,
		AutoTLS:     autoTLS,
		TLSCertFile: tlsCertFile,
		TLSKeyFile:  tlsKeyFile,

		// Anthropic（Claude Code协议）相关配置
		AnthropicTargetURL:      anthropicTargetURL,
		AnthropicTargetAuthType: anthropicTargetAuthType,
		AnthropicAPIKey:         anthropicAPIKey,
		AnthropicAuthToken:      anthropicAuthToken,
		AnthropicModelsFile:     anthropicModelsFile,
	}

	// 创建并启动服务器
	apiServer := server.NewServerWithConfig(config)

	// 打印服务器信息(带颜色)
	addr := fmt.Sprintf(":%d", port)

	// 定义颜色代码
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorGreen  = "\033[32m"
		colorYellow = "\033[33m"
		colorBlue   = "\033[34m"
		colorPurple = "\033[35m"
		colorCyan   = "\033[36m"
		colorWhite  = "\033[37m"
		colorBold   = "\033[1m"
	)

	// 获取UI配置
	serverConfig := apiServer.GetConfig()

	// 精简登录信息显示 - 数据库版本
	fmt.Println()
	fmt.Printf("%s┌─────────────────────────────────────────────────┐%s\n", colorGreen+colorBold, colorReset)
	fmt.Printf("%s│%s%s            MITM OpenAI Server 已启动            %s%s│%s\n", colorGreen+colorBold, colorReset, colorWhite+colorBold, colorReset, colorGreen+colorBold, colorReset)
	fmt.Printf("%s│%s%s              (多用户数据库版本)                %s%s│%s\n", colorGreen+colorBold, colorReset, colorCyan, colorReset, colorGreen+colorBold, colorReset)
	fmt.Printf("%s└─────────────────────────────────────────────────┘%s\n", colorGreen+colorBold, colorReset)
	fmt.Printf("登录地址: %s%shttp://localhost%s/ui/login%s\n", colorBlue+colorBold, addr, colorReset, colorReset)
	fmt.Printf("用户名:   %s%s%s\n", colorYellow+colorBold, serverConfig.UIUsername, colorReset)
	fmt.Printf("密码:     %s%s%s\n", colorYellow+colorBold, serverConfig.UIPassword, colorReset)
	fmt.Printf("数据库:   %s%s/mitm_server.db%s\n", colorCyan, absDataDir, colorReset)
	fmt.Printf("存储模式: %s多用户隔离 (SQLite)%s\n", colorGreen+colorBold, colorReset)
	fmt.Println()
	fmt.Printf("%s请使用上述凭据登录系统，监控和分析OpenAI API请求。%s\n", colorWhite, colorReset)
	fmt.Printf("%s每个用户的数据完全隔离，支持多用户同时使用。%s\n", colorGreen, colorReset)
	fmt.Println()

	// 启动服务器
	if err := apiServer.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

// 确保UI目录存在
// 如果目录不存在则创建
// 参数:
//   - uiDir: UI目录路径
//
// 返回:
//   - error: 如果发生错误，返回错误信息
func ensureUIDirectory(uiDir string) error {
	// 检查主目录是否存在
	stat, err := os.Stat(uiDir)
	if os.IsNotExist(err) {
		log.Printf("创建UI目录: %s", uiDir)
		if err := os.MkdirAll(uiDir, 0755); err != nil {
			return fmt.Errorf("创建UI目录失败: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("检查UI目录失败: %v", err)
	} else if !stat.IsDir() {
		return fmt.Errorf("UI路径已存在但不是目录: %s", uiDir)
	}

	return nil
}
