import React from 'react';
import { Space, Tag, Button, Tooltip, Typography, Row, Col } from 'antd';
import {
  ArrowLeftOutlined,
  ArrowRightOutlined,
  DeleteOutlined,
  RollbackOutlined
} from '@ant-design/icons';
import ExportButton from './ExportButton';

const { Title } = Typography;

interface RequestHeaderProps {
  method: string;
  path: string;
  prevRequestId: string | null;
  nextRequestId: string | null;
  navigateToPrevious: () => void;
  navigateToNext: () => void;
  navigateToList: () => void;
  handleDelete: () => void;
  getMethodColor: (method: string) => string;
  requestData: any;
  requestId: string;
}

const RequestHeader: React.FC<RequestHeaderProps> = ({
  method,
  path,
  prevRequestId,
  nextRequestId,
  navigateToPrevious,
  navigateToNext,
  navigateToList,
  handleDelete,
  getMethodColor,
  requestData,
  requestId
}) => {
  return (
    <Row justify="space-between" align="middle" gutter={[12, 12]} wrap>
      <Col flex="auto" style={{ minWidth: 0 }}>
        <Space align="center" size="small" style={{ width: '100%' }}>
          <Tag color={getMethodColor(method)} style={{ fontSize: 15, padding: '3px 8px', borderRadius: 4, flexShrink: 0 }}>
            {method}
          </Tag>
          <Title level={4} ellipsis={{ tooltip: path }} style={{ margin: 0, minWidth: 0 }}>
            {path}
          </Title>
        </Space>
      </Col>
      <Col flex="none">
        <Space size="middle" wrap>
          <Space.Compact>
            <Tooltip title={prevRequestId ? "查看上一个请求" : "没有上一个请求"}>
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={navigateToPrevious}
                disabled={!prevRequestId}
              />
            </Tooltip>
            <Tooltip title={nextRequestId ? "查看下一个请求" : "没有下一个请求"}>
              <Button
                icon={<ArrowRightOutlined />}
                onClick={navigateToNext}
                disabled={!nextRequestId}
              />
            </Tooltip>
          </Space.Compact>
          <Tooltip title="返回请求列表">
            <Button
              icon={<RollbackOutlined />}
              onClick={navigateToList}
            />
          </Tooltip>
          <Tooltip title="删除此请求">
            <Button
              icon={<DeleteOutlined />}
              onClick={handleDelete}
              danger
            />
          </Tooltip>
          <ExportButton data={requestData} requestId={requestId} />
        </Space>
      </Col>
    </Row>
  );
};

export default RequestHeader;