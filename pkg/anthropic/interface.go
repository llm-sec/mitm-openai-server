package anthropic

// Service 定义Anthropic（Claude）API服务接口
// 与openai.Service保持同样的签名，便于server包统一处理
type Service interface {
	// Name 返回服务名称
	Name() string

	// HandleRequest 处理API请求并返回模拟或代理响应
	// method: HTTP方法(GET, POST等)
	// path: 请求路径（不含/v1前缀）
	// headers: 请求头
	// queryParams: 查询参数
	// body: 请求体(字节数组)
	// 返回值: 状态码，响应头，响应体，错误信息
	// 注意：对于流式响应，responseData 会是一个 io.Reader，
	//       调用方需要将其直接透传给客户端
	HandleRequest(method, path string, headers, queryParams map[string]string, body []byte) (int, map[string]string, interface{}, error)
}

// 服务创建工厂函数类型定义
type ServiceCreator func(config Config) Service

// MockServiceCreator 创建Mock服务的函数
var MockServiceCreator ServiceCreator

// ProxyServiceCreator 创建代理服务的函数
var ProxyServiceCreator ServiceCreator
