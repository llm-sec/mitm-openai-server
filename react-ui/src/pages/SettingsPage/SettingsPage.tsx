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
  Col
} from 'antd';
import Layout from '../../components/Layout/Layout';
import { ProxyConfig } from '../../types';
import { apiService } from '../../services/api';
import { useNotification } from '../../components/Notification/Notification';

const { Title, Paragraph, Link } = Typography;
const { Option } = Select;

const SettingsPage: React.FC = () => {
  const [form] = Form.useForm();
  const [proxyConfig, setProxyConfig] = useState<ProxyConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const { addNotification } = useNotification();
  
  const isProxyEnabled = Form.useWatch('enabled', form);

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

  return (
    <Layout title="系统设置">
      <Title level={2}>系统设置</Title>
      <Paragraph>配置代理服务器以将请求转发到实际的API端点。</Paragraph>
      <Spin spinning={loading}>
        <Card>
          <Form
            form={form}
            layout="vertical"
            onFinish={handleFinish}
            initialValues={proxyConfig || undefined}
          >
            <Form.Item label="代理模式" name="enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            
            <Paragraph type="secondary">
              {isProxyEnabled ? '代理模式已启用，请求将转发到目标API服务器。' : '代理模式已禁用，服务器将返回模拟数据。'}
            </Paragraph>

            <Form.Item label="目标API服务器地址" name="targetURL">
              <Input placeholder="例如: https://api.openai.com" disabled={!isProxyEnabled} />
            </Form.Item>
            
            <Form.Item label="快捷配置">
              <Space>
                <Button onClick={() => handleQuickConfig('https://api.openai.com')} disabled={!isProxyEnabled}>OpenAI</Button>
                <Button onClick={() => handleQuickConfig('https://api.deepseek.com')} disabled={!isProxyEnabled}>Deepseek</Button>
              </Space>
              <Paragraph type="secondary" style={{ marginTop: 8 }}>
                <Link href="https://platform.openai.com/signup" target="_blank">OpenAI API Key</Link> | <Link href="https://platform.deepseek.com/usage" target="_blank">Deepseek API Key</Link>
              </Paragraph>
            </Form.Item>

            <Form.Item label="认证类型" name="authType">
              <Select disabled={!isProxyEnabled}>
                <Option value="none">无认证</Option>
                <Option value="basic">基本认证 (用户名/密码)</Option>
                <Option value="token">令牌认证 (Bearer Token)</Option>
              </Select>
            </Form.Item>

            <Form.Item noStyle shouldUpdate={(prev, curr) => prev.authType !== curr.authType}>
              {({ getFieldValue }) => {
                const authType = getFieldValue('authType');
                if (authType === 'basic') {
                  return (
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item label="用户名" name="username">
                          <Input placeholder="用户名" disabled={!isProxyEnabled} />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item label="密码" name="password">
                          <Input.Password placeholder="密码" disabled={!isProxyEnabled} />
                        </Form.Item>
                      </Col>
                    </Row>
                  );
                }
                if (authType === 'token') {
                  return (
                    <Form.Item label="访问令牌" name="token">
                      <Input.Password placeholder="例如: sk-xxxxxxxx" disabled={!isProxyEnabled} />
                    </Form.Item>
                  );
                }
                return null;
              }}
            </Form.Item>
            
            <Form.Item>
              <Button type="primary" htmlType="submit" loading={saving}>
                保存配置
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </Spin>
    </Layout>
  );
};

export default SettingsPage; 