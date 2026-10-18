import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, theme } from 'antd';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import RequestsPage from './pages/RequestsPage';
import RequestDetailPage from './pages/RequestDetailPage';
import SettingsPage from './pages/SettingsPage';
import GuidePage from './pages/GuidePage';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { NotificationProvider } from './components/Notification';
import './styles/Global.css';

// 受保护的路由组件
const ProtectedRoute: React.FC<{children: React.ReactNode}> = ({ children }) => {
  const { isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }

  return <>{children}</>;
};

// 全局主题配置：与 Global.css 中的设计变量保持一致的统一品牌风格
const themeConfig = {
  token: {
    colorPrimary: '#1890ff',
    colorInfo: '#1890ff',
    colorSuccess: '#27ae60',
    colorWarning: '#f39c12',
    colorError: '#e74c3c',
    colorBgLayout: '#f0f2f5',
    borderRadius: 6,
    fontSize: 14,
    fontFamily:
      "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Helvetica Neue', Helvetica, Arial, sans-serif",
  },
  components: {
    Layout: {
      headerBg: '#001529',
      bodyBg: '#f0f2f5',
    },
    Card: {
      headerFontSize: 16,
    },
    Table: {
      headerBg: '#fafafa',
      headerColor: '#2c3e50',
    },
  },
  algorithm: theme.defaultAlgorithm,
};

const App: React.FC = () => {
  return (
    <ConfigProvider theme={themeConfig}>
    <AuthProvider>
      <NotificationProvider>
        <Router basename="/ui">
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            
            <Route path="/" element={
              <ProtectedRoute>
                <RequestsPage />
              </ProtectedRoute>
            } />

            <Route path="/requests/:id" element={
              <ProtectedRoute>
                <RequestDetailPage />
              </ProtectedRoute>
            } />

            <Route path="/settings" element={
              <ProtectedRoute>
                <SettingsPage />
              </ProtectedRoute>
            } />

            <Route path="/guide" element={
              <ProtectedRoute>
                <GuidePage />
              </ProtectedRoute>
            } />
          </Routes>
        </Router>
      </NotificationProvider>
    </AuthProvider>
    </ConfigProvider>
  );
};

export default App; 