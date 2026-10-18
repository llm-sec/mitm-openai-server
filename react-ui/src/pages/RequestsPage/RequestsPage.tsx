import React, { useState, useEffect, useRef } from 'react';
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
  Row,
  Col,
  Typography
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  SearchOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  ExclamationCircleOutlined
} from '@ant-design/icons';
import { RequestRecord } from '../../types/RequestRecord';
import { RequestService } from '../../services/RequestService';
import Layout from '../../components/Layout/Layout';

const { Title } = Typography;

// 从localStorage获取页面大小设置，默认为10
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
  const [searchedColumn, setSearchedColumn] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [requestRecords, setRequestRecords] = useState<RequestRecord[]>([]);
  const [pageSize, setPageSize] = useState<number>(getStoredPageSize());
  const [autoRefresh, setAutoRefresh] = useState<boolean>(getStoredAutoRefresh());
  const [refreshInterval, setRefreshInterval] = useState<number>(getStoredRefreshInterval());
  const refreshTimerRef = useRef<NodeJS.Timeout | null>(null);

  // 加载请求数据
  const loadRequests = async () => {
    try {
      setLoading(true);
      const data = await RequestService.getRequests();
      setRequestRecords(data);
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

  // 处理搜索输入
  const handleSearch = (
    selectedKeys: string[],
    confirm: Function,
    dataIndex: string
  ) => {
    confirm();
    setSearchText(selectedKeys[0]);
    setSearchedColumn(dataIndex);
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
          onPressEnter={() => handleSearch(selectedKeys, confirm, dataIndex)}
          style={{ marginBottom: 8, display: 'block' }}
        />
        <Space>
          <Button
            type="primary"
            onClick={() => handleSearch(selectedKeys, confirm, dataIndex)}
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
    render: (text: string) =>
      searchedColumn === dataIndex ? (
        <Tooltip title={text}>{text}</Tooltip>
      ) : (
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
      ellipsis: true,
      ...getColumnSearchProps('id'),
      width: '18%',
    },
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: '12%',
      render: (timestamp: string) => {
        if (!timestamp) {
          return <span>-</span>;
        }
        const date = new Date(timestamp);
        return (
          <Tooltip title={date.toLocaleString()}>
            <span>{date.toLocaleString()}</span>
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
            return <span className="request-body">-</span>;
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
          
          return (
            <Tooltip title={content}>
              <span className="request-body">{content}</span>
            </Tooltip>
          );
        } catch (error) {
          return <span className="request-body">解析错误</span>;
        }
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
        
        let color = 'default';
        switch (method.toUpperCase()) {
          case 'GET':
            color = 'success';
            break;
          case 'POST':
            color = 'processing';
            break;
          case 'PUT':
            color = 'warning';
            break;
          case 'DELETE':
            color = 'error';
            break;
        }
        return <Tag color={color}>{method}</Tag>;
      },
    },
    {
      title: '操作',
      key: 'action',
      width: '8%',
      render: (_, record) => (
        <Button 
          type="primary" 
          size="small" 
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

  return (
    <Layout title="请求列表">
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Row justify="space-between" align="middle">
          <Col>
            <Title level={2}>请求列表</Title>
          </Col>
          <Col>
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
          </Col>
        </Row>

        <Row justify="end">
          <Col>
            <Space>
              <Switch
                checkedChildren="自动刷新"
                unCheckedChildren="自动刷新"
                checked={autoRefresh}
                onChange={handleAutoRefreshChange}
              />
              {autoRefresh && (
                <Space>
                  <ClockCircleOutlined />
                  <InputNumber
                    min={1}
                    max={60}
                    value={refreshInterval}
                    onChange={handleRefreshIntervalChange}
                    addonAfter="秒"
                  />
                </Space>
              )}
            </Space>
          </Col>
        </Row>

        <Table
          columns={columns}
          dataSource={requestRecords}
          rowKey="id"
          pagination={{ 
            pageSize: pageSize,
            showSizeChanger: true,
            pageSizeOptions: ['10', '20', '50', '100'],
            onShowSizeChange: (_, size) => handlePageSizeChange(size),
            showTotal: (total) => `共 ${total} 条记录`
          }}
          loading={loading}
          onRow={(record) => ({
            onClick: () => navigate(`/requests/${record.id}`),
            style: { cursor: 'pointer' }
          })}
          showSorterTooltip={false}
          tableLayout="fixed"
        />
      </Space>
    </Layout>
  );
};

export default RequestsPage; 