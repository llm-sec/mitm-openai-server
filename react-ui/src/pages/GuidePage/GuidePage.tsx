import React, { useState, useEffect } from 'react';
import {
  Input,
  Button,
  Card,
  Typography,
  Space,
  Row,
  Col,
  List,
  Avatar,
  Spin,
  Tabs,
  Alert,
} from 'antd';
import { CopyOutlined, EyeOutlined, EyeInvisibleOutlined, UserOutlined, RobotOutlined } from '@ant-design/icons';
import Layout from '../../components/Layout/Layout';
import { useNotification } from '../../components/Notification';

const { Title, Paragraph, Text } = Typography;

const GuidePage: React.FC = () => {
  const [showToken, setShowToken] = useState(false);
  const [backendUrl, setBackendUrl] = useState('');
  const [fullApiUrl, setFullApiUrl] = useState('');
  const [token, setToken] = useState('');
  const [messages, setMessages] = useState<Array<{role: string, content: string}>>([]);
  const [inputMessage, setInputMessage] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const { addNotification } = useNotification();

  useEffect(() => {
    const origin = window.location.origin;
    setBackendUrl(origin);
    setFullApiUrl(`${origin}/v1`);

    const fetchToken = async () => {
      try {
        const response = await fetch(`${origin}/ui/api/token`);
        const data = await response.json();
        if (data.code === 0 && data.data?.token) {
          setToken(`Bearer ${data.data.token}`);
        }
      } catch (error) {
        addNotification('获取认证Token失败', 'danger');
      }
    };
    fetchToken();
  }, [addNotification]);

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      addNotification('已复制到剪贴板', 'success');
    } catch (error) {
      addNotification('复制失败', 'danger');
    }
  };

  const sendMessage = async () => {
    if (!inputMessage.trim()) return;

    const userMessage = { role: 'user', content: inputMessage };
    const updatedMessages = [...messages, userMessage];
    setMessages(updatedMessages);
    setInputMessage('');
    setIsLoading(true);

    try {
      const requestData = { model: 'gpt-3.5-turbo', messages: updatedMessages, temperature: 0.7 };
      const response = await fetch(`${backendUrl}/v1/chat/completions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': token },
        body: JSON.stringify(requestData)
      });

      if (!response.ok) throw new Error(`请求失败: ${response.status}`);

      const data = await response.json();
      if (data.choices?.[0]?.message) {
        setMessages([...updatedMessages, data.choices[0].message]);
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : '未知错误';
      setMessages([...updatedMessages, { role: 'assistant', content: `发生错误: ${errorMessage}` }]);
    } finally {
      setIsLoading(false);
    }
  };

  // 代码块组件（带复制按钮）
  const CodeBlock: React.FC<{text: string}> = ({ text }) => (
    <div style={{ position: 'relative', backgroundColor: '#282c34', padding: '16px 44px 16px 16px', borderRadius: 8, color: '#e6e6e6', fontFamily: 'monospace', fontSize: 13, overflowX: 'auto' }}>
      <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{text}</pre>
      <Button icon={<CopyOutlined />} onClick={() => copyToClipboard(text)} style={{ position: 'absolute', top: 8, right: 8 }} size="small" />
    </div>
  );

  // 可复制的只读输入框
  const CopyField: React.FC<{value: string; showToggle?: boolean}> = ({ value, showToggle }) => (
    <Input
      value={value}
      readOnly
      type={showToggle && !showToken ? 'password' : 'text'}
      addonAfter={
        <Space size={4}>
          {showToggle && (
            <Button icon={showToken ? <EyeInvisibleOutlined /> : <EyeOutlined />} onClick={() => setShowToken(!showToken)} type="text" />
          )}
          <Button icon={<CopyOutlined />} onClick={() => copyToClipboard(value)} type="text" />
        </Space>
      }
    />
  );

  // OpenAI 配置步骤
  const openaiGuide = (
    <Row gutter={[16, 24]}>
      <Col span={24}>
        <Card title="1. API地址">
          <CopyField value={fullApiUrl} />
          <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>在您的OpenAI客户端中，将baseURL设置为上述地址。</Text>
        </Card>
      </Col>
      <Col span={24}>
        <Card title="2. 认证Token">
          <CopyField value={token} showToggle />
          <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>在您的OpenAI客户端中，使用此Token作为API密钥。</Text>
        </Card>
      </Col>
      <Col span={24}>
        <Card title="3. 验证API连接">
          <Paragraph>您可以使用以下命令测试API连接：</Paragraph>
          <CodeBlock text={`curl -X GET ${fullApiUrl}/models -H "Authorization: ${token}"`} />
        </Card>
      </Col>
      <Col span={24}>
        <Card title="4. 测试对话">
          <List
            dataSource={messages}
            renderItem={item => (
              <List.Item>
                <List.Item.Meta
                  avatar={item.role === 'user' ? <Avatar icon={<UserOutlined />} /> : <Avatar style={{ backgroundColor: '#1890ff' }} icon={<RobotOutlined />} />}
                  title={item.role === 'user' ? '你' : 'AI助手'}
                  description={item.content}
                />
              </List.Item>
            )}
            style={{ minHeight: 200, maxHeight: 400, overflowY: 'auto', marginBottom: 16, border: '1px solid #f0f0f0', borderRadius: 8, padding: 16, backgroundColor: '#fafafa' }}
          />
          {isLoading && <Spin style={{ display: 'block', margin: '8px 0' }} />}
          <Input.TextArea
            rows={2}
            value={inputMessage}
            onChange={e => setInputMessage(e.target.value)}
            placeholder="输入消息..."
            disabled={isLoading}
            onPressEnter={e => { if (!e.shiftKey) { e.preventDefault(); sendMessage(); } }}
          />
          <Button type="primary" onClick={sendMessage} loading={isLoading} style={{ marginTop: 16 }}>发送</Button>
        </Card>
      </Col>
    </Row>
  );

  // Claude Code (Anthropic) 配置步骤
  const claudeGuide = (
    <Row gutter={[16, 24]}>
      <Col span={24}>
        <Alert
          type="info"
          showIcon
          message="Claude Code 协议支持"
          description="本服务器原生支持 Anthropic Messages API，Claude Code 可直接连接。模拟模式下不校验 API 密钥，填写任意 sk-ant- 前缀的密钥即可。"
          style={{ marginBottom: 8 }}
        />
      </Col>
      <Col span={24}>
        <Card title="1. API地址">
          <CopyField value={fullApiUrl} />
          <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>将 ANTHROPIC_BASE_URL 环境变量设置为上述地址。</Text>
        </Card>
      </Col>
      <Col span={24}>
        <Card title="2. 认证Token">
          <CopyField value={token} showToggle />
          <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>将 ANTHROPIC_AUTH_TOKEN 环境变量设置为上述Token。</Text>
        </Card>
      </Col>
      <Col span={24}>
        <Card title="3. 配置Claude Code连接">
          <Paragraph>设置环境变量后直接启动 Claude Code 即可（示例）：</Paragraph>
          <CodeBlock text={`export ANTHROPIC_BASE_URL=${backendUrl}\nexport ANTHROPIC_AUTH_TOKEN=${token.replace('Bearer ', '')}\nclaude`} />
        </Card>
      </Col>
      <Col span={24}>
        <Card title="4. 验证API连接">
          <Paragraph>您可以使用以下命令测试 An anthropic Messages API 连接：</Paragraph>
          <CodeBlock text={`curl -X POST ${fullApiUrl}/messages \\\n  -H "Content-Type: application/json" \\\n  -H "x-api-key: ${token.replace('Bearer ', '')}" \\\n  -H "anthropic-version: 2023-06-01" \\\n  -d '{"model":"claude-sonnet-5-5","max_tokens":64,"messages":[{"role":"user","content":"你好"}]}'`} />
        </Card>
      </Col>
    </Row>
  );

  return (
    <Layout title="API配置教程">
      <Title level={2}>API 配置指南</Title>
      <Paragraph>要使用此中间人服务器，请将您的 AI 客户端配置为使用以下 API 地址和认证 Token。所有请求将通过此服务器转发，并被记录下来供后续分析。</Paragraph>

      <Tabs
        defaultActiveKey="openai"
        items={[
          { key: 'openai', label: 'OpenAI', children: openaiGuide },
          { key: 'anthropic', label: 'Claude Code (Anthropic)', children: claudeGuide },
        ]}
      />
    </Layout>
  );
};

export default GuidePage;