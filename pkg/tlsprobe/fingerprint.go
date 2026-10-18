package tlsprobe

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// ComputeJA3 计算JA3指纹
// JA3 = MD5(SSLVersion,Ciphers,Extensions,EllipticCurves,EllipticCurvePointFormats)
// 其中各字段为十进制逗号分隔列表
func ComputeJA3(ch *ClientHello) string {
	var sb strings.Builder

	// SSLVersion: TLS记录层的legacy_version十进制（TLS 1.2/1.3均为0x0303=771）
	sb.WriteString(strconv.Itoa(int(ch.LegacyVersion)))
	sb.WriteString(",")

	// Ciphers: 十进制逗号分隔
	for i, cs := range ch.CipherSuites {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Itoa(int(cs)))
	}
	sb.WriteString(",")

	// Extensions: 十进制逗号分隔（原始顺序）
	for i, ext := range ch.ExtensionTypes {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Itoa(int(ext)))
	}
	sb.WriteString(",")

	// EllipticCurves: supported_groups十进制逗号分隔
	for i, g := range ch.SupportedGroups {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Itoa(int(g)))
	}
	sb.WriteString(",")

	// EllipticCurvePointFormats: ec_point_formats十进制逗号分隔
	// （当前未单独解析，常见值只有0；若扩展存在则解析）
	formats := parseECPointFormats(ch.Extensions[0x000b])
	for i, f := range formats {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Itoa(int(f)))
	}

	ja3Str := sb.String()
	// 移除末尾的逗号（不存在的列表）
	ja3Str = strings.TrimRight(ja3Str, ",")

	sum := md5.Sum([]byte(ja3Str))
	return hex.EncodeToString(sum[:])
}

// parseECPointFormats 解析ec_point_formats扩展（0x000b）
// 结构: 1字节长度 + 格式列表（1字节每项）
func parseECPointFormats(data []byte) []byte {
	if len(data) < 1 {
		return nil
	}
	l := int(data[0])
	if len(data) < 1+l {
		return nil
	}
	return data[1 : 1+l]
}

// ComputeJA4 计算JA4指纹（RFC草案/FOXIO规范）
// 格式: [prefix]_[cipher hash]_[extension hash]
//   - prefix: t/protocol + 版本(2字符) + SNI类型 + 密码套件数(2字符) + 扩展数(2字符) + ALPN(2字符)
//   - cipher hash: 密码套件（按16进制排序）SHA256前12字符
//   - extension hash: 扩展（排序，排除SNI/ALPN） + "_" + 签名算法（原顺序）SHA256前12字符
func ComputeJA4(ch *ClientHello) string {
	prefix := buildJA4Prefix(ch)

	// Cipher hash
	cipherList := make([]string, 0, len(ch.CipherSuites))
	for _, cs := range ch.CipherSuites {
		if isGREASE(cs) {
			continue
		}
		cipherList = append(cipherList, formatHexID(cs))
	}
	sortStrings(cipherList)
	cipherHash := hash12(strings.Join(cipherList, ","))
	if len(cipherList) == 0 {
		cipherHash = "000000000000"
	}

	// Extension hash：排除SNI(0x0000)与ALPN(0x0010)，因为已在前缀体现
	extList := make([]string, 0, len(ch.ExtensionTypes))
	for _, ext := range ch.ExtensionTypes {
		if isGREASE(ext) || ext == 0x0000 || ext == 0x0010 {
			continue
		}
		extList = append(extList, formatHexID(ext))
	}
	sortStrings(extList)

	var extHashStr string
	if len(extList) == 0 && len(ch.SignatureAlgorithms) == 0 {
		extHashStr = "000000000000"
	} else {
		extHashStr = strings.Join(extList, ",")
		// 追加签名算法（原顺序，不做排序）
		if len(ch.SignatureAlgorithms) > 0 {
			sigList := make([]string, 0, len(ch.SignatureAlgorithms))
			for _, sig := range ch.SignatureAlgorithms {
				if isGREASE(sig) {
					continue
				}
				sigList = append(sigList, formatHexID(sig))
			}
			if len(sigList) > 0 {
				extHashStr += "_" + strings.Join(sigList, ",")
			}
		}
		extHashStr = hash12(extHashStr)
	}

	return prefix + "_" + cipherHash + "_" + extHashStr
}

// buildJA4Prefix 构建JA4的前缀部分
// 格式: t(协议) + 版本2字符 + SNI类型(1字符) + 密码套件数(2字符) + 扩展数(2字符) + ALPN(2字符)
func buildJA4Prefix(ch *ClientHello) string {
	// 协议: t=TLS over TCP
	proto := "t"

	// 版本: 优先用supported_versions扩展的最高值，否则用legacy_version
	ver := versionCode(ch.LegacyVersion)
	if ch.SupportedVersion != 0 {
		ver = versionCode(ch.SupportedVersion)
	}

	// SNI类型: d=域名, i=IP地址, e=空
	sniType := "e"
	if ch.ServerName != "" {
		if isIPAddress(ch.ServerName) {
			sniType = "i"
		} else {
			sniType = "d"
		}
	}

	// 密码套件数量（排除GREASE，最多2字符，上限99）
	cipherCount := 0
	for _, cs := range ch.CipherSuites {
		if isGREASE(cs) {
			continue
		}
		cipherCount++
	}
	if cipherCount > 99 {
		cipherCount = 99
	}
	cipherCountStr := pad2(cipherCount)

	// 扩展数量（包含SNI和ALPN，排除GREASE，上限99）
	extCount := 0
	for _, ext := range ch.ExtensionTypes {
		if isGREASE(ext) {
			continue
		}
		extCount++
	}
	if extCount > 99 {
		extCount = 99
	}
	extCountStr := pad2(extCount)

	// ALPN: 取第一个协议的字符串（最多2字符）
	alpnStr := "00"
	if len(ch.ALPNProtocols) > 0 && ch.ALPNProtocols[0] != "" {
		alpnStr = alpn2ch(ch.ALPNProtocols[0])
	}

	return proto + ver + sniType + cipherCountStr + extCountStr + alpnStr
}

// versionCode 将版本号映射为JA4的2字符代码
func versionCode(v uint16) string {
	switch v {
	case 0x0304: // TLS 1.3
		return "13"
	case 0x0303: // TLS 1.2
		return "12"
	case 0x0302: // TLS 1.1
		return "11"
	case 0x0301: // TLS 1.0
		return "10"
	case 0x0300: // SSL 3.0
		return "s3"
	case 0x0002: // SSL 2.0
		return "s2"
	case 0xfeff: // DTLS 1.2
		return "d1"
	case 0xfefd: // DTLS 1.0
		return "d2"
	default:
		return "00"
	}
}

// isIPAddress 粗略判断字符串是否为IP地址
func isIPAddress(s string) bool {
	// 简单判断：IPv4包含数字和点，IPv6包含冒号
	// 更准确的判断需要net.ParseIP，但SNI中IP很少见
	for _, c := range s {
		if c == ':' {
			return true
		}
	}
	// IPv4
	hasDigit := false
	hasDot := false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if c == '.' {
			hasDot = true
		} else {
			return false
		}
	}
	return hasDigit && hasDot
}

// alpn2ch 将ALPN协议名转换为JA4前缀中的2字符
// http/1.1 => "h1", h2 => "h2", 其他取首尾字符
func alpn2ch(s string) string {
	if len(s) == 1 {
		// 单字符使用两次
		return s + s
	}
	if len(s) >= 2 {
		return string(s[0]) + string(s[len(s)-1])
	}
	return "00"
}

// pad2 将数字填充为2位字符串（如6 => "06", 15 => "15", 100 => "99"封顶已在调用方处理）
func pad2(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 99 {
		n = 99
	}
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// sortStrings 按字典序升序排序字符串切片
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// hash12 计算SHA256并取前12个十六进制字符
func hash12(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}