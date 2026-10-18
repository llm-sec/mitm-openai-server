package tlsprobe

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

// ProbeConn 包装 net.Conn，保存窥探到的 ClientHello 信息，
// 并将已读取的原始字节回放给后续的 TLS 处理
type ProbeConn struct {
	net.Conn
	hello *ClientHello

	// 回放缓冲：保存窥探阶段已读取的原始字节
	replayBuf []byte
	replayPos int

	once sync.Once
	// 连接关闭时的清理回调（由 ProbeListener 注入）
	onClose func(remoteAddr string)
}

// Read 回放窥探阶段已读取的字节，之后透传给底层连接
func (c *ProbeConn) Read(p []byte) (int, error) {
	if c.replayPos < len(c.replayBuf) {
		n := copy(p, c.replayBuf[c.replayPos:])
		c.replayPos += n
		return n, nil
	}
	return c.Conn.Read(p)
}

// ClientHello 返回窥探到的 ClientHello 信息（可能为 nil）
func (c *ProbeConn) ClientHello() *ClientHello {
	return c.hello
}

// Close 关闭连接并触发清理
func (c *ProbeConn) Close() error {
	var err error
	c.once.Do(func() {
		if c.onClose != nil {
			c.onClose(c.RemoteAddr().String())
		}
		err = c.Conn.Close()
	})
	return err
}

// ProbeListener 包装 net.Listener，
// 在 TLS 握手开始前（TCP 层）窥探第一个 ClientHello 消息，
// 解析 JA3/JA4 指纹等关键信息，并将原始字节回放，确保 TLS 握手正常进行
type ProbeListener struct {
	net.Listener
	// 记录每个远程连接对应的 ClientHello 信息，按 remoteAddr 索引
	mu      sync.RWMutex
	hellos  map[string]*ClientHello
	timeout time.Duration // 窥探读超时
}

// NewProbeListener 创建 TLS 探测监听器
// 参数:
//   - inner: 底层 TCP 监听器
//   - probeTimeout: 窥探 ClientHello 的读超时（<=0 时使用默认 5 秒）
func NewProbeListener(inner net.Listener, probeTimeout time.Duration) *ProbeListener {
	if probeTimeout <= 0 {
		probeTimeout = 5 * time.Second
	}
	return &ProbeListener{
		Listener: inner,
		hellos:   make(map[string]*ClientHello),
		timeout:  probeTimeout,
	}
}

// Accept 接受新连接并窥探 TLS ClientHello
// 窥探成功后将解析结果与连接关联，供 HTTP 层通过 RemoteAddr 查询
func (l *ProbeListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// 窥探 ClientHello（同步读取，超时则放弃并直接透传）
	hello, consumed := peekClientHello(conn, l.timeout)

	probeConn := &ProbeConn{
		Conn:      conn,
		hello:     hello,
		replayBuf: consumed,
		onClose: func(remoteAddr string) {
			l.mu.Lock()
			delete(l.hellos, remoteAddr)
			l.mu.Unlock()
		},
	}

	if hello != nil {
		l.mu.Lock()
		l.hellos[conn.RemoteAddr().String()] = hello
		l.mu.Unlock()
	}

	return probeConn, nil
}

// GetClientHello 根据远程地址获取该连接的 ClientHello 信息
// 若连接未建立 TLS 或不含 ClientHello，返回 nil
func (l *ProbeListener) GetClientHello(remoteAddr string) *ClientHello {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.hellos[remoteAddr]
}

// peekResult 窥探结果
type peekResult struct {
	hello *ClientHello
	bytes []byte
}

// peekClientHello 从连接中读取并解析 TLS ClientHello
// 读取超时或非 TLS 流量时 hello 为 nil（不报错，直接透传），
// 已读取的字节始终返回，供上层回放
func peekClientHello(conn net.Conn, timeout time.Duration) (*ClientHello, []byte) {
	// 设置读超时，避免阻塞等待不存在的 TLS 握手
	conn.SetReadDeadline(time.Now().Add(timeout))
	defer func() {
		conn.SetReadDeadline(time.Time{})
	}()

	var consumed []byte
	take := func(buf []byte) {
		consumed = append(consumed, buf...)
	}

	// 读取 TLS 记录头：1字节类型 + 2字节版本 + 2字节长度
	header := make([]byte, 5)
	n, err := io.ReadFull(conn, header)
	if n > 0 {
		take(header[:n])
	}
	if err != nil {
		// 连接未发送任何数据（或超时、关闭），不是 TLS 握手
		return nil, consumed
	}

	// TLS Handshake 记录类型为 0x16
	if header[0] != 0x16 {
		return nil, consumed
	}

	recordLen := int(binary.BigEndian.Uint16(header[3:5]))
	if recordLen <= 0 || recordLen > 65535 {
		return nil, consumed
	}

	body := make([]byte, recordLen)
	n, err = io.ReadFull(conn, body)
	if n > 0 {
		take(body[:n])
	}
	if err != nil {
		return nil, consumed
	}

	// 解析 ClientHello（握手消息必然在第一条记录内，跨记录极罕见）
	hello, err := parseClientHelloFromRecord(header, body)
	if err != nil {
		return nil, consumed
	}
	return hello, consumed
}

// parseClientHelloFromRecord 从 TLS 记录中提取并解析 ClientHello
func parseClientHelloFromRecord(header, body []byte) (*ClientHello, error) {
	// TLS 1.3 中 record 的 legacy_version 可能是 0x0301/0x0303，不影响解析
	if len(body) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	// 握手消息：1字节类型 + 3字节长度 + 内容
	handshakeType := body[0]
	if handshakeType != 0x01 { // client_hello
		return nil, io.ErrUnexpectedEOF
	}
	helloLen := int(body[1])<<16 | int(body[2])<<8 | int(body[3])
	if len(body) < 4+helloLen {
		return nil, io.ErrUnexpectedEOF
	}
	return ParseClientHello(body[:4+helloLen])
}