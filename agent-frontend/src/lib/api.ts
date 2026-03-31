// API configuration
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8888';
const USER_ID = import.meta.env.VITE_USER_ID || '550e8400-e29b-41d4-a716-446655440000';

// API types
export interface APIError {
  error_code: string;
  message: string;
  timestamp: string;
}

export interface APIResponse<T = any> {
  data?: T;
  error?: APIError;
}

export interface SessionSummary {
  conversation_id: string;
  user_id: string;
  created_at: string;
  updated_at: string;
  message_count: number;
  first_message?: string;
}

export interface ListSessionsResponse {
  items: SessionSummary[];
  total: number;
  page: number;
  page_size: number;
}

export interface Message {
  conversation_id: string;
  message_index: number;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  status: string;
}

export interface ChatRequest {
  message: string;
}

export interface ChatResponse {
  response: string;
}

// API client
class APIClient {
  private baseURL: string;
  private userID: string;

  constructor(baseURL: string, userID: string) {
    this.baseURL = baseURL;
    this.userID = userID;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseURL}${endpoint}`;
    const headers = {
      'Content-Type': 'application/json',
      'X-User-ID': this.userID,
      ...options.headers,
    };

    const response = await fetch(url, {
      ...options,
      headers,
    });

    const data: APIResponse<T> = await response.json();

    if (data.error) {
      throw new Error(data.error.message);
    }

    // Handle null or undefined data
    if (data.data === null || data.data === undefined) {
      return (Array.isArray(data.data) ? [] : {}) as T;
    }

    return data.data as T;
  }

  // Session management (Projects)
  async createSession(): Promise<{ conversation_id: string }> {
    return this.request('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify({}),
    });
  }

  async listSessions(limit = 20, offset = 0): Promise<ListSessionsResponse> {
    const result = await this.request<ListSessionsResponse>(`/api/v1/projects?limit=${limit}&offset=${offset}`);
    // Ensure items is always an array
    return {
      items: Array.isArray(result?.items) ? result.items : [],
      total: result?.total || 0,
      page: result?.page || 1,
      page_size: result?.page_size || limit,
    };
  }

  async getSession(conversationId: string): Promise<SessionSummary> {
    return this.request(`/api/v1/projects/${conversationId}`);
  }

  async deleteSession(conversationId: string): Promise<{ message: string }> {
    return this.request(`/api/v1/projects/${conversationId}`, {
      method: 'DELETE',
    });
  }

  // Chat
  async sendMessage(
    conversationId: string,
    message: string,
    onChunk?: (chunk: string) => void
  ): Promise<void> {
    const url = `${this.baseURL}/api/v1/projects/${conversationId}/chat`;
    const headers = {
      'Content-Type': 'application/json',
      'X-User-ID': this.userID,
    };

    console.log(`[${new Date().toISOString()}] 发送消息到后端:`, message);

    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify({ message }),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    console.log(`[${new Date().toISOString()}] 开始接收 SSE 流`);

    // Handle SSE stream
    const reader = response.body?.getReader();
    const decoder = new TextDecoder();

    if (!reader) {
      throw new Error('Response body is not readable');
    }

    let buffer = '';
    let chunkIndex = 0;
    let currentEventId = '';
    let currentEventType = '';
    let currentData: string[] = [];

    // 处理完整事件的函数
    const processEvent = () => {
      if (currentData.length === 0) return;

      const data = currentData.join('\n');
      console.log(`[${new Date().toISOString()}] 处理事件 - type: "${currentEventType}", id: "${currentEventId}", data: "${data.substring(0, 50)}..."`);

      if (currentEventType === 'done') {
        console.log(`[${new Date().toISOString()}] 收到完成事件`);
        return 'done';
      } else if (currentEventType === 'error') {
        throw new Error(data);
      } else if (currentEventType === 'message' || currentEventType === '' || currentEventType === 'ping') {
        // message 事件、默认事件或 ping 事件
        if (currentEventType !== 'ping' && data && onChunk) {
          chunkIndex++;
          const processTime = new Date().toISOString();
          console.log(`[${processTime}] 前端处理块 #${chunkIndex}:`, data);
          onChunk(data);
        } else if (currentEventType === 'ping') {
          console.log(`[${new Date().toISOString()}] 跳过 ping 事件`);
        }
      }
      
      return null;
    };

    try {
      while (true) {
        const { done, value } = await reader.read();
        
        if (done) {
          // 流结束前处理最后一个事件
          console.log(`[${new Date().toISOString()}] 流结束，处理最后一个事件`);
          processEvent();
          console.log(`[${new Date().toISOString()}] 流结束，共接收 ${chunkIndex} 个块`);
          break;
        }

        const chunk = decoder.decode(value, { stream: true });
        const receiveTime = new Date().toISOString();
        console.log(`[${receiveTime}] 接收到原始数据 (${value.length} 字节):`, chunk);
        
        buffer += chunk;

        // Process complete lines
        const lines = buffer.split('\n');
        buffer = lines.pop() || ''; // Keep incomplete line in buffer

        for (const line of lines) {
          // 跳过注释行（以 : 开头，但不是字段）
          if (line.startsWith(':') && !line.startsWith('id:') && !line.startsWith('event:') && !line.startsWith('data:')) {
            console.log(`[${new Date().toISOString()}] 跳过注释行:`, line);
            continue;
          }

          const trimmedLine = line.trim();
          
          // 空行表示事件结束
          if (trimmedLine === '') {
            const result = processEvent();
            if (result === 'done') {
              return;
            }
            // 重置当前事件
            currentEventId = '';
            currentEventType = '';
            currentData = [];
            continue;
          }
          
          // 解析 SSE 字段
          const colonIndex = trimmedLine.indexOf(':');
          if (colonIndex === -1) continue;

          const field = trimmedLine.substring(0, colonIndex);
          const value = trimmedLine.substring(colonIndex + 1).trim();

          if (field === 'id') {
            currentEventId = value;
          } else if (field === 'event') {
            currentEventType = value;
          } else if (field === 'data') {
            currentData.push(value);
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  async getMessages(conversationId: string): Promise<Message[]> {
    const result = await this.request<Message[]>(`/api/v1/projects/${conversationId}/messages`);
    return Array.isArray(result) ? result : [];
  }
}

export const apiClient = new APIClient(API_BASE_URL, USER_ID);
