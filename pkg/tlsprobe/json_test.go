package tlsprobe

import "testing"

// TestClientHelloToMap 验证 ClientHello 到 JSON 兼容 map 的转换
func TestClientHelloToMap(t *testing.T) {
	ch := &ClientHello{
		LegacyVersion:    0x0303,
		SupportedVersion: 0x0304,
		ServerName:       "api.example.com",
		CipherSuites:     []uint16{0x1301, 0xc02f},
		ExtensionTypes:   []uint16{0x0000, 0x000a, 0x000d, 0x0010, 0x002b},
		ALPNProtocols:    []string{"h2", "http/1.1"},
		SupportedGroups:  []uint16{0x001d, 0x0017},
		SessionID:        []byte{0x01, 0x02, 0x03},
		Random:           make([]byte, 32),
	}

	m := ClientHelloToMap(ch)
	if m == nil {
		t.Fatal("ClientHelloToMap 返回 nil")
	}

	checks := []struct {
		key string
		val string
	}{
		{"legacy_version", "0303"},
		{"supported_version", "0304"},
		{"supported_version_t", "TLS 1.3"},
		{"server_name", "api.example.com"},
	}
	for _, c := range checks {
		if got, ok := m[c.key].(string); !ok || got != c.val {
			t.Errorf("%s = %v (%T), 期望 %q", c.key, m[c.key], m[c.key], c.val)
		}
	}

	// 指纹应为32位hex（JA3 MD5）与长字符串（JA4）
	ja3, _ := m["ja3_fingerprint"].(string)
	if len(ja3) != 32 {
		t.Errorf("ja3_fingerprint 长度异常: %q", ja3)
	}
	ja4, _ := m["ja4_fingerprint"].(string)
	if len(ja4) == 0 || ja4[:1] != "t" {
		t.Errorf("ja4_fingerprint 异常: %q", ja4)
	}

	// 列表字段
	ciphers, _ := m["cipher_suites"].([]interface{})
	if len(ciphers) != 2 || ciphers[0] != "1301" {
		t.Errorf("cipher_suites = %v, 期望 [1301 c02f]", ciphers)
	}

	// nil 输入返回 nil
	if ClientHelloToMap(nil) != nil {
		t.Error("nil 输入应返回 nil")
	}
}
