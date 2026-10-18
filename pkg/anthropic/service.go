package anthropic

// NewService 根据配置创建适当的Anthropic服务
// 根据ProxyMode配置，返回代理服务或模拟服务
func NewService(config Config) Service {
	if config.ProxyMode && config.TargetURL != "" {
		// 使用代理模式
		return ProxyServiceCreator(config)
	}

	// 使用模拟模式
	return MockServiceCreator(config)
}
