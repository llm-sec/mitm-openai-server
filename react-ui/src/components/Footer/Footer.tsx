import React from 'react';
import { Layout, Tag, Space, Tooltip } from 'antd';
import './Footer.css';

const { Footer: AntFooter } = Layout;

const Footer: React.FC = () => {
  const year = new Date().getFullYear();
  const repoUrl = 'https://github.com/llm-sec/mitm-openai-server';

  return (
    <AntFooter className="footer">
      <div className="footer-content">
        <a href={repoUrl} target="_blank" rel="noopener noreferrer" className="footer-link">
          MITM AI Server
        </a>
        <span className="footer-divider">•</span>
        <span className="footer-copyright">{year}</span>
        <span className="footer-divider">•</span>
        <span className="footer-protocols">
          <Space size={4}>
            <Tooltip title="OpenAI API 协议">
              <Tag color="geekblue" className="footer-tag">OpenAI</Tag>
            </Tooltip>
            <Tooltip title="Anthropic Messages API 协议（Claude）">
              <Tag color="purple" className="footer-tag">Claude</Tag>
            </Tooltip>
            <Tooltip title="TLS ClientHello 指纹捕获">
              <Tag color="green" className="footer-tag">TLS</Tag>
            </Tooltip>
          </Space>
        </span>
      </div>
    </AntFooter>
  );
};

export default Footer;