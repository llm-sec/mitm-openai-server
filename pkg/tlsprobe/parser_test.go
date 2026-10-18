package tlsprobe

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"testing"
)

// TestParseClientHello_Synthetic 用构造的 ClientHello 验证解析正确性
func TestParseClientHello_Synthetic(t *testing.T) {
	// 构造含正确长度的握手消息
	msg := buildClientHelloBody()
	hello := append([]byte{0x01, byte(len(msg) >> 16), byte(len(msg) >> 8), byte(len(msg))}, msg...)

	ch, err := ParseClientHello(hello)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if ch.LegacyVersion != 0x0303 {
		t.Errorf("LegacyVersion = %#04x, 期望 0x0303", ch.LegacyVersion)
	}
	if len(ch.CipherSuites) != 2 || ch.CipherSuites[0] != 0xc02f || ch.CipherSuites[1] != 0x1301 {
		t.Errorf("CipherSuites = %v, 期望 [c02f 1301]", ch.CipherSuites)
	}
	if ch.ServerName != "ex" {
		t.Errorf("ServerName = %q, 期望 %q", ch.ServerName, "ex")
	}
	if len(ch.SupportedGroups) != 2 || ch.SupportedGroups[0] != 0x001d {
		t.Errorf("SupportedGroups = %v, 期望 [001d 0017]", ch.SupportedGroups)
	}

	ja3 := ComputeJA3(ch)
	if len(ja3) != 32 {
		t.Errorf("JA3长度异常: %d", len(ja3))
	}
	t.Logf("JA3 = %s", ja3)

	ja4 := ComputeJA4(ch)
	if len(ja4) < 10 {
		t.Errorf("JA4长度异常: %d", len(ja4))
	}
	t.Logf("JA4 = %s", ja4)
	if ja4[:1] != "t" {
		t.Errorf("JA4应以t开头: %s", ja4)
	}
}

// buildClientHelloBody 构造一个 ClientHello 的消息体（不含握手头）
func buildClientHelloBody() []byte {
	var b []byte
	// legacy_version: TLS 1.2
	b = append(b, 0x03, 0x03)
	// random: 32 字节
	for i := 0; i < 32; i++ {
		b = append(b, byte(i))
	}
	// session_id: 空
	b = append(b, 0x00)
	// cipher suites: c02f, 1301
	b = append(b, 0x00, 0x04, 0xc0, 0x2f, 0x13, 0x01)
	// compression: null
	b = append(b, 0x01, 0x00)

	// extensions
	exts := []byte{}
	// SNI: type 0000, data: listlen(0005) nametype(00) namelen(0002) "ex"
	sni := append([]byte{}, 0x00, 0x05, 0x00, 0x00, 0x02, 'e', 'x')
	exts = append(exts, 0x00, 0x00, 0x00, byte(len(sni)))
	exts = append(exts, sni...)
	// supported_groups: type 000a, data: listlen(0004) 001d 0017
	groups := []byte{0x00, 0x04, 0x00, 0x1d, 0x00, 0x17}
	exts = append(exts, 0x00, 0x0a, 0x00, byte(len(groups)))
	exts = append(exts, groups...)

	b = append(b, byte(len(exts)>>8), byte(len(exts)))
	b = append(b, exts...)
	return b
}

// TestParseClientHello_Incomplete 数据不完整时不应 panic
func TestParseClientHello_Incomplete(t *testing.T) {
	cases := [][]byte{
		nil,
		{0x01},
		{0x01, 0x00},
		{0x01, 0x00, 0x00, 0x05, 0x03}, // 长度声明了但没有数据
	}
	for _, c := range cases {
		ch, err := ParseClientHello(c)
		if err == nil && c != nil {
			// 只有空数据可能不报错
			if len(c) > 0 && ch != nil && ch.Complete {
				t.Errorf("数据不完整但解析成功: %x", c)
			}
		}
		// 不应 panic
	}
}

// TestIsGREASE GREASE 值识别
func TestIsGREASE(t *testing.T) {
	greaseVals := []uint16{0x0a0a, 0x1a1a, 0x2a2a, 0xfafa}
	for _, v := range greaseVals {
		if !isGREASE(v) {
			t.Errorf("%#04x 应为GREASE", v)
		}
	}
	normalVals := []uint16{0x1301, 0xc02f, 0x000a, 0x0010, 0x002b, 0x000d}
	for _, v := range normalVals {
		if isGREASE(v) {
			t.Errorf("%#04x 不应是GREASE", v)
		}
	}
}

// TestComputeJA4Prefix 前缀字段编码
func TestComputeJA4Prefix(t *testing.T) {
	ch := &ClientHello{
		LegacyVersion:   0x0303,
		SupportedVersion: 0x0304,
		ServerName:      "example.com",
		CipherSuites:    []uint16{0x1301, 0x1302, 0xc02f, 0x0a0a},  // 含GREASE
		ExtensionTypes:  []uint16{0x0000, 0x0010, 0x002b, 0x000d, 0x1a1a}, // 含GREASE
		ALPNProtocols:   []string{"h2", "http/1.1"},
	}
	prefix := buildJA4Prefix(ch)
	// t13 + d + 03(3个真实cipher) + 04(4个真实扩展) + h2
	expected := "t13d0304h2"
	if prefix != expected {
		t.Errorf("前缀 = %q, 期望 %q", prefix, expected)
	}
}

// TestParseRealisticClientHello 用贴近真实结构的 TLS 1.3 ClientHello 验证
func TestParseRealisticClientHello(t *testing.T) {
	var b []byte
	b = append(b, 0x01) // handshake type

	// 构造消息体
	var body []byte
	body = append(body, 0x03, 0x03) // legacy version
	// random 32 字节
	for i := 0; i < 32; i++ {
		body = append(body, byte(i*7))
	}
	// session_id 32 字节
	body = append(body, 0x20)
	for i := 0; i < 32; i++ {
		body = append(body, byte(i+1))
	}
	// cipher suites: 16 个（含 TLS1.3 与 TLS1.2 常见套件）
	ciphers := []uint16{
		0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f, 0xc02c, 0xc030, 0xcca9,
		0xcca8, 0xc013, 0xc014, 0x009c, 0x009d, 0x002f, 0x0035, 0x000a,
	}
	body = binary.BigEndian.AppendUint16(body, uint16(len(ciphers)*2))
	for _, c := range ciphers {
		body = binary.BigEndian.AppendUint16(body, c)
	}
	// compression
	body = append(body, 0x01, 0x00)

	// extensions
	exts := []byte{}
	// SNI: www.example.com
	sniName := "www.example.com"
	sni := []byte{0x00, byte(len(sniName) + 3), 0x00, byte(len(sniName)>>8), byte(len(sniName))}
	sni = append(sni, sniName...)
	exts = append(exts, []byte{0x00, 0x00, 0x00, byte(len(sni))}...)
	exts = append(exts, sni...)
	// supported_versions: 长度前缀04 + 0304, 0303（RFC 8446 §4.2.1）
	sv := []byte{0x04, 0x03, 0x04, 0x03, 0x03}
	exts = append(exts, []byte{0x00, 0x2b, 0x00, byte(len(sv))}...)
	exts = append(exts, sv...)
	// ALPN: h2, http/1.1（协议列表长度 = 3 + 9 = 12 = 0x000c）
	alpn := []byte{0x00, 0x0c, 0x02, 'h', '2', 0x08, 'h', 't', 't', 'p', '/', '1', '.', '1'}
	exts = append(exts, []byte{0x00, 0x10, 0x00, byte(len(alpn))}...)
	exts = append(exts, alpn...)
	// signature_algorithms: 0403 0804 0401 0503 0805 0501
	sig := []byte{0x00, 0x0c, 0x04, 0x03, 0x08, 0x04, 0x04, 0x01, 0x05, 0x03, 0x08, 0x05, 0x05, 0x01}
	exts = append(exts, []byte{0x00, 0x0d, 0x00, byte(len(sig))}...)
	exts = append(exts, sig...)

	body = append(body, byte(len(exts)>>8), byte(len(exts)))
	body = append(body, exts...)

	// 组装握手消息
	raw := append([]byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)

	ch, err := ParseClientHello(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !ch.Complete {
		t.Fatalf("ClientHello 未完整解析")
	}
	if ch.ServerName != "www.example.com" {
		t.Errorf("SNI = %q, 期望 www.example.com", ch.ServerName)
	}
	if ch.SupportedVersion != 0x0304 {
		t.Errorf("SupportedVersion = %#04x, 期望 0x0304", ch.SupportedVersion)
	}
	if len(ch.ALPNProtocols) != 2 || ch.ALPNProtocols[0] != "h2" {
		t.Errorf("ALPN = %v, 期望 [h2 http/1.1]", ch.ALPNProtocols)
	}
	if len(ch.SignatureAlgorithms) == 0 {
		t.Error("应解析到签名算法")
	}

	ja3 := ComputeJA3(ch)
	ja4 := ComputeJA4(ch)
	t.Logf("JA3 = %s", ja3)
	t.Logf("JA4 = %s", ja4)

	// JA4 前缀: t13 + d + 16(真实cipher数) + 4(4个真实扩展) + h2
	expectedPrefix := "t13d1604h2"
	if ja4[:10] != expectedPrefix {
		t.Errorf("JA4前缀 = %q, 期望 %q", ja4[:10], expectedPrefix)
	}
}

// TestProbeListener 端到端：构造一个假TLS客户端连到 ProbeListener，
// 验证 ClientHello 被窥探且字节被回放
func TestProbeListener(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	pl := NewProbeListener(inner, 0)

	acceptCh := make(chan *ProbeConn, 1)
	go func() {
		c, err := pl.Accept()
		if err != nil {
			acceptCh <- nil
			return
		}
		pc := c.(*ProbeConn)
		acceptCh <- pc
	}()

	// 构造 ClientHello 字节
	body := buildClientHelloBody()
	helloMsg := append([]byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)

	// TLS 记录：type 0x16, version 0x0303, length
	record := []byte{0x16, 0x03, 0x03, byte(len(helloMsg) >> 8), byte(len(helloMsg))}
	record = append(record, helloMsg...)

	// 客户端连接并发送
	conn, err := net.Dial("tcp", inner.Addr().String())
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write(record); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	pc := <-acceptCh
	if pc == nil {
		t.Fatal("Accept 返回 nil")
	}
	defer pc.Close()

	if pc.ClientHello() == nil {
		t.Fatal("ClientHello 未被窥探")
	}
	if pc.ClientHello().ServerName != "ex" {
		t.Errorf("SNI = %q, 期望 ex", pc.ClientHello().ServerName)
	}

	// 验证回放：读回应等于原始 record 字节
	replay := make([]byte, len(record))
	if _, err := io.ReadFull(pc, replay); err != nil {
		t.Fatalf("回放读取失败: %v", err)
	}
	if hex.EncodeToString(replay) != hex.EncodeToString(record) {
		t.Errorf("回放字节不一致")
	}

	// 通过 listener 查询
	ch := pl.GetClientHello(pc.RemoteAddr().String())
	if ch == nil || ch.ServerName != "ex" {
		t.Errorf("GetClientHello 查询失败: %+v", ch)
	}
}