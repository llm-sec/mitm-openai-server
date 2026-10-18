package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// IsURLSafe 检查URL是否安全，防止SSRF和递归调用
func IsURLSafe(targetURL string, serverAddr string) error {
	// 解析目标URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("无效的URL格式: %v", err)
	}

	// 无scheme的相对路径（如 "not-a-valid-url"）会被解析成空Host，必须拒绝
	if parsedURL.Host == "" {
		return fmt.Errorf("无效的URL格式: 缺少主机名")
	}

	// 检查是否为本地回环地址
	if isLoopbackAddress(parsedURL.Host) {
		return fmt.Errorf("不允许使用本地回环地址")
	}

	// 检查是否为私有网络地址
	if isPrivateNetwork(parsedURL.Host) {
		return fmt.Errorf("不允许使用私有网络地址")
	}

	// 检查是否为云服务元数据地址
	if isCloudMetadataAddress(parsedURL.Host) {
		return fmt.Errorf("不允许使用云服务元数据地址")
	}

	// 检查是否与当前服务器地址相同（防止递归调用）
	if isSameAddress(parsedURL.Host, serverAddr) {
		return fmt.Errorf("目标地址不能与当前服务器地址相同，防止递归调用")
	}

	return nil
}

// stripPort 从主机地址中提取主机名部分，兼容IPv4、IPv6（含方括号）和域名
// "127.0.0.1:8080" -> "127.0.0.1"；"[::1]:8080" -> "::1"；"::1" -> "::1"；"::1:8080" -> "::1"；"localhost" -> "localhost"
func stripPort(host string) string {
	// SplitHostPort 可以正确处理 "host:port" 和 "[::1]:8080" 两种形式
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	// 不带方括号的IPv6+端口（如 "::1:8080"）SplitHostPort 会解析失败，
	// 尝试按最后一个冒号拆分：前半部分是合法IP且后半部分是数字时，视为IPv6+端口
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		hostPart := host[:idx]
		portPart := strings.Trim(host[idx+1:], "[]")
		if net.ParseIP(hostPart) != nil && isNumericPort(portPart) {
			return hostPart
		}
	}
	// 剩下的直接去掉方括号即可
	return strings.Trim(host, "[]")
}

// isNumericPort 判断字符串是否为纯数字端口
func isNumericPort(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isLoopbackAddress 检查是否为回环地址
func isLoopbackAddress(host string) bool {
	hostWithoutPort := stripPort(host)

	// 检查IPv4回环地址
	if hostWithoutPort == "127.0.0.1" || hostWithoutPort == "localhost" {
		return true
	}

	// 检查IPv6回环地址
	if hostWithoutPort == "::1" {
		return true
	}

	// 解析IP地址并检查是否为回环地址
	if ip := net.ParseIP(hostWithoutPort); ip != nil && ip.IsLoopback() {
		return true
	}

	return false
}

// isPrivateNetwork 检查是否为私有网络地址
func isPrivateNetwork(host string) bool {
	hostWithoutPort := stripPort(host)
	
	// 解析IP地址
	ip := net.ParseIP(hostWithoutPort)
	if ip == nil {
		// 如果不是IP地址，尝试解析域名
		ips, err := net.LookupIP(hostWithoutPort)
		if err != nil || len(ips) == 0 {
			return false
		}
		ip = ips[0]
	}
	
	// 检查是否为私有网络地址
	privateNetworks := []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"127.0.0.0/8",    // Loopback
		"169.254.0.0/16", // Link-local
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local address
		"fe80::/10",      // IPv6 link-local
	}
	
	for _, network := range privateNetworks {
		_, ipNet, _ := net.ParseCIDR(network)
		if ipNet.Contains(ip) {
			return true
		}
	}
	
	return false
}

// isCloudMetadataAddress 检查是否为云服务元数据地址
func isCloudMetadataAddress(host string) bool {
	hostWithoutPort := strings.ToLower(stripPort(host))
	
	// 常见的云服务元数据地址
	cloudMetadataHosts := []string{
		"169.254.169.254", // AWS, Google Cloud
		"169.254.0.1",     // OpenStack
		"metadata",        // AWS (hostname)
		"metadata.google.internal", // Google Cloud (hostname)
		"100.100.100.200", // Alibaba Cloud
	}
	
	for _, metadataHost := range cloudMetadataHosts {
		if hostWithoutPort == metadataHost || strings.HasSuffix(hostWithoutPort, "."+metadataHost) {
			return true
		}
	}
	
	return false
}

// isSameAddress 检查目标地址是否与服务器地址相同
func isSameAddress(targetHost, serverAddr string) bool {
	// 如果服务器地址为空，则无法比较
	if serverAddr == "" {
		return false
	}
	
	// 解析目标主机和服务器地址
	targetHostWithoutPort := stripPort(targetHost)
	serverHostWithoutPort := stripPort(serverAddr)
	
	// 直接比较主机名
	if targetHostWithoutPort == serverHostWithoutPort {
		return true
	}
	
	// 解析IP地址进行比较
	targetIPs, err1 := net.LookupIP(targetHostWithoutPort)
	serverIPs, err2 := net.LookupIP(serverHostWithoutPort)
	
	// 如果解析失败，无法比较
	if err1 != nil || err2 != nil {
		return false
	}
	
	// 比较IP地址
	for _, targetIP := range targetIPs {
		for _, serverIP := range serverIPs {
			if targetIP.Equal(serverIP) {
				return true
			}
		}
	}
	
	return false
}