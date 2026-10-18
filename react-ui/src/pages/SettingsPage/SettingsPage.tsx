import React, { useState, useEffect } from 'react';
import {
  Form,
  Input,
  Button,
  Switch,
  Select,
  Card,
  Typography,
  Spin,
  Space,
  Row,
  Col,
  Alert,
  Divider,
  Tag,
  Tooltip,
} from 'antd';
import {
  CloudServerOutlined,
  SafetyCertificateOutlined,
  KeyOutlined,
  ApiOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import Layout from '../../components/Layout/Layout';
import { ProxyConfig } from '../../types';
import { apiService } from '../../services/api';
import { useNotification } from '../../components/Notification/Notification';

const { Title, Paragraph, Text } = Typography;
const { Option } = Select;

const SettingsPage: React.FC = () => {
  const [form] = Form.useForm();
  const [proxyConfig, setProxyConfig] = useState<ProxyConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const { addNotification } = useNotification();

  const isProxyEnabled = Form.useWatch('enabled', form);
  const authType = Form.useWatch('authType', form);

  useEffect(() => {
    const loadProxyConfig = async () => {
      setLoading(true);
      try {
        const config = await apiService.getProxyConfig();
        if (config) {
          setProxyConfig(config);
          form.setFieldsValue(config);
        }
      } catch (error) {
        addNotification('加载代理配置失败', 'danger');
      } finally {
        setLoading(false);
      }
    };
    loadProxyConfig();
  }, [form, addNotification]);

  const handleFinish = async (values: ProxyConfig) => {
    setSaving(true);
    try {
      const success = await apiService.saveProxyConfig(values);
      if (success) {
        addNotification('代理设置已保存', 'success');
        setProxyConfig(values);
      } else {
        addNotification('保存代理设置失败', 'danger');
      }
    } catch (error) {
      addNotification('保存代理设置失败', 'danger');
    } finally {
      setSaving(false);
    }
  };

  const handleQuickConfig = (url: string) => {
    form.setFieldsValue({ targetURL: url });
  };

  // 认证类型描述映射
  const authDescriptions: Record<string, string> = {
    none: '不附加任何认证信息，直接转发请求。',
    basic: '使用用户名/密码进行 Basic 认证。',
    token: '使用 Bearer Token 认证（适用于大多数 API）。',
  };

  return (
    <Layout title="系统设置">
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Title level={2} style={{ marginBottom: 4 }}>系统设置</Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            配置代理服务器以将请求转发到实际的 API 端点
          </Paragraph>
        </div>

        <Spin spinning={loading}>
          <Card
            title={
              <Space>
                <CloudServerOutlined />
                <span>代理配置</span>
              </Space>
            }
          >
            <Form
              form={form}
              layout="vertical"
              onFinish={handleFinish}
              initialValues={proxyConfig || undefined}
            >
              <Form.Item
                label={
                  <Space>
                    <span>代理模式</span>
                    <Tooltip title="启用后将请求转发到目标API服务器；禁用时返回内置模拟数据">
                      <InfoCircleOutlined style={{ color: '#999' }} />
                    </Tooltip>
                  </Space>
                }
                name="enabled"
                valuePropName="checked"
              >
                <Switch checkedChildren="启用" unCheckedChildren="禁用" />
              </Form.Item>

              <Alert
                type={isProxyEnabled ? 'success' : 'info'}
                showIcon
                message={isProxyEnabled ? '代理模式已启用' : '模拟模式已启用'}
                description={
                  isProxyEnabled
                    ? '请求将转发到目标 API 服务器，并完整记录请求与响应。'
                    : '服务器返回内置模拟数据，适合本地开发与测试。'
                }
                style={{ marginBottom: 24 }}
              />

              <Form.Item label="目标API服务器地址" name="targetURL">
                <Input
                  placeholder="例如: https://api.openai.com"
                  disabled={!isProxyEnabled}
                  prefix={<ApiOutlined style={{ color: '#999' }} />}
                />
              </Form.Item>

              <Form.Item label="快捷配置">
                <Space wrap>
                  <Button onClick={() => handleQuickConfig('https://api.openai.com')} disabled={!isProxyEnabled}>
                    <Tag color="geekblue" style={{ marginRight: 0 }}>OpenAI</Tag>
                  </Button>
                  <Button onClick={() => handleQuickConfig('https://api.deepseek.com')} disabled={!isProxyEnabled}>
                    <Tag color="cyan" style={{ marginRight: 0 }}>Deepseek</Tag>
                  </Button>
                  <Button onClick={() => handleQuickConfig('https://api.anthropic.com')} disabled={!isProxyEnabled}>
                    <Tag color="purple" style={{ marginRight: 0 }}>Anthropic</Tag>
                  </Button>
                </Space>
                <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
                  <Text type="secondary">
                    官方密钥获取：{' '}
                    <a href="https://platform.openai.com/signup" target="_blank" rel="noopener noreferrer">OpenAI</a> ·{' '}
                    <a href="https://platform.deepseek.com/usage" target="_blank" rel="noopener noreferrer">Deepseek</a> ·{' '}
                    <a href="https://console.anthropic.com/" target="_blank" rel="noopener noreferrer">Anthropic</a>
                  </Text>
                </Paragraph>
              </Form.Item>

              <Divider style={{ margin: '8px 0 24px' }} />

              <Form.Item label="认证类型" name="authType">
                <Select disabled={!isProxyEnabled} style={{ maxWidth: 320 }}>
                  <Option value="none">无认证</Option>
                  <Option value="basic">基本认证 (用户名/密码)</Option>
                  <Option value="token">令牌认证 (Bearer Token)</Option>
                </Select>
              </Form.Item>

              {authType && (
                <Paragraph type="secondary" style={{ marginTop: -12 }}>
                  {authDescriptions[authType] || ''}
                </Paragraph>
              )}

              <Form.Item noStyle shouldUpdate={(prev, curr) => prev.authType !== curr.authType}>
                {({ getFieldValue }) => {
                  const currentAuthType = getFieldValue('authType');
                  if (currentAuthType === 'basic') {
                    return (
                      <Row gutter={16}>
                        <Col xs={24} sm={12}>
                          <Form.Item label="用户名" name="username">
                            <Input placeholder="用户名" disabled={!isProxyEnabled} prefix={<SafetyCertificateOutlined style={{ color: '#999' }} />} />
                          </Form.Item>
                        </Col>
                        <Col xs={24} sm={12}>
                          <Form.Item label="密码" name="password">
                            <Input.Password placeholder="密码" disabled={!isProxyEnabled} />
                          </Form.Item>
                        </Col>
                      </Row>
                    );
                  }
                  if (currentAuthType === 'token') {
                    return (
                      <Form.Item label="访问令牌" name="token">
                        <Input.Password placeholder="例如: sk-xxxxxxxx" disabled={!isProxyEnabled} prefix={<KeyOutlined style={{ color: '#999' }} />} />
                      </Form.Item>
                    );
                  }
                  return null;
                }}
              </Form.Item>

              <Form.Item style={{ marginBottom: 0 }}>
                <Button type="primary" htmlType="submit" loading={saving} icon={<SafetyCertificateOutlined />}>
                  保存配置
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </Spin>
      </Space>
    </Layout>
  );
};

export default SettingsPage;