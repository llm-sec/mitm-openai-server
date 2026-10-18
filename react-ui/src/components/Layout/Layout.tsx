import React from 'react';
import { Layout as AntLayout } from 'antd';
import Navbar from '../Navbar';
import Footer from '../Footer';

const { Content, Footer: AntFooter } = AntLayout;

const Layout: React.FC<{
  children: React.ReactNode;
  title?: string;
}> = ({ children, title }) => {
  // 设置页面标题
  React.useEffect(() => {
    if (title) {
      document.title = `${title} - 中间人AI API服务器`;
    } else {
      document.title = '中间人AI API服务器';
    }
  }, [title]);

  return (
    <AntLayout>
      <Navbar />
      <Content style={{
        marginTop: 63, // 导航栏高度(60) + 装饰线(3)
        minHeight: 'calc(100vh - 63px)',
      }}>
        <div
          style={{
            maxWidth: 1400,
            margin: '0 auto',
            padding: '24px 48px 16px',
            minHeight: 'calc(100vh - 200px)',
          }}
        >
          {children}
        </div>
      </Content>
      <AntFooter style={{
        padding: 0,
        backgroundColor: 'transparent',
      }}>
        <Footer />
      </AntFooter>
    </AntLayout>
  );
};

export default Layout;