import { RequestRecord } from '../types/RequestRecord';
import { apiService } from './api';

export class RequestService {
  /**
   * 获取所有请求记录
   */
  static async getRequests(): Promise<RequestRecord[]> {
    try {
      const result = await apiService.getRequests();
      return result as any;
    } catch (error) {
      console.error('获取请求记录时出错:', error);
      throw error;
    }
  }

  /**
   * 获取请求记录（含分页元信息，用于统计面板）
   */
  static async getRequestsWithTotal(): Promise<{ requests: RequestRecord[]; total: number }> {
    try {
      const result = await apiService.getRequestsWithTotal();
      return { requests: result.requests as any, total: result.total };
    } catch (error) {
      console.error('获取请求记录(含统计)时出错:', error);
      throw error;
    }
  }

  /**
   * 获取单个请求记录详情
   */
  static async getRequestById(id: string): Promise<RequestRecord> {
    try {
      const result = await apiService.getRequestById(id);
      return result as any;
    } catch (error) {
      console.error('获取请求记录详情时出错:', error);
      throw error;
    }
  }

  /**
   * 删除所有请求记录
   */
  static async deleteAllRequests(): Promise<void> {
    try {
      await apiService.clearAllRequests();
    } catch (error) {
      console.error('删除所有请求记录时出错:', error);
      throw error;
    }
  }
} 