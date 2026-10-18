import React, { useMemo } from 'react';
import { Card, Button, Tooltip, Typography } from 'antd';
import { CopyOutlined } from '@ant-design/icons';
import JSONPretty from 'react-json-pretty';
import 'react-json-pretty/themes/monikai.css';

const { Text } = Typography;

interface JsonCardViewProps {
  title: string;
  data: any;
  copyToClipboard: (text: any) => void;
  style?: React.CSSProperties;
  jsonPrettyTheme: any;
}

const JsonCardView: React.FC<JsonCardViewProps> = ({
  title,
  data,
  copyToClipboard,
  style,
  jsonPrettyTheme
}) => {
  const renderCardExtra = () => (
    <Tooltip title={`复制${title}内容到剪贴板`} placement="left">
      <Button 
        type="text" 
        icon={<CopyOutlined />} 
        onClick={() => copyToClipboard(data)}
        size="small"
        title={`复制${title}`}
      />
    </Tooltip>
  );

  const hasData = data && (
    typeof data === 'object' ? Object.keys(data).length > 0 : true
  );

  const processedData = useMemo(() => {
    if (typeof data !== 'object' || data === null) {
      return data;
    }
    
    try {
      return data;
    } catch (error) {
      console.error('JSON处理错误:', error);
      return '数据格式化错误';
    }
  }, [data]);

  return (
    <Card
      size="small"
      title={title}
      style={style}
      extra={renderCardExtra()}
      styles={{ body: { padding: 0 } }}
      className="json-card"
    >
      {hasData ? (
        <div className="json-code-block">
          <JSONPretty
            id={`json-pretty-${title.replace(/\s+/g, '-').toLowerCase()}`}
            data={processedData}
            theme={jsonPrettyTheme}
            mainStyle="padding: 1em; line-height: 1.3;"
          />
        </div>
      ) : (
        <div style={{ padding: '16px 24px' }}>
          <Text type="secondary">无数据</Text>
        </div>
      )}
    </Card>
  );
};

export default JsonCardView; 