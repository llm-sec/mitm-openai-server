import { useState, useEffect } from 'react';
import { apiService } from '../../../services/api';

export const useRequestDetail = (id: string | undefined) => {
  const [request, setRequest] = useState<any>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [prevRequestId, setPrevRequestId] = useState<string | null>(null);
  const [nextRequestId, setNextRequestId] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      if (!id) return;
      
      setLoading(true);
      setError(null);
      
      try {
        // 获取当前请求详情
        const requestData: any = await apiService.getRequestById(id);
        
        // 处理数据字段映射
        if (requestData) {
          // 尝试从路径或URL中提取HOST
          let extractedHost = '';
          const path = requestData.path || requestData.url || '';
          
          // 尝试从路径/URL提取主机名
          if (path) {
            try {
              // 处理完整URL的情况
              if (path.startsWith('http')) {
                const url = new URL(path);
                extractedHost = url.host;
              } 
              // 处理不带协议的URL
              else if (path.includes('//')) {
                extractedHost = path.split('//')[1].split('/')[0];
              }
              // 如果路径中包含主机名信息
              else if (path.startsWith('/') && path.split('/').length > 1) {
                const possibleHost = path.split('/')[1];
                if (possibleHost.includes('.') || possibleHost.includes(':')) {
                  extractedHost = possibleHost;
                }
              }
            } catch (e) {
              console.error('从URL提取主机名失败:', e);
            }
          }
          
          // 适配不同的字段名称
          const adaptedRequest = {
            ...requestData,
            // 请求头可能使用 headers 或 requestHeaders
            requestHeaders: requestData.requestHeaders || requestData.headers || {},
            // 查询参数可能使用 query 或 queryParams
            queryParams: requestData.queryParams || requestData.query || {},
            // 请求体可能使用 body 或 requestBody
            requestBody: requestData.requestBody || requestData.body || null,
            // 响应头可能在 response.headers 或 responseHeaders
            responseHeaders: requestData.responseHeaders || 
                           (requestData.response ? requestData.response.headers : {}) || {},
            // 响应体可能在 response.body 或 responseBody
            responseBody: requestData.responseBody || 
                         (requestData.response ? requestData.response.body : null) || null,
            // 状态码可能在多个不同字段中（兼容 status_code 与 statusCode 两种命名）
            statusCode: requestData.statusCode ||
                       (requestData.response ? requestData.response.statusCode : null) ||
                       (requestData.response ? requestData.response.status_code : null) ||
                       requestData.responseCode ||
                       requestData.status_code ||
                       (requestData.responseHeaders && requestData.responseHeaders['status-code']) ||
                       (requestData.responseBody && requestData.responseBody.status) ||
                       200,
            // 延迟可能在 latency、latency_ms 或 response.latency 中
            latency: requestData.latency ||
                    (requestData.response ? requestData.response.latency : null) ||
                    (requestData.response ? requestData.response.latency_ms : null) ||
                    (requestData.metadata ? requestData.metadata.latency_ms : null) ||
                    requestData.responseTime || null,
            // Host可能在不同的字段中
            host: requestData.host || 
                 (requestData.headers && (requestData.headers.host || requestData.headers.Host)) ||
                 (requestData.requestHeaders && (requestData.requestHeaders.host || requestData.requestHeaders.Host)) ||
                 extractedHost ||
                 (requestData.method === 'POST' && path.includes('/v1/') ? 'api.openai.com' : '') || 
                 'localhost',
          };
          
          setRequest(adaptedRequest);
        }
        
        // 获取导航ID
        try {
          const { prevId, nextId } = await apiService.getRequestNavigationIds(id);
          setPrevRequestId(prevId);
          setNextRequestId(nextId);
        } catch (navErr) {
          console.error('获取导航ID失败:', navErr);
        }
      } catch (err) {
        console.error('Failed to fetch request data:', err);
        setError('无法加载请求详情');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [id]);

  return {
    request,
    loading,
    error,
    prevRequestId,
    nextRequestId
  };
};

export default useRequestDetail; 