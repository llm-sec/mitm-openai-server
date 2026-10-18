import React, { useState } from 'react';
import { Tag, Typography, Button, Tooltip, Alert, Collapse, Descriptions } from 'antd';
import { CopyOutlined, SecurityScanOutlined, DownOutlined } from '@ant-design/icons';
import './TlsClientHelloContent.css';

const { Text } = Typography;

interface TlsClientHelloContentProps {
  /** 请求元数据中的 tls_client_hello 对象 */
  tls: any;
  copyToClipboard: (text: any) => void;
}

/**
 * TLS ClientHello 内容组件
 * 展示客户端TLS握手阶段的ClientHello信息，包括JA3/JA4指纹、SNI、密码套件、扩展等
 */
const TlsClientHelloContent: React.FC<TlsClientHelloContentProps> = ({ tls, copyToClipboard }) => {
  const [showDetails, setShowDetails] = useState(false);

  // 如果未捕获到TLS信息，给出提示
  if (!tls || typeof tls !== 'object') {
    return (
      <Alert
        type="info"
        showIcon
        message="未捕获到 TLS ClientHello 信息"
        description="服务器以 HTTPS 模式启动（--tls / --auto-tls）时，会自动记录每个请求的 TLS 握手信息（SNI、JA3/JA4 指纹、密码套件等）。当前请求可能通过 HTTP 发出，或服务器未启用 HTTPS。"
      />
    );
  }

  const fingerprintBlock = (
    title: string,
    value: string,
    desc: string
  ) => (
    <div className="tls-fingerprint-card">
      <div className="tls-fingerprint-title">{title}</div>
      <div className="tls-fingerprint-value">{value || '—'}</div>
      <div className="tls-fingerprint-desc">{desc}</div>
      <div className="tls-fingerprint-actions">
        <Tooltip title={`复制${title}`}>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            onClick={() => copyToClipboard(value)}
          />
        </Tooltip>
      </div>
    </div>
  );

  const renderTagList = (items: any[], color?: string) => {
    if (!Array.isArray(items) || items.length === 0) {
      return <Text type="secondary">无</Text>;
    }
    return (
      <div className="tls-tag-list">
        {items.map((item, idx) => (
          <Tag key={`${item}-${idx}`} color={color} className="tls-tag">
            {item}
          </Tag>
        ))}
      </div>
    );
  };

  const cipherSuites = tls.cipher_suites || [];
  const extensions = tls.extensions || [];
  const supportedGroups = tls.supported_groups || [];
  const signatureAlgorithms = tls.signature_algorithms || [];

  const details = (
    <Descriptions
      column={{ xs: 1, sm: 1, md: 2 }}
      size="small"
      bordered
      className="tls-descriptions"
    >
      <Descriptions.Item label="TLS版本">{tls.supported_version_t || '未知'}</Descriptions.Item>
      <Descriptions.Item label="协议版本号">
        <code>{tls.supported_version || tls.legacy_version || '—'}</code>
      </Descriptions.Item>
      <Descriptions.Item label="SNI（服务器名称）">
        {tls.server_name || <Text type="secondary">无（直连IP或未携带SNI）</Text>}
      </Descriptions.Item>
      <Descriptions.Item label="ALPN 协议">
        {Array.isArray(tls.alpn) && tls.alpn.length > 0
          ? tls.alpn.join(', ')
          : <Text type="secondary">无</Text>}
      </Descriptions.Item>
    </Descriptions>
  );

  return (
    <div className="tls-content">
      <Alert
        type="success"
        showIcon
        icon={<SecurityScanOutlined />}
        message="已捕获 TLS ClientHello 指纹"
        description="以下信息来自客户端发起 TLS 握手时的明文 ClientHello 报文，可用于 JA3/JA4 指纹识别与 SNI 数据外泄检测。"
        className="tls-banner"
      />

      {/* 指纹区域 */}
      <div className="tls-fingerprints">
        {fingerprintBlock(
          'JA3 指纹',
          tls.ja3_fingerprint,
          'SSLVersion,Ciphers,Extensions,EC曲线,点格式 的 MD5 值'
        )}
        {fingerprintBlock(
          'JA4 指纹',
          tls.ja4_fingerprint,
          '协议版本 + 密码套件/扩展哈希 + ALPN 的指纹'
        )}
      </div>

      {/* 基本信息 */}
      {details}

      {/* 详细字段 */}
      <Collapse
        ghost
        activeKey={showDetails ? 'details' : undefined}
        onChange={() => setShowDetails(!showDetails)}
        expandIcon={({ isActive }) => <DownOutlined rotate={isActive ? 180 : 0} />}
      >
        <Collapse.Panel header="查看更多字段（密码套件 / 扩展 / 群组 / 签名算法）" key="details">
          <div className="tls-detail-section">
            <Text strong>密码套件（{cipherSuites.length}）</Text>
            {renderTagList(cipherSuites, 'blue')}
          </div>
          <div className="tls-detail-section">
            <Text strong>扩展（{extensions.length}）</Text>
            {renderTagList(extensions, 'geekblue')}
          </div>
          <div className="tls-detail-section">
            <Text strong>支持的椭圆曲线群组（{supportedGroups.length}）</Text>
            {renderTagList(supportedGroups, 'purple')}
          </div>
          <div className="tls-detail-section">
            <Text strong>签名算法（{signatureAlgorithms.length}）</Text>
            {renderTagList(signatureAlgorithms, 'cyan')}
          </div>

          {(tls.session_id || tls.random) && (
            <div className="tls-detail-section">
              <Text strong>会话信息</Text>
              <div className="tls-hex-block">
                {tls.session_id && (
                  <div>
                    <Text type="secondary">Session ID：</Text>
                    <code>{tls.session_id}</code>
                  </div>
                )}
                {tls.random && (
                  <div>
                    <Text type="secondary">Random（32字节）：</Text>
                    <code className="tls-random">{tls.random}</code>
                  </div>
                )}
              </div>
            </div>
          )}
        </Collapse.Panel>
      </Collapse>
    </div>
  );
};

export default TlsClientHelloContent;
