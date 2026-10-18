import React, { useState } from 'react';
import { useNavigate, Link as RouterLink } from 'react-router-dom';
import {
  Button,
  Form,
  Input,
  Card,
  Typography,
  Alert,
  Layout,
  Row,
  Col,
  Space
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  EyeTwoTone,
  EyeInvisibleOutlined,
  UserAddOutlined,
  LoginOutlined,
  SafetyOutlined,
  ApiOutlined,
  FileSearchOutlined,
  ClusterOutlined,
  GithubOutlined
} from '@ant-design/icons';
import Footer from '../../components/Footer';

const { Title, Paragraph, Text, Link } = Typography;

const RegisterPage: React.FC = () => {
  const [form] = Form.useForm();
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [loading, setLoading] = useState(false);

  const navigate = useNavigate();

  const handleSubmit = async (values: { username: string; password: string; confirmPassword: string }) => {
    const { username, password, confirmPassword } = values;

    // 确认密码验证
    if (password !== confirmPassword) {
      setError('两次输入的密码不一致');
      return;
    }

    setLoading(true);
    setError('');
    setSuccess('');

    try {
      const response = await fetch('/ui/api/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          username,
          password
        })
      });

      const data = await response.json();

      if (response.ok && data.code === 0) {
        // 注册成功
        setSuccess('注册成功！3秒后自动跳转到登录页面...');

        // 3秒后跳转到登录页面
        setTimeout(() => {
          navigate('/login');
        }, 3000);
      } else {
        setError(data.msg || '注册失败，请稍后重试');
      }
    } catch (err) {
      setError('注册失败，请稍后重试');
      console.error('注册错误:', err);
    } finally {
      setLoading(false);
    }
  };

  // 品牌特性列表（与登录页一致）
  const brandFeatures = [
    { icon: <ApiOutlined />, text: '支持 OpenAI & Claude 协议' },
    { icon: <FileSearchOutlined />, text: '实时记录与分析 AI 请求' },
    { icon: <ClusterOutlined />, text: 'TLS 握手指纹与安全监控' },
  ];

  return (
    <Layout style={{ minHeight: '100vh', backgroundColor: '#f0f2f5' }}>
      <Row justify="center" align="middle" style={{ minHeight: '100vh' }}>
        <Col xs={22} sm={20} md={16} lg={12} xl={10}>
          <Card
            style={{
              boxShadow: 'none',
              border: 'none',
              background: 'transparent',
              backgroundColor: 'transparent',
              borderRadius: '0',
              padding: '0'
            }}
            bodyStyle={{
              padding: '0',
              background: 'transparent',
              backgroundColor: 'transparent'
            }}
          >
            <Row style={{ overflow: 'hidden', borderRadius: '12px', boxShadow: '0 8px 24px rgba(0, 0, 0, 0.15)' }}>
              <Col xs={0} sm={0} md={10} style={{
                background: 'linear-gradient(160deg, #001529 0%, #0a2540 100%)',
                color: 'white',
                padding: '48px 36px',
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between'
              }}>
                <div>
                  <Space align="center" style={{ marginBottom: 32 }}>
                    <span
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: 44,
                        height: 44,
                        borderRadius: 10,
                        background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)',
                        fontSize: 22,
                        color: '#fff',
                        boxShadow: '0 2px 8px rgba(24, 144, 255, 0.4)'
                      }}
                    >
                      <SafetyOutlined />
                    </span>
                    <Title level={3} style={{ color: 'white', margin: 0 }}>MITM Server</Title>
                  </Space>
                  <Title level={4} style={{ color: 'white', marginBottom: 8 }}>中间人 AI API 服务器</Title>
                  <Paragraph style={{ color: 'rgba(255, 255, 255, 0.75)', marginBottom: 4 }}>
                    安全监控与分析 API 请求的强大工具
                  </Paragraph>
                </div>
                <Space direction="vertical" size={12}>
                  {brandFeatures.map((f, i) => (
                    <Text key={i} style={{ color: 'rgba(255, 255, 255, 0.75)', display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ color: '#1890ff', fontSize: 16 }}>{f.icon}</span>
                      {f.text}
                    </Text>
                  ))}
                  <Link
                    href="https://github.com/llm-sec/mitm-openai-server"
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ color: 'rgba(255, 255, 255, 0.85)', marginTop: 8 }}
                  >
                    <GithubOutlined /> GitHub 项目仓库
                  </Link>
                </Space>
              </Col>
              <Col xs={24} sm={24} md={14} style={{ padding: '40px 40px', backgroundColor: 'white' }}>
                <Title level={3} style={{ textAlign: 'center', marginBottom: 4 }}>创建新账号</Title>
                <Paragraph type="secondary" style={{ textAlign: 'center', marginBottom: 24 }}>
                  注册后即可查看和管理 AI API 请求记录
                </Paragraph>
                {error && (
                  <Alert
                    message={error}
                    type="error"
                    showIcon
                    style={{ marginBottom: 24 }}
                  />
                )}

                {success && (
                  <Alert
                    message={success}
                    type="success"
                    showIcon
                    style={{ marginBottom: 24 }}
                  />
                )}

                <Form
                  form={form}
                  name="register"
                  onFinish={handleSubmit}
                  layout="vertical"
                  size="large"
                >
                  <Form.Item
                    name="username"
                    rules={[
                      { required: true, message: '请输入用户名' },
                      { min: 3, message: '用户名至少需要3个字符' },
                      { pattern: /^[a-zA-Z0-9_-]+$/, message: '用户名只能包含字母、数字、下划线和横线' }
                    ]}
                  >
                    <Input
                      prefix={<UserOutlined />}
                      placeholder="用户名（至少3个字符）"
                      autoComplete="username"
                      disabled={loading}
                    />
                  </Form.Item>
                  <Form.Item
                    name="password"
                    rules={[
                      { required: true, message: '请输入密码' },
                      { min: 6, message: '密码至少需要6个字符' }
                    ]}
                  >
                    <Input.Password
                      prefix={<LockOutlined />}
                      placeholder="密码（至少6个字符）"
                      autoComplete="new-password"
                      disabled={loading}
                      iconRender={(visible) => (visible ? <EyeTwoTone /> : <EyeInvisibleOutlined />)}
                    />
                  </Form.Item>
                  <Form.Item
                    name="confirmPassword"
                    dependencies={['password']}
                    rules={[
                      { required: true, message: '请确认密码' },
                      ({ getFieldValue }) => ({
                        validator(_, value) {
                          if (!value || getFieldValue('password') === value) {
                            return Promise.resolve();
                          }
                          return Promise.reject(new Error('两次输入的密码不一致'));
                        },
                      }),
                    ]}
                  >
                    <Input.Password
                      prefix={<LockOutlined />}
                      placeholder="确认密码"
                      autoComplete="new-password"
                      disabled={loading}
                      iconRender={(visible) => (visible ? <EyeTwoTone /> : <EyeInvisibleOutlined />)}
                    />
                  </Form.Item>
                  <Form.Item style={{ marginBottom: 16 }}>
                    <Button
                      type="primary"
                      htmlType="submit"
                      loading={loading}
                      block
                      icon={<UserAddOutlined />}
                    >
                      {loading ? '注册中...' : '创建账号'}
                    </Button>
                  </Form.Item>
                  <Text style={{ textAlign: 'center', display: 'block' }}>
                    已有账号？ <RouterLink to="/login"><LoginOutlined /> 立即登录</RouterLink>
                  </Text>
                </Form>
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>
      <Footer />
    </Layout>
  );
};

export default RegisterPage;