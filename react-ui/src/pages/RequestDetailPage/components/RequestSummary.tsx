import React from 'react';
import { Descriptions, Tag, Space } from 'antd';
import { ClockCircleOutlined } from '@ant-design/icons';

interface RequestSummaryProps {
  id: string;
  timestamp: string | number;
  latency?: number | null;
  statusCode?: number;
  host: string;
  protocol?: string;
  getStatusColor: (status: number) => string;
}

// 协议类型对应的标签颜色
const PROTOCOL_TAG_COLOR: Record<string, string> = {
  openai: 'geekblue',
  anthropic: 'purple',
};

const RequestSummary: React.FC<RequestSummaryProps> = ({
  id,
  timestamp,
  latency,
  statusCode = 200,
  host,
  protocol,
  getStatusColor
}) => {
  return (
    <Descriptions bordered column={{ xxl: 4, xl: 3, lg: 3, md: 2, sm: 1, xs: 1 }} style={{ marginTop: 24 }} className="detail-summary">
      <Descriptions.Item label="请求ID">
        <span className="detail-request-id">{id}</span>
      </Descriptions.Item>
      <Descriptions.Item label="时间">
        <Space>
          {new Date(timestamp).toLocaleString()}
          {latency != null && (
            <Tag color="blue" icon={<ClockCircleOutlined />}>
              {latency}ms
            </Tag>
          )}
        </Space>
      </Descriptions.Item>
      <Descriptions.Item label="状态码">
        <Tag color={getStatusColor(statusCode)}>
          {statusCode}
        </Tag>
      </Descriptions.Item>
      <Descriptions.Item label="Host">{host}</Descriptions.Item>
      <Descriptions.Item label="协议">
        <Tag color={PROTOCOL_TAG_COLOR[protocol || 'openai']} className="protocol-tag">
          {protocol === 'anthropic' ? 'Anthropic (Claude)' : 'OpenAI'}
        </Tag>
      </Descriptions.Item>
    </Descriptions>
  );
};

export default RequestSummary; 