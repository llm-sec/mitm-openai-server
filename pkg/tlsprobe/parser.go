package tlsprobe

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// GREASE 值范围（RFC 8701）：0x0a0a、0x1a1a、... 0xfafa，
// 两个字节的高4位相同、低4位均为 0xa。
func isGREASE(val uint16) bool {
	return val&0x000f == 0x000a && // 低字节低4位 = a
		(val>>8)&0x000f == 0x000a && // 高字节低4位 = a
		val>>12 == (val>>4)&0x000f // 两字节的高4位相同
}

// ClientHello 保存解析后的 TLS ClientHello 关键字段
type ClientHello struct {
	// 原始ClientHello字节（不含TLS记录头）
	Raw []byte

	// TLS记录层的协议版本（legacy_version，位于握手消息前两个字节）
	LegacyVersion uint16
	// 支持的最高TLS版本（来自 supported_versions 扩展，若存在）
	SupportedVersion uint16

	// 客户端随机数（32字节）
	Random []byte
	// SessionID
	SessionID []byte

	// 密码套件列表（原始顺序）
	CipherSuites []uint16
	// 扩展列表（原始顺序，仅类型号）
	ExtensionTypes []uint16
	// 扩展类型 -> 扩展内容（原始字节，不含类型号和长度）
	Extensions map[uint16][]byte

	// 各扩展的解析结果
	ServerName          string // SNI 明文（主机名）
	ALPNProtocols       []string
	SupportedGroups     []uint16
	SignatureAlgorithms []uint16

	// 是否成功解析到完整ClientHello
	Complete bool
}

// ParseClientHello 解析原始TLS ClientHello字节流（不包含TLS记录头）
// 如果数据不完整或格式错误，返回部分解析结果与错误
func ParseClientHello(raw []byte) (*ClientHello, error) {
	ch := &ClientHello{
		Raw:        raw,
		Extensions: make(map[uint16][]byte),
	}

	// 握手消息头：1字节类型 + 3字节长度
	if len(raw) < 4 {
		return ch, fmt.Errorf("ClientHello数据过短: %d 字节", len(raw))
	}

	if raw[0] != 0x01 {
		return ch, fmt.Errorf("不是ClientHello消息: 握手类型 0x%02x", raw[0])
	}

	// 握手长度（3字节大端）
	handshakeLen := int(raw[1])<<16 | int(raw[2])<<8 | int(raw[3])
	if len(raw) < 4+handshakeLen {
		return ch, fmt.Errorf("ClientHello数据不完整: 需要 %d 字节，实际 %d", 4+handshakeLen, len(raw))
	}

	body := raw[4 : 4+handshakeLen]
	pos := 0

	// legacy_version: 2字节
	if len(body) < 2 {
		return ch, fmt.Errorf("ClientHello缺少legacy_version")
	}
	ch.LegacyVersion = binary.BigEndian.Uint16(body[pos:])
	pos += 2

	// random: 32字节
	if len(body) < pos+32 {
		return ch, fmt.Errorf("ClientHello缺少random")
	}
	ch.Random = make([]byte, 32)
	copy(ch.Random, body[pos:pos+32])
	pos += 32

	// session_id: 1字节长度 + 内容
	if len(body) < pos+1 {
		return ch, fmt.Errorf("ClientHello缺少session_id长度")
	}
	sidLen := int(body[pos])
	pos += 1
	if len(body) < pos+sidLen {
		return ch, fmt.Errorf("ClientHello缺少session_id内容")
	}
	ch.SessionID = make([]byte, sidLen)
	copy(ch.SessionID, body[pos:pos+sidLen])
	pos += sidLen

	// cipher_suites: 2字节总长度（必须是偶数）+ 2字节每项
	if len(body) < pos+2 {
		return ch, fmt.Errorf("ClientHello缺少cipher_suites长度")
	}
	cipherLen := int(binary.BigEndian.Uint16(body[pos:]))
	pos += 2
	if cipherLen%2 != 0 || len(body) < pos+cipherLen {
		return ch, fmt.Errorf("ClientHello密码套件长度非法: %d", cipherLen)
	}
	for i := 0; i < cipherLen; i += 2 {
		ch.CipherSuites = append(ch.CipherSuites, binary.BigEndian.Uint16(body[pos+i:pos+i+2]))
	}
	pos += cipherLen

	// compression_methods: 1字节长度 + 内容（可跳过）
	if len(body) < pos+1 {
		return ch, fmt.Errorf("ClientHello缺少compression_methods长度")
	}
	compLen := int(body[pos])
	pos += 1 + compLen
	if len(body) < pos {
		return ch, fmt.Errorf("ClientHello压缩方法长度非法")
	}

	// extensions: 2字节总长度 + 各扩展
	if pos >= len(body) {
		// 没有扩展（TLS 1.3 必须有扩展，但 TLS 1.2 及更早版本可以没有）
		ch.Complete = true
		return ch, nil
	}
	if len(body) < pos+2 {
		return ch, fmt.Errorf("ClientHello缺少extensions长度")
	}
	extTotalLen := int(binary.BigEndian.Uint16(body[pos:]))
	pos += 2
	if len(body) < pos+extTotalLen {
		return ch, fmt.Errorf("ClientHello扩展数据不完整")
	}

	extEnd := pos + extTotalLen
	for pos < extEnd {
		if len(body) < pos+4 {
			return ch, fmt.Errorf("ClientHello扩展头不完整")
		}
		extType := binary.BigEndian.Uint16(body[pos:])
		extLen := int(binary.BigEndian.Uint16(body[pos+2:]))
		pos += 4
		if len(body) < pos+extLen {
			return ch, fmt.Errorf("ClientHello扩展内容不完整")
		}
		ch.ExtensionTypes = append(ch.ExtensionTypes, extType)
		extData := make([]byte, extLen)
		copy(extData, body[pos:pos+extLen])
		ch.Extensions[extType] = extData
		pos += extLen
	}

	// 解析关键扩展
	ch.parseExtensions()

	ch.Complete = true
	return ch, nil
}

// parseExtensions 解析SNI、ALPN、supported_groups、signature_algorithms、supported_versions
func (ch *ClientHello) parseExtensions() {
	// SNI (0x0000)
	if data, ok := ch.Extensions[0x0000]; ok {
		ch.ServerName = parseSNI(data)
	}

	// ALPN (0x0010)
	if data, ok := ch.Extensions[0x0010]; ok {
		ch.ALPNProtocols = parseALPN(data)
	}

	// supported_groups (0x000a)
	if data, ok := ch.Extensions[0x000a]; ok {
		ch.SupportedGroups = parseUint16List(data)
	}

	// signature_algorithms (0x000d)
	if data, ok := ch.Extensions[0x000d]; ok {
		ch.SignatureAlgorithms = parseUint16List(data)
	}

	// supported_versions (0x002b)
	if data, ok := ch.Extensions[0x002b]; ok {
		ch.SupportedVersion = parseSupportedVersions(data)
	}
}

// parseSNI 解析SNI扩展（0x0000）
// 结构: 2字节列表长度 | 1字节名称类型(0=host_name) | 2字节名称长度 | 名称
func parseSNI(data []byte) string {
	if len(data) < 5 {
		return ""
	}
	// 跳过2字节列表长度
	nameType := data[2]
	if nameType != 0 {
		return ""
	}
	nameLen := int(binary.BigEndian.Uint16(data[3:]))
	if len(data) < 5+nameLen {
		return ""
	}
	return string(data[5 : 5+nameLen])
}

// parseALPN 解析ALPN扩展（0x0010）
// 结构: 2字节协议列表长度 | (1字节长度 + 协议名)*
func parseALPN(data []byte) []string {
	if len(data) < 2 {
		return nil
	}
	listLen := int(binary.BigEndian.Uint16(data[:2]))
	if len(data) < 2+listLen {
		return nil
	}
	var protos []string
	pos := 2
	end := 2 + listLen
	for pos < end {
		plen := int(data[pos])
		pos += 1
		if pos+plen > end {
			break
		}
		protos = append(protos, string(data[pos:pos+plen]))
		pos += plen
	}
	return protos
}

// parseUint16List 解析"2字节总长度 + 2字节项列表"结构
func parseUint16List(data []byte) []uint16 {
	if len(data) < 2 {
		return nil
	}
	listLen := int(binary.BigEndian.Uint16(data[:2]))
	if len(data) < 2+listLen || listLen%2 != 0 {
		return nil
	}
	var vals []uint16
	for i := 0; i < listLen; i += 2 {
		vals = append(vals, binary.BigEndian.Uint16(data[2+i:2+i+2]))
	}
	return vals
}

// parseSupportedVersions 解析supported_versions扩展（0x002b），取最高版本
// RFC 8446 §4.2.1: ClientHello中格式为 ProtocolVersion versions<2..254>（1字节长度前缀）
func parseSupportedVersions(data []byte) uint16 {
	if len(data) < 3 {
		return 0
	}
	// ClientHello: 1字节长度前缀 + 2字节版本列表
	listLen := int(data[0])
	if listLen < 2 || len(data) < 1+listLen {
		return 0
	}
	versions := data[1 : 1+listLen]
	var highest uint16
	for i := 0; i+1 < len(versions); i += 2 {
		v := binary.BigEndian.Uint16(versions[i:])
		if isGREASE(v) {
			continue
		}
		if v > highest {
			highest = v
		}
	}
	return highest
}

// formatHexID 返回2字节ID的4位小写十六进制形式（如 "1301"）
func formatHexID(v uint16) string {
	s := hex.EncodeToString([]byte{byte(v >> 8), byte(v & 0xff)})
	if len(s) < 4 {
		// 理论上不会发生，防御性填充
		for len(s) < 4 {
			s = "0" + s
		}
	}
	return s
}
