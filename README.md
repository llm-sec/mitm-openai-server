# 中间人OpenAPI服务器

这是一个中间人OpenAPI服务器，用于接收、转发和记录API请求数据。服务器可以工作在两种模式下：
1. 独立模式：返回模拟的数据响应（类似假的OpenAPI服务器）
2. 代理模式：将请求转发到真实的API服务，同时记录请求和响应

## 功能特点

- 独立模式：提供模拟的API响应
- 代理模式：充当中间人，转发请求到真实API
- 完全符合OpenAPI 3.0规范
- 捕获并记录所有API请求和响应的详细信息
- 提供REST API接口查询已记录的请求
- 可配置的数据存储目录
- 支持对目标API的各种认证方式（无认证、基本认证、令牌认证）
- 包含友好的Web UI界面查看请求历史
- 模块化设计，易于扩展

## 安装

确保你已安装Go（版本1.20或更高）。

```bash
# 克隆仓库
git clone https://github.com/llm-sec/mitm-openai-server.git
cd mitm-openai-server

# 安装依赖
go mod tidy

# 构建
go build
```

## 运行服务器

### 独立模式（模拟响应）

```bash
# 使用默认设置运行
./mitm-openai-server server

# 指定端口和数据目录
./mitm-openai-server server --port 9000 --data ./my-data
```

### 代理模式（转发到真实API）

```bash
# 基本代理模式
./mitm-openai-server server --proxy-mode --target-url https://api.example.com

# 带认证的代理模式
./mitm-openai-server server --proxy-mode --target-url https://api.example.com \
  --target-auth-type basic --target-username user --target-password pass

# 使用令牌认证
./mitm-openai-server server --proxy-mode --target-url https://api.example.com \
  --target-auth-type token --target-token your-api-token
```

### HTTPS模式（捕获TLS ClientHello）

服务器可以HTTPS模式启动，自动窥探并记录每个请求的 **TLS ClientHello** 握手信息（TLS版本、SNI、密码套件、扩展列表、ALPN等），并计算 **JA3 / JA4 指纹**，用于客户端指纹识别与基于SNI的数据外泄检测。

```bash
# 自动生成自签名证书并启用HTTPS
./mitm-openai-server server --auto-tls

# 使用自定义证书启用HTTPS
./mitm-openai-server server --tls --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

TLS相关标志：

- `--tls` - 启用HTTPS（需配合 `--tls-cert` / `--tls-key` 提供证书）
- `--auto-tls` - 自动生成自签名证书并启用HTTPS（证书保存在UI目录下的 `cert.pem` / `key.pem`）
- `--tls-cert <path>` - TLS证书文件路径（PEM格式）
- `--tls-key <path>` - TLS私钥文件路径（PEM格式）

捕获到的ClientHello信息会作为请求元数据（`metadata.tls_client_hello`）保存，包含：JA3指纹、JA4指纹、TLS版本、SNI、密码套件、扩展、支持的群组、签名算法、ALPN协议等。在Web界面的请求详情页可通过 **"TLS 握手"** 标签页查看。

### Claude Code协议支持（Anthropic Messages API）

服务器原生支持 **Claude Code**（Anthropic Messages API）协议，与OpenAI API的支持方式一致：默认使用内置模拟服务，无需任何配置即可让 Claude Code 连接；也可以配置代理模式转发到真实的 Anthropic API。

**开箱即用（模拟模式）** - 启动服务器后，将 Claude Code 的 API 地址指向本服务器即可：

```bash
# 启动服务器（默认内置Claude模拟服务）
./mitm-openai-server server --port 8080

# 在另一个终端配置Claude Code连接本服务器
ANTHROPIC_BASE_URL=http://127.0.0.1:8080 ANTHROPIC_AUTH_TOKEN=sk-ant-test claude
```

说明：模拟模式**不校验**客户端密钥，`ANTHROPIC_AUTH_TOKEN` 填写任意值即可连接（与 Claude 官方SDK的API密钥格式兼容，便于平滑切换）。

**代理模式（转发到真实Anthropic API）**：

```bash
# 使用 x-api-key 认证
./mitm-openai-server server --anthropic-target-url https://api.anthropic.com --anthropic-api-key sk-ant-xxxxx

# 使用 Bearer Token 认证
./mitm-openai-server server --anthropic-target-url https://api.anthropic.com --anthropic-auth-token sk-ant-xxxxx

# 自定义模型列表文件
./mitm-openai-server server --anthropic-models-file ./models.json
```

Anthropic相关标志：

- `--anthropic-target-url <url>` - 目标Anthropic API地址（配置后启用代理模式；默认使用内置模拟服务）
- `--anthropic-target-auth-type <type>` - 目标认证类型（`none` / `api_key` / `bearer`）
- `--anthropic-api-key <key>` - 目标API密钥（通过 `x-api-key` 请求头发送）
- `--anthropic-auth-token <token>` - 目标Bearer Token（通过 `Authorization: Bearer` 请求头发送）
- `--anthropic-models-file <path>` - 模型列表文件（JSON，支持字符串数组或模型对象数组，仅模拟模式生效）

**支持的Anthropic端点**：

- `POST /v1/messages` - 聊天消息（支持非流式与SSE流式，Claude Code默认使用流式）
- `POST /v1/messages/count_tokens` - Token数量统计
- `GET /v1/models` - 模型列表（Anthropic格式）

**协议自动识别**：`GET /v1/models` 同时被OpenAI与Anthropic使用但返回格式不同，服务器根据请求头自动分派——带有 `anthropic-version` 请求头、或 `x-api-key` / `Authorization: Bearer` 以 `sk-ant-` 开头的请求按Anthropic格式处理，其余按OpenAI格式处理。所有请求都会在记录中标记 `metadata.protocol`（`openai` / `anthropic`），便于在Web界面区分。

## 管理请求记录

```bash
# 列出所有记录的请求
./mitm-openai-server requests list

# 查看特定ID的请求详情
./mitm-openai-server requests get <request-id>

# 删除特定ID的请求
./mitm-openai-server requests delete <request-id>

# 删除所有请求
./mitm-openai-server requests delete --all
```

## API端点

- `GET /health` - 健康检查
- `GET /openapi.json`, `GET /v1/openapi.json`, `GET /v2/openapi.json` - OpenAPI规范
- `GET /ui/api/requests` - 获取所有记录的请求（需登录UI）
- `GET /ui/api/requests/:id` - 获取特定ID的请求
- `DELETE /ui/api/requests/:id` - 删除特定请求
- `DELETE /ui/api/requests` - 删除所有请求
- `ANY /v1/*path`, `ANY /v2/*path` - 捕获所有API请求并记录
- `GET /ui/` - Web界面查看请求历史

## Web UI界面

访问 http://localhost:8080/ui/ 查看请求历史的Web界面。默认情况下，会自动生成UI访问凭证。

## 数据存储

请求数据存储在SQLite数据库中（默认位于数据目录下的 `mitm_server.db`），并通过 `--data` 标志指定数据目录。

## 许可证

MIT

## 作者

LLM-SEC团队 