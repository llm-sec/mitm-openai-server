package tlsprobe

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
)

// ClientHelloJSON 是 ClientHello 的可序列化表示，用于存入请求元数据并回显到前端
type ClientHelloJSON struct {
	LegacyVersion     string   `json:"legacy_version"`      // 如 "0x0303"
	SupportedVersion  string   `json:"supported_version"`   // 如 "0x0304"
	SupportedVersionT string   `json:"supported_version_t"` // 可读版本，如 "TLS 1.3"
	JA3               string   `json:"ja3_fingerprint"`     // JA3 指纹（MD5）
	JA4               string   `json:"ja4_fingerprint"`     // JA4 指纹
	ServerName        string   `json:"server_name"`         // SNI
	ALPN              []string `json:"alpn,omitempty"`
	CipherSuites      []string `json:"cipher_suites"` // 十六进制编号，保留原始顺序
	Extensions        []string `json:"extensions"`
	SupportedGroups   []string `json:"supported_groups,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
	Random            string   `json:"random,omitempty"`
	Raw               string   `json:"-"` // 原始字节（hex），不输出到元数据避免膨胀
}

// ClientHelloToMap 将 ClientHello 转换为可序列化的 map（存储到请求元数据）
func ClientHelloToMap(ch *ClientHello) map[string]interface{} {
	if ch == nil {
		return nil
	}
	j := ClientHelloJSON{
		LegacyVersion:     formatHexID(ch.LegacyVersion),
		SupportedVersion:  formatHexID(ch.SupportedVersion),
		SupportedVersionT: tlsVersionName(ch.SupportedVersion, ch.LegacyVersion),
		JA3:               ComputeJA3(ch),
		JA4:               ComputeJA4(ch),
		ServerName:        ch.ServerName,
	}
	if len(ch.ALPNProtocols) > 0 {
		j.ALPN = ch.ALPNProtocols
	}
	for _, cs := range ch.CipherSuites {
		j.CipherSuites = append(j.CipherSuites, formatHexID(cs))
	}
	for _, ext := range ch.ExtensionTypes {
		j.Extensions = append(j.Extensions, formatHexID(ext))
	}
	for _, g := range ch.SupportedGroups {
		j.SupportedGroups = append(j.SupportedGroups, formatHexID(g))
	}
	if len(ch.SessionID) > 0 {
		j.SessionID = hex.EncodeToString(ch.SessionID)
	}
	if len(ch.Random) > 0 {
		j.Random = hex.EncodeToString(ch.Random)
	}

	// 使用json序列化后反序列化为map，保证返回值的类型与前端期望一致
	data, err := json.Marshal(j)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// tlsVersionName 返回版本号的可读名称
func tlsVersionName(supported, legacy uint16) string {
	v := supported
	if v == 0 {
		v = legacy
	}
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	case 0x0300:
		return "SSL 3.0"
	default:
		return ""
	}
}