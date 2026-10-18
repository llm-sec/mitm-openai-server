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
} from 'antd';
import { CopyOutlined, EyeOutlined, EyeInvisibleOutlined, UserOutlined } from '@ant-design/icons';
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

  const CodeBlock: React.FC<{text: string}> = ({ text }) => (
    <div style={{ position: 'relative', backgroundColor: '#282c34', padding: '16px', borderRadius: 8, color: 'white', fontFamily: 'monospace' }}>
      <code>{text}</code>
      <Button icon={<CopyOutlined />} onClick={() => copyToClipboard(text)} style={{ position: 'absolute', top: 8, right: 8 }} />
    </div>
  );

  return (
    <Layout title="OpenAI API配置教程">
      <Title level={2}>OpenAI API配置指南</Title>
      <Paragraph>要使用此中间人服务器，您需要将OpenAI客户端配置为使用以下API地址和认证Token。这样您的所有API请求将通过此服务器，并被记录下来供后续分析。</Paragraph>
      
      <Row gutter={[16, 24]}>
        <Col span={24}>
          <Card title="1. API地址">
            <Input value={fullApiUrl} readOnly addonAfter={<Button icon={<CopyOutlined />} onClick={() => copyToClipboard(fullApiUrl)} />} />
            <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>在您的OpenAI客户端中，将baseURL设置为上述地址。</Text>
          </Card>
        </Col>
        <Col span={24}>
          <Card title="2. 认证Token">
            <Input 
              value={token} 
              readOnly 
              type={showToken ? 'text' : 'password'}
              addonAfter={
                <Space>
                  <Button icon={showToken ? <EyeInvisibleOutlined /> : <EyeOutlined />} onClick={() => setShowToken(!showToken)} />
                  <Button icon={<CopyOutlined />} onClick={() => copyToClipboard(token)} />
                </Space>
              }
            />
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
                    avatar={item.role === 'user' ? <Avatar icon={<UserOutlined />} /> : <Avatar>AI</Avatar>}
                    title={item.role === 'user' ? '你' : 'AI助手'}
                    description={item.content}
                  />
                </List.Item>
              )}
              style={{ minHeight: 200, maxHeight: 400, overflowY: 'auto', marginBottom: 16, border: '1px solid #f0f0f0', padding: 16 }}
            />
            {isLoading && <Spin />}
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
    </Layout>
  );
};

export default GuidePage; 