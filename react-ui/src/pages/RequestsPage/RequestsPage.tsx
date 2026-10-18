import React, { useState, useEffect, useRef, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Input,
  Space,
  Tag,
  message,
  Tooltip,
  Switch,
  InputNumber,
  Modal,
  Typography,
  Empty,
  Card,
  Statistic,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  SearchOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  ExclamationCircleOutlined,
  ApiOutlined,
  RobotOutlined,
  ThunderboltOutlined,
  WarningOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { RequestRecord } from '../../types/RequestRecord';
import { RequestService } from '../../services/RequestService';
import Layout from '../../components/Layout/Layout';
import './RequestsPage.css';

// 协议标签颜色映射
const PROTOCOL_TAG: Record<string, { color: string; label: string }> = {
  openai: { color: 'geekblue', label: 'OpenAI' },
  anthropic: { color: 'purple', label: 'Anthropic' },
};

// 从请求记录中提取协议类型（默认视为 openai，兼容历史数据）
const getProtocol = (record: RequestRecord): string =>
  (record.metadata && record.metadata.protocol) || 'openai';

// 从请求记录中提取响应状态码（兼容 status_code 与 statusCode 两种字段命名）
const getStatusCode = (record: RequestRecord): number | null => {
  const resp = record.response as any;
  if (resp) {
    if (typeof resp.statusCode === 'number') {
      return resp.statusCode;
    }
    if (typeof resp.status_code === 'number') {
      return resp.status_code;
    }
  }
  return null;
};

// HTTP 方法对应的圆点颜色
const METHOD_DOT: Record<string, string> = {
  GET: '#52c41a',
  POST: '#1890ff',
  PUT: '#faad14',
  DELETE: '#ff4d4f',
  PATCH: '#722ed1',
};

// 从localStorage获取页面大小设置，默认为20
const getStoredPageSize = (): number => {
  const storedSize = localStorage.getItem('requestsPageSize');
  return storedSize ? parseInt(storedSize, 10) : 20;
};

// 从localStorage获取自动刷新设置，默认为true
const getStoredAutoRefresh = (): boolean => {
  const storedAutoRefresh = localStorage.getItem('requestsAutoRefresh');
  return storedAutoRefresh === null ? true : storedAutoRefresh === 'true';
};

// 从localStorage获取刷新间隔设置，默认为3秒
const getStoredRefreshInterval = (): number => {
  const storedInterval = localStorage.getItem('requestsRefreshInterval');
  return storedInterval ? parseInt(storedInterval, 10) : 3;
};

const RequestsPage: React.FC = () => {
  const navigate = useNavigate();
  const searchInput = useRef<any>(null);
  const [, setSearchText] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [requestRecords, setRequestRecords] = useState<RequestRecord[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [pageSize, setPageSize] = useState<number>(getStoredPageSize());
  const [autoRefresh, setAutoRefresh] = useState<boolean>(getStoredAutoRefresh());
  const [refreshInterval, setRefreshInterval] = useState<number>(getStoredRefreshInterval());
  const refreshTimerRef = useRef<NodeJS.Timeout | null>(null);

  // 加载请求数据
  const loadRequests = async () => {
    try {
      setLoading(true);
      const { requests, total: totalCount } = await RequestService.getRequestsWithTotal();
      setRequestRecords(requests);
      setTotal(totalCount);
    } catch (error) {
      console.error('Failed to load requests:', error);
      message.error('加载请求数据失败');
    } finally {
      setLoading(false);
    }
  };

  // 初始化加载
  useEffect(() => {
    loadRequests();
  }, []);

  // 自动刷新定时器
  useEffect(() => {
    // 清除现有定时器
    if (refreshTimerRef.current) {
      clearInterval(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }

    // 如果自动刷新开启，设置新定时器
    if (autoRefresh && refreshInterval > 0) {
      refreshTimerRef.current = setInterval(() => {
        loadRequests();
      }, refreshInterval * 1000);
    }

    // 组件卸载时清除定时器
    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
      }
    };
  }, [autoRefresh, refreshInterval]);

  // 统计当前加载请求的协议分布与失败数量
  const stats = useMemo(() => {
    let anthropic = 0;
    let failed = 0;
    for (const r of requestRecords) {
      if (getProtocol(r) === 'anthropic') {
        anthropic += 1;
      }
      const sc = getStatusCode(r);
      if (sc !== null && sc >= 400) {
        failed += 1;
      }
    }
    return {
      total,
      openai: total - anthropic,
      anthropic,
      failed,
    };
  }, [requestRecords, total]);

  // 统计卡片配置
  const statCards = [
    {
      key: 'total',
      label: '总请求数',
      value: stats.total,
      icon: <ApiOutlined />,
      iconClass: 'stats-icon-blue',
      tip: '服务器记录的 API 请求总数',
    },
    {
      key: 'openai',
      label: 'OpenAI',
      value: stats.openai,
      icon: <RobotOutlined />,
      iconClass: 'stats-icon-geekblue',
      tip: 'OpenAI 协议请求数',
    },
    {
      key: 'anthropic',
      label: 'Anthropic',
      value: stats.anthropic,
      icon: <ThunderboltOutlined />,
      iconClass: 'stats-icon-purple',
      tip: 'Anthropic (Claude) 协议请求数',
    },
    {
      key: 'failed',
      label: '失败请求',
      value: stats.failed,
      icon: <WarningOutlined />,
      iconClass: 'stats-icon-red',
      tip: '状态码 ≥ 400 的请求数',
    },
  ];

  // 处理搜索输入
  const handleSearch = (selectedKeys: string[], confirm: Function) => {
    confirm();
    setSearchText(selectedKeys[0]);
  };

  // 重置搜索
  const handleReset = (clearFilters: Function) => {
    clearFilters();
    setSearchText('');
  };

  // 获取列搜索属性
  const getColumnSearchProps = (dataIndex: string) => ({
    filterDropdown: ({
      setSelectedKeys,
      selectedKeys,
      confirm,
      clearFilters,
    }: any) => (
      <div style={{ padding: 8 }}>
        <Input
          ref={searchInput}
          placeholder={`搜索 ${dataIndex}`}
          value={selectedKeys[0]}
          onChange={(e) =>
            setSelectedKeys(e.target.value ? [e.target.value] : [])
          }
          onPressEnter={() => handleSearch(selectedKeys, confirm)}
          style={{ marginBottom: 8, display: 'block' }}
        />
        <Space>
          <Button
            type="primary"
            onClick={() => handleSearch(selectedKeys, confirm)}
            icon={<SearchOutlined />}
            size="small"
            style={{ width: 90 }}
          >
            搜索
          </Button>
          <Button
            onClick={() => handleReset(clearFilters)}
            size="small"
            style={{ width: 90 }}
          >
            重置
          </Button>
        </Space>
      </div>
    ),
    filterIcon: (filtered: boolean) => (
      <SearchOutlined style={{ color: filtered ? '#1890ff' : undefined }} />
    ),
    onFilter: (value: any, record: RequestRecord) =>
      record[dataIndex as keyof RequestRecord]
        ?.toString()
        .toLowerCase()
        .includes(value.toString().toLowerCase()) || false,
    filterDropdownProps: {
      onOpenChange: (visible: boolean) => {
        if (visible) {
          setTimeout(() => searchInput.current?.select(), 100);
        }
      }
    },
    render: (text: string) => (
      <Tooltip title={text}>{text}</Tooltip>
    ),
  });

  // 处理页面大小变更
  const handlePageSizeChange = (value: number) => {
    setPageSize(value);
    localStorage.setItem('requestsPageSize', value.toString());
  };

  // 处理自动刷新开关变更
  const handleAutoRefreshChange = (checked: boolean) => {
    setAutoRefresh(checked);
    localStorage.setItem('requestsAutoRefresh', checked.toString());
  };

  // 处理刷新间隔变更
  const handleRefreshIntervalChange = (value: number | null) => {
    if (value !== null && value > 0) {
      setRefreshInterval(value);
      localStorage.setItem('requestsRefreshInterval', value.toString());
    }
  };

  // 删除所有请求记录
  const handleDeleteAll = () => {
    Modal.confirm({
      title: '确认清空',
      icon: <ExclamationCircleOutlined />,
      content: '确定要清空所有请求记录吗？此操作不可恢复。',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          await RequestService.deleteAllRequests();
          message.success('所有请求记录已删除');
          setRequestRecords([]);
          setTotal(0);
        } catch (error) {
          console.error('Failed to delete all requests:', error);
          message.error('删除请求记录失败');
        }
      }
    });
  };

  // 表格列定义
  const columns: ColumnsType<RequestRecord> = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: '10%',
      render: (id: string) => (
        <Tooltip title={id}>
          <span className="request-id-cell">{id.slice(0, 8)}</span>
        </Tooltip>
      ),
    },
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: '11%',
      render: (timestamp: string) => {
        if (!timestamp) {
          return <span>-</span>;
        }
        const date = new Date(timestamp);
        return (
          <Tooltip title={date.toLocaleString()}>
            <span>{date.toLocaleTimeString()}</span>
          </Tooltip>
        );
      },
      sorter: (a, b) => {
        if (!a.timestamp || !b.timestamp) {
          return 0;
        }
        const dateA = new Date(a.timestamp).getTime();
        const dateB = new Date(b.timestamp).getTime();
        return dateA - dateB;
      },
      defaultSortOrder: 'descend',
      showSorterTooltip: false,
    },
    {
      title: '协议',
      dataIndex: 'metadata',
      key: 'protocol',
      width: '10%',
      filters: [
        { text: 'OpenAI', value: 'openai' },
        { text: 'Anthropic', value: 'anthropic' },
      ],
      onFilter: (value, record) => getProtocol(record) === value,
      render: (_, record) => {
        const protocol = getProtocol(record);
        const { color, label } = PROTOCOL_TAG[protocol] || PROTOCOL_TAG.openai;
        return <Tag color={color} style={{ borderRadius: 4 }}>{label}</Tag>;
      },
    },
    {
      title: '方法',
      dataIndex: 'method',
      key: 'method',
      width: '8%',
      filters: [
        { text: 'GET', value: 'GET' },
        { text: 'POST', value: 'POST' },
        { text: 'PUT', value: 'PUT' },
        { text: 'DELETE', value: 'DELETE' },
      ],
      onFilter: (value, record) => record.method === value,
      render: (method: string) => {
        if (!method) {
          return <Tag color="default">未知</Tag>;
        }
        const m = method.toUpperCase();
        const dotColor = METHOD_DOT[m] || '#8c8c8c';
        return (
          <Tooltip title={method}>
            <span>
              <span className="method-dot" style={{ backgroundColor: dotColor }} />
              {m}
            </span>
          </Tooltip>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'response',
      key: 'status',
      width: '8%',
      render: (_, record) => {
        const sc = getStatusCode(record);
        if (sc === null) {
          return <Typography.Text type="secondary">-</Typography.Text>;
        }
        let color = 'default';
        if (sc < 300) color = 'success';
        else if (sc < 400) color = 'processing';
        else if (sc < 500) color = 'warning';
        else color = 'error';
        return (
          <Tooltip title={`HTTP ${sc}`}>
            <Tag color={color} style={{ borderRadius: 4 }}>{sc}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: '路径',
      dataIndex: 'path',
      key: 'path',
      ellipsis: true,
      ...getColumnSearchProps('path'),
      width: '15%',
    },
    {
      title: '消息内容预览',
      dataIndex: 'body',
      key: 'body',
      ellipsis: true,
      render: (_, record) => {
        try {
          const body = record.body;

          if (!body) {
            return <span className="request-body-preview">-</span>;
          }

          let content = '';

          if (typeof body === 'string') {
            content = body;
          } else if (body.messages && Array.isArray(body.messages) && body.messages.length > 0) {
            // 如果是 Chat API 请求（OpenAI 或 Anthropic/Claude Code）
            const lastMessage = body.messages[body.messages.length - 1];
            // content 可能是字符串（OpenAI）或内容块数组（Anthropic: [{type:"text",text:"..."}]）
            if (typeof lastMessage.content === 'string') {
              content = lastMessage.content;
            } else if (Array.isArray(lastMessage.content)) {
              content = lastMessage.content
                .filter((block: any) => block && block.type === 'text' && block.text)
                .map((block: any) => block.text)
                .join('');
            } else {
              content = lastMessage.content || '';
            }
          } else if (body.prompt) {
            // 如果是 Completions API 请求
            content = body.prompt;
          } else {
            content = JSON.stringify(body);
          }

          if (!content) {
            return <span className="request-body-preview">-</span>;
          }

          return (
            <Tooltip title={content}>
              <span className="request-body-preview">{content}</span>
            </Tooltip>
          );
        } catch (error) {
          return <span className="request-body-preview">解析错误</span>;
        }
      },
    },
    {
      title: '操作',
      key: 'action',
      width: '8%',
      render: (_, record) => (
        <Button
          type="link"
          size="small"
          className="row-action-btn"
          onClick={(e) => {
            e.stopPropagation();
            navigate(`/requests/${record.id}`);
          }}
        >
          详情
        </Button>
      ),
    },
  ];

  // 自动刷新控制条（用于表格卡片头部）
  const autoRefreshControl = (
    <Space>
      <ClockCircleOutlined style={{ color: '#8c8c8c' }} />
      <Switch
        size="small"
        checkedChildren="自动刷新"
        unCheckedChildren="自动刷新"
        checked={autoRefresh}
        onChange={handleAutoRefreshChange}
      />
      {autoRefresh && (
        <InputNumber
          size="small"
          min={1}
          max={60}
          value={refreshInterval}
          onChange={handleRefreshIntervalChange}
          addonAfter="秒"
          style={{ width: 90 }}
        />
      )}
    </Space>
  );

  return (
    <Layout title="请求列表">
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* 页头 */}
        <div className="requests-page-header">
          <div>
            <Typography.Title level={2} className="requests-page-title">
              请求列表
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="requests-page-subtitle">
              实时监控与记录经过本服务器的 OpenAI / Claude API 请求
            </Typography.Paragraph>
          </div>
          <Space>
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              loading={loading}
              onClick={loadRequests}
            >
              刷新
            </Button>
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={handleDeleteAll}
            >
              清空
            </Button>
          </Space>
        </div>

        {/* 统计卡片 */}
        <div className="stats-row">
          {statCards.map((card) => (
            <Card key={card.key} className="stats-card" styles={{ body: { padding: 0 } }}>
              <Tooltip title={card.tip}>
                <div className="stats-card-inner">
                  <div className={`stats-icon ${card.iconClass}`}>
                    {card.icon}
                  </div>
                  <div>
                    <Statistic
                      value={card.value}
                      valueStyle={{ fontSize: 26, fontWeight: 700, color: '#2c3e50', lineHeight: 1.2 }}
                      suffix=""
                    />
                    <div className="stats-label">{card.label}</div>
                  </div>
                </div>
              </Tooltip>
            </Card>
          ))}
        </div>

        {/* 请求记录表格 */}
        <Card
          title={
            <Space>
              <DatabaseOutlined style={{ color: '#1890ff' }} />
              <span>请求记录</span>
              <Tag color="blue" style={{ borderRadius: 4 }}>
                {total} 条
              </Tag>
            </Space>
          }
          extra={autoRefreshControl}
          style={{ borderRadius: 12 }}
        >
          <Table
            columns={columns}
            dataSource={requestRecords}
            rowKey="id"
            pagination={{
              pageSize: pageSize,
              showSizeChanger: true,
              pageSizeOptions: ['10', '20', '50', '100'],
              onShowSizeChange: (_, size) => handlePageSizeChange(size),
              showTotal: (t) => `共 ${t} 条记录`,
              showQuickJumper: true,
            }}
            loading={loading}
            locale={{
              emptyText: (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="暂无请求记录"
                  style={{ padding: '40px 0' }}
                >
                  <Typography.Text type="secondary" style={{ fontSize: 13 }}>
                    发送到本服务器的 API 请求将在此展示
                  </Typography.Text>
                </Empty>
              ),
            }}
            onRow={(record) => ({
              onClick: () => navigate(`/requests/${record.id}`),
              style: { cursor: 'pointer' }
            })}
            showSorterTooltip={false}
            tableLayout="fixed"
            size="middle"
          />
        </Card>
      </Space>
    </Layout>
  );
};

export default RequestsPage;