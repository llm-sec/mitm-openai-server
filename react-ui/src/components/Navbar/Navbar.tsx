import React, { useState, useEffect } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { Typography, Dropdown, Avatar } from 'antd';
import { 
  HomeOutlined, 
  SettingOutlined, 
  BookOutlined,
  LogoutOutlined,
  GithubOutlined,
  UserOutlined
} from '@ant-design/icons';
import { useAuth } from '../../contexts/AuthContext';
import './Navbar.css';

const { Title } = Typography;

const Navbar: React.FC = () => {
  const { isAuthenticated, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [current, setCurrent] = useState('');

  // 根据当前路径更新选中的菜单项
  useEffect(() => {
    const pathname = location.pathname;
    if (pathname === '/') {
      setCurrent('home');
    } else if (pathname.startsWith('/guide')) {
      setCurrent('guide');
    } else if (pathname.startsWith('/settings')) {
      setCurrent('settings');
    } else if (pathname.startsWith('/requests/')) {
      setCurrent('home'); // 查看请求详情时，仍然高亮"首页"
    }
  }, [location]);

  // 如果未认证，不显示导航栏
  if (!isAuthenticated) {
    return null;
  }

  // 菜单项定义
  const menuItems = [
    {
      key: 'home',
      icon: <HomeOutlined />,
      label: '请求列表',
      onClick: () => navigate('/')
    },
    {
      key: 'guide',
      icon: <BookOutlined />,
      label: 'OpenAI API配置教程',
      onClick: () => navigate('/guide')
    },
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: '系统设置',
      onClick: () => navigate('/settings')
    }
  ];

  // 用户下拉菜单
  const userMenuItems = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: () => logout()
    }
  ];

  // 渲染GitHub链接
  const renderGithubLink = () => (
    <a 
      href="https://github.com/llm-sec/mitm-openai-server" 
      target="_blank" 
      rel="noopener noreferrer"
      className="github-link"
    >
      <span className="navbar-icon"><GithubOutlined /></span>
      <span>GitHub</span>
    </a>
  );

  // 渲染用户头像
  const renderUserAvatar = () => (
    <Dropdown 
      menu={{ items: userMenuItems }} 
      placement="bottomRight"
      trigger={['click']}
    >
      <Avatar
        className="user-avatar"
        icon={<UserOutlined />}
        style={{ 
          cursor: 'pointer',
          backgroundColor: '#1890ff',
          marginRight: 0
        }}
      />
    </Dropdown>
  );

  return (
    <nav className="navbar">
      <div className="navbar-container">
        {/* 左侧品牌和菜单 */}
        <div className="navbar-left">
          <Link to="/" className="navbar-brand">
            <Title level={4}>中间人OpenAI API服务器</Title>
          </Link>
          <ul className="navbar-menu">
            {menuItems.map(item => (
              <li key={item.key} className="navbar-item">
                <a
                  className={`navbar-link ${current === item.key ? 'active' : ''}`}
                  onClick={item.onClick}
                >
                  <span className="navbar-icon">{item.icon}</span>
                  <span>{item.label}</span>
                </a>
              </li>
            ))}
          </ul>
        </div>
        
        {/* 右侧GitHub链接和用户头像 */}
        <div className="navbar-right">
          <div className="github-container">
            {renderGithubLink()}
          </div>
          <div className="avatar-container">
            {renderUserAvatar()}
          </div>
        </div>
      </div>
    </nav>
  );
};

export default Navbar; 