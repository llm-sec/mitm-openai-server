import React, { useState, useEffect } from 'react';
import { useNavigate, Link as RouterLink } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { utils } from '../../services/api';
import {
  Button,
  Checkbox,
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
  LoginOutlined,
  GithubOutlined,
  SafetyOutlined,
  ApiOutlined,
  FileSearchOutlined,
  ClusterOutlined
} from '@ant-design/icons';
import Footer from '../../components/Footer';

const { Title, Paragraph, Link, Text } = Typography;

const LoginPage: React.FC = () => {
  const [form] = Form.useForm();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);

  const { login } = useAuth();
  const navigate = useNavigate();

  // 从本地存储加载之前保存的用户名和密码
  useEffect(() => {
    const savedUsername = localStorage.getItem('mitm_remembered_username');
    const savedPassword = localStorage.getItem('mitm_remembered_password');
    const savedRememberMe = localStorage.getItem('mitm_remember_me');

    if (savedRememberMe === 'true') {
      setRememberMe(true);

      if (savedUsername && savedPassword) {
        form.setFieldsValue({
          username: savedUsername,
          password: savedPassword,
          remember: true
        });
      }
    } else {
      setRememberMe(false);
    }
  }, [form]);

  const handleSubmit = async (values: { username: string; password: string; remember: boolean }) => {
    const { username, password, remember } = values;

    setLoading(true);
    setError('');

    try {
      // 使用专门的登录API接口
      const response = await fetch('/ui/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, password })
      });

      const data = await response.json();

      if (response.ok && data.code === 0) {
        // 登录成功

        // 保存用户的"记住我"选择
        localStorage.setItem('mitm_remember_me', remember.toString());

        // 如果用户选择了"记住我"，保存凭据
        if (remember) {
          localStorage.setItem('mitm_remembered_username', username);
          localStorage.setItem('mitm_remembered_password', password);
        } else {
          // 清除之前可能存在的凭据
          localStorage.removeItem('mitm_remembered_username');
          localStorage.removeItem('mitm_remembered_password');
        }

        // 保存后端返回的token，设置为一年有效期
        if (data.data?.token) {
          utils.saveAuth(data.data.token, 365);
          login(username);
          navigate('/');
        } else {
          setError('服务器没有返回有效的认证令牌');
        }
      } else {
        setError(data.msg || '用户名或密码不正确');
      }
    } catch (err) {
      setError('登录失败，请稍后重试');
      console.error('登录错误:', err);
    } finally {
      setLoading(false);
    }
  };

  // 品牌特性列表
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
                  <Paragraph style={{ color: 'rgba(255, 255, 255, 0.55)' }}>
                    实时拦截、记录和检查 AI 交互的完整内容
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
              <Col xs={24} sm={24} md={14} style={{ padding: '48px 40px', backgroundColor: 'white' }}>
                <Title level={3} style={{ textAlign: 'center', marginBottom: 4 }}>系统登录</Title>
                <Paragraph type="secondary" style={{ textAlign: 'center', marginBottom: 24 }}>
                  登录以查看和管理 AI API 请求记录
                </Paragraph>
                {error && (
                  <Alert
                    message={error}
                    type="error"
                    showIcon
                    style={{ marginBottom: 24 }}
                  />
                )}
                <Form
                  form={form}
                  name="login"
                  initialValues={{ remember: rememberMe }}
                  onFinish={handleSubmit}
                  layout="vertical"
                  size="large"
                >
                  <Form.Item
                    name="username"
                    rules={[{ required: true, message: '请输入用户名' }]}
                  >
                    <Input
                      prefix={<UserOutlined />}
                      placeholder="用户名"
                      autoComplete="username"
                      disabled={loading}
                    />
                  </Form.Item>
                  <Form.Item
                    name="password"
                    rules={[{ required: true, message: '请输入密码' }]}
                  >
                    <Input.Password
                      prefix={<LockOutlined />}
                      placeholder="密码"
                      autoComplete="current-password"
                      disabled={loading}
                      iconRender={(visible) => (visible ? <EyeTwoTone /> : <EyeInvisibleOutlined />)}
                    />
                  </Form.Item>
                  <Form.Item style={{ marginBottom: 16 }}>
                    <Form.Item name="remember" valuePropName="checked" noStyle>
                      <Checkbox disabled={loading}>记住我</Checkbox>
                    </Form.Item>
                  </Form.Item>
                  <Form.Item style={{ marginBottom: 16 }}>
                    <Button
                      type="primary"
                      htmlType="submit"
                      loading={loading}
                      block
                      icon={<LoginOutlined />}
                    >
                      {loading ? '登录中...' : '登录'}
                    </Button>
                  </Form.Item>
                  <Text style={{ textAlign: 'center', display: 'block' }}>
                    还没有账号？ <RouterLink to="/register">立即注册</RouterLink>
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

export default LoginPage;