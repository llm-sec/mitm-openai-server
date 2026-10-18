import axios from 'axios';
import { Request, StorageStats, ProxyConfig } from '../types';

// API基础配置
const API = axios.create({
  baseURL: '/ui/api',
  headers: {
    'Content-Type': 'application/json'
  },
  withCredentials: true // 启用跨域凭证支持
});

// 拦截器添加认证信息
API.interceptors.request.use(config => {
  // 从多个来源获取token
  let token = localStorage.getItem('auth_token');
  
  if (!token) {
    token = sessionStorage.getItem('auth_token');
  }
  
  if (!token) {
    const cookieMatch = document.cookie.match(/auth_token=([^;]+)/);
    if (cookieMatch && cookieMatch[1]) {
      token = cookieMatch[1];
    }
  }
  
  if (token) {
    // 使用Bearer格式
    config.headers.Authorization = `Bearer ${token}`;
    // 同时尝试使用自定义头，某些代理可能会保留这个
    config.headers['X-Auth-Token'] = token;
  } else {
    console.warn('发送请求时未找到认证令牌! 这将导致401错误');
  }
  
  return config;
}, error => {
  console.error('请求拦截器错误:', error);
  return Promise.reject(error);
});

// 拦截器处理认证失败
API.interceptors.response.use(
  response => {
    return response;
  },
  error => {
    console.error('API错误响应:', error);
    
    if (error.response) {
      console.error(`响应状态: ${error.response.status}`);
      console.error('响应数据:', error.response.data);
      
      if (error.response.status === 401) {
        console.warn('收到401未授权响应，即将清除token并重定向到登录页');
        localStorage.removeItem('auth_token');
        window.location.href = '/ui/login';
      }
    } else if (error.request) {
      console.error('没有收到响应:', error.request);
    } else {
      console.error('请求设置错误:', error.message);
    }
    
    return Promise.reject(error);
  }
);

// 工具函数
export const utils = {
  saveAuth: (token: string, expiryDays = 365) => {
    try {
      // 直接保存token，不需要用户名和密码
      localStorage.removeItem('auth_token'); // 先清除
      localStorage.setItem('auth_token', token);
      
      // 验证保存是否成功
      const savedToken = localStorage.getItem('auth_token');
      if (!savedToken) {
        console.error('Token保存失败! localStorage.getItem返回null');
      } else if (savedToken !== token) {
        console.error('Token保存不一致!', 
          '预期:', token.substring(0, 10) + '...', 
          '实际:', savedToken.substring(0, 10) + '...');
      }
      
      // 设置cookie有效期
      const expiryDate = new Date();
      expiryDate.setDate(expiryDate.getDate() + expiryDays);
      
      // 设置cookie，允许跨域访问
      const cookieOptions = `expires=${expiryDate.toUTCString()}; path=/; SameSite=Lax`;
      document.cookie = `auth_token=${token}; ${cookieOptions}`;
      
      // 同时设置会话存储作为备份
      sessionStorage.setItem('auth_token', token);
    } catch (error) {
      console.error('保存token时出错:', error);
      // 尝试使用备选存储方式
      try {
        sessionStorage.setItem('auth_token', token);
      } catch (e) {
        console.error('所有存储方式都失败');
      }
    }
  },
  
  clearAuth: () => {
    localStorage.removeItem('auth_token');
    sessionStorage.removeItem('auth_token');
    document.cookie = 'auth_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; SameSite=Lax';
  },
  
  isAuthenticated: () => {
    // 首先检查localStorage
    const localToken = localStorage.getItem('auth_token');
    if (localToken) {
      return true;
    }
    
    // 然后检查sessionStorage
    const sessionToken = sessionStorage.getItem('auth_token');
    if (sessionToken) {
      localStorage.setItem('auth_token', sessionToken);
      return true;
    }
    
    // 最后检查cookie
    const cookieMatch = document.cookie.match(/auth_token=([^;]+)/);
    if (cookieMatch && cookieMatch[1]) {
      localStorage.setItem('auth_token', cookieMatch[1]);
      return true;
    }
    
    return false;
  },
  
  formatDateTime: (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  },
  
  truncate: (str: string, length = 50) => {
    if (!str) return '';
    return str.length > length ? str.substring(0, length) + '...' : str;
  },
  
  formatFileSize: (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }
};

// API方法
export const apiService = {
  // 认证
  login: async (username: string, password: string) => {
    try {
      // 使用增强的API实例而不是直接使用axios
      const response = await API.post('/login', {
        username,
        password
      });
      
      // 检查登录是否成功
      if (response.data && response.data.status === 'success' && response.data.token) {
        // 保存返回的token
        utils.saveAuth(response.data.token);
        return true;
      }
      
      return false;
    } catch (error) {
      console.error('API登录出错:', error);
      if (axios.isAxiosError(error) && error.response) {
        console.error('登录响应状态:', error.response.status);
        console.error('登录响应数据:', error.response.data);
      }
      utils.clearAuth();
      return false;
    }
  },
  
  // 获取请求列表
  getRequests: async (): Promise<Request[]> => {
    try {
      const response = await API.get('/requests');
      // 处理 StandardResponse 格式
      if (response.data && response.data.data) {
        // 如果data.requests存在且是数组，返回请求列表
        if (response.data.data.requests && Array.isArray(response.data.data.requests)) {
          return response.data.data.requests;
        }
        // 向后兼容: 如果data本身是数组，直接返回
        if (Array.isArray(response.data.data)) {
        return response.data.data;
        }
      }
      // 兼容处理，如果response.data本身是数组，则直接返回
      if (Array.isArray(response.data)) {
        return response.data;
      }
      return [];
    } catch (error) {
      console.error('获取请求列表失败:', error);
      return [];
    }
  },

  // 获取请求列表（含分页元信息 total/page/size，用于统计面板）
  getRequestsWithTotal: async (): Promise<{ requests: Request[]; total: number; page: number; size: number }> => {
    const fallback = { requests: [], total: 0, page: 1, size: 20 };
    try {
      const response = await API.get('/requests');
      const data = response.data && response.data.data;
      if (data && Array.isArray(data.requests)) {
        return {
          requests: data.requests,
          total: typeof data.total === 'number' ? data.total : data.requests.length,
          page: typeof data.page === 'number' ? data.page : 1,
          size: typeof data.size === 'number' ? data.size : data.requests.length,
        };
      }
      if (Array.isArray(data)) {
        return { ...fallback, requests: data, total: data.length };
      }
      return fallback;
    } catch (error) {
      console.error('获取请求列表(含统计)失败:', error);
      return fallback;
    }
  },
  
  // 获取请求详情
  getRequestById: async (id: string): Promise<Request | null> => {
    try {
      const response = await API.get(`/requests/${id}`);
      // 处理 StandardResponse 格式
      if (response.data && response.data.data) {
        return response.data.data;
      }
      // 兼容处理，如果response.data本身就是请求对象
      if (response.data && response.data.id) {
        return response.data;
      }
      return null;
    } catch (error) {
      console.error(`获取请求详情失败 ID=${id}:`, error);
      return null;
    }
  },
  
  // 获取前一条请求
  getPreviousRequestId: async (currentId: string): Promise<string | null> => {
    try {
      const response = await API.get(`/requests/navigation/prev/${currentId}`);
      // 处理 StandardResponse 格式
      if (response.data && response.data.data && response.data.data.id) {
        return response.data.data.id;
      }
      
      // 兼容处理：如果API不支持导航功能，则获取所有请求并在前端筛选
      const requests = await apiService.getRequests();
      const currentIndex = requests.findIndex(r => r.id === currentId);
      
      if (currentIndex > 0) {
        return requests[currentIndex - 1].id;
      }
      
      return null;
    } catch (error) {
      console.error(`获取前一条请求失败 ID=${currentId}:`, error);
      return null;
    }
  },
  
  // 获取后一条请求
  getNextRequestId: async (currentId: string): Promise<string | null> => {
    try {
      const response = await API.get(`/requests/navigation/next/${currentId}`);
      // 处理 StandardResponse 格式
      if (response.data && response.data.data && response.data.data.id) {
        return response.data.data.id;
      }
      
      // 兼容处理：如果API不支持导航功能，则获取所有请求并在前端筛选
      const requests = await apiService.getRequests();
      const currentIndex = requests.findIndex(r => r.id === currentId);
      
      if (currentIndex !== -1 && currentIndex < requests.length - 1) {
        return requests[currentIndex + 1].id;
      }
      
      return null;
    } catch (error) {
      console.error(`获取后一条请求失败 ID=${currentId}:`, error);
      return null;
    }
  },
  
  // 删除请求
  deleteRequest: async (id: string): Promise<boolean> => {
    try {
      await API.delete(`/requests/${id}`);
      return true;
    } catch (error) {
      console.error(`删除请求失败 ID=${id}:`, error);
      return false;
    }
  },
  
  // 获取存储统计
  getStorageStats: async (): Promise<StorageStats | null> => {
    try {
      const response = await API.get('/storage-stats');
      // 处理 StandardResponse 格式
      if (response.data && response.data.data) {
        return response.data.data;
      }
      // 兼容处理
      if (response.data && typeof response.data === 'object') {
        return response.data;
      }
      return null;
    } catch (error) {
      console.error('获取存储统计失败:', error);
      return null;
    }
  },
  
  // 清空所有请求
  clearAllRequests: async (): Promise<boolean> => {
    try {
      await API.delete('/requests');
      return true;
    } catch (error) {
      console.error('清空请求失败:', error);
      return false;
    }
  },
  
  // 导出请求数据
  exportRequests: async (): Promise<void> => {
    try {
      const response = await API.get('/export', {
        responseType: 'blob'
      });
      
      // 创建下载链接
      const url = window.URL.createObjectURL(new Blob([response.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', `requests-export-${Date.now()}.jsonl`);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (error) {
      console.error('导出请求失败:', error);
    }
  },
  
  // 获取代理配置
  getProxyConfig: async (): Promise<ProxyConfig | null> => {
    try {
      const response = await API.get('/proxy-config');
      // 处理 StandardResponse 格式
      if (response.data && response.data.data) {
        return response.data.data;
      }
      // 兼容处理
      if (response.data && typeof response.data === 'object') {
        return response.data;
      }
      return null;
    } catch (error) {
      console.error('获取代理配置失败:', error);
      return null;
    }
  },
  
  // 保存代理配置
  saveProxyConfig: async (config: ProxyConfig): Promise<boolean> => {
    try {
      await API.post('/proxy-config', config);
      return true;
    } catch (error) {
      console.error('保存代理配置失败:', error);
      return false;
    }
  },

  getRequest: async (id: string): Promise<any> => {
    try {
      const response = await API.get(`/requests/${id}`);
      return response.data;
    } catch (error) {
      console.error(`获取请求失败 ID=${id}:`, error);
      return null;
    }
  },

  getRequestNavigationIds: async (id: string): Promise<{prevId: string | null, nextId: string | null}> => {
    try {
      // 尝试调用单独的导航接口
      // 由于后端没有实现 /requests/${id}/navigation 接口，我们使用单独的上一个和下一个请求API
      const [prevId, nextId] = await Promise.all([
        apiService.getPreviousRequestId(id),
        apiService.getNextRequestId(id)
      ]);
      
      return { prevId, nextId };
    } catch (error) {
      console.error(`获取请求导航失败 ID=${id}:`, error);
      return {prevId: null, nextId: null};
    }
  }
}; 