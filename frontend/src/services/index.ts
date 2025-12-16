import { apiClient } from './api';
import type {
  KnowledgeBase,
  CreateKBRequest,
  Document,
  Chunk,
  Conversation,
  ConversationDetailRes,
  ChatRequest,
  ChatResponse,
  Model,
  CreateModelRequest,
  UpdateModelRequest,
  MCPRegistry,
  CreateMCPRequest,
  UpdateMCPRequest,
  MCPCallLog,
  MCPStats,
  MCPTool,
} from '@/types';

// 知识库 API
export const knowledgeBaseApi = {
  // 获取知识库列表
  list: (params?: { name?: string; status?: 1 | 2; category?: string }) =>
    apiClient.get<{ list: KnowledgeBase[] }>('/api/v1/kb', { params }),

  // 获取单个知识库
  get: (id: string) =>
    apiClient.get<KnowledgeBase>(`/v1/kb/${id}`),

  // 创建知识库
  create: (data: CreateKBRequest) =>
    apiClient.post<{ id: string }>('/api/v1/kb', data),

  // 更新知识库
  update: (id: string, data: Partial<CreateKBRequest & { status: 1 | 2 }>) =>
    apiClient.put<void>(`/v1/kb/${id}`, data),

  // 删除知识库
  delete: (id: string) =>
    apiClient.delete<void>(`/v1/kb/${id}`),

  // 更新知识库状态
  updateStatus: (id: string, status: 1 | 2) =>
    apiClient.patch<void>(`/v1/kb/${id}/status`, { status }),
};

// 文档 API
export const documentApi = {
  // 上传文件
  upload: (formData: FormData) =>
    apiClient.upload<{ document_ids: string[] }>('/api/v1/upload', formData),

  // 索引文档
  index: (data: { document_ids: string[]; chunk_size?: number; chunk_overlap?: number }) =>
    apiClient.post<void>('/api/v1/index', data),

  // 获取文档列表
  list: (params: { kb_id: string; page?: number; page_size?: number }) =>
    apiClient.get<{ list: Document[]; total?: number }>('/api/v1/documents', { params }),

  // 删除文档
  delete: (documentIds: string[]) =>
    apiClient.delete<void>('/api/v1/documents', { data: { document_ids: documentIds } }),

  // 重新索引
  reindex: (documentIds: string[]) =>
    apiClient.post<void>('/api/v1/documents/reindex', { document_ids: documentIds }),

  // 获取分块列表
  getChunks: (params: { document_id: string; page?: number; page_size?: number }) =>
    apiClient.get<{ list: Chunk[]; total?: number }>('/api/v1/chunks', { params }),
};

// 对话 API
export const conversationApi = {
  // 获取会话列表
  list: (params?: {
    knowledge_id?: string;
    page?: number;
    page_size?: number;
    status?: string;
    sort_by?: string;
    order?: string;
  }) =>
    apiClient.get<{
      conversations: Conversation[];
      total: number;
      page: number;
      page_size: number;
    }>('/api/v1/conversations', { params }),

  // 获取会话详情
  get: (convId: string) =>
    apiClient.get<ConversationDetailRes>(`/v1/conversations/${convId}`),

  // 删除会话
  delete: (convId: string) =>
    apiClient.delete<{ message: string }>(`/v1/conversations/${convId}`),

  // 更新会话
  update: (convId: string, data: { title?: string; status?: string; tags?: string[]; metadata?: Record<string, any> }) =>
    apiClient.put<{ message: string }>(`/v1/conversations/${convId}`, data),

  // 批量删除会话
  batchDelete: (convIds: string[]) =>
    apiClient.post<{ deleted_count: number; failed_convs?: string[]; message: string }>('/api/v1/conversations/batch/delete', { conv_ids: convIds }),
};

// 聊天 API
export const chatApi = {
  // 发送消息
  send: (data: ChatRequest) =>
    apiClient.post<ChatResponse>('/api/v1/chat', data),

  // 流式聊天（需要特殊处理）
  sendStream: async (
    data: ChatRequest,
    onMessage: (chunk: string) => void,
    onError?: (error: Error) => void
  ) => {
    try {
      const response = await fetch('/api/v1/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ ...data, stream: true }),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const reader = response.body?.getReader();
      const decoder = new TextDecoder();

      if (!reader) {
        throw new Error('Response body is null');
      }

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          const chunk = decoder.decode(value, { stream: true });

          // 直接传递文本块，不进行 SSE 解析
          if (chunk) {
            onMessage(chunk);
          }
        }
      } finally {
        reader.releaseLock();
      }
    } catch (error) {
      console.error('Stream error:', error);
      if (onError) {
        onError(error as Error);
      } else {
        throw error;
      }
    }
  },
};

// 模型 API
export const modelApi = {
  // 获取模型列表
  list: (params?: { model_type?: string }) =>
    apiClient.get<{ models: Model[] }>('/api/v1/model/list', { params }),

  // 获取单个模型
  get: (modelId: string) =>
    apiClient.get<{ model: Model }>(`/v1/model/${modelId}`),

  // 创建模型
  create: (data: CreateModelRequest) =>
    apiClient.post<{ success: boolean; message: string; model_id: string }>('/api/v1/model/register', data),

  // 更新模型
  update: (modelId: string, data: Omit<UpdateModelRequest, 'model_id'>) =>
    apiClient.put<{ success: boolean; message: string }>(`/v1/model/${modelId}`, data),

  // 删除模型
  delete: (modelId: string) =>
    apiClient.delete<{ success: boolean; message: string }>(`/v1/model/${modelId}`),

  // 重新加载模型配置
  reload: () =>
    apiClient.post<void>('/api/v1/model/reload'),
};

// MCP API
export const mcpApi = {
  // 获取MCP服务列表
  list: (params?: { page?: number; page_size?: number; status?: 1 | 0 }) =>
    apiClient.get<{ list: MCPRegistry[]; total: number; page: number; page_size: number }>('/api/v1/mcp/registry', { params }),

  // 获取单个MCP服务
  get: (id: string) =>
    apiClient.get<MCPRegistry>(`/v1/mcp/registry/${id}`),

  // 创建MCP服务
  create: (data: CreateMCPRequest) =>
    apiClient.post<{ id: string }>('/api/v1/mcp/registry', data),

  // 更新MCP服务
  update: (id: string, data: UpdateMCPRequest) =>
    apiClient.put<void>(`/v1/mcp/registry/${id}`, data),

  // 删除MCP服务
  delete: (id: string) =>
    apiClient.delete<void>(`/v1/mcp/registry/${id}`),

  // 更新MCP服务状态
  updateStatus: (id: string, status: 1 | 0) =>
    apiClient.patch<void>(`/v1/mcp/registry/${id}/status`, { status }),

  // 测试MCP服务连接
  test: (id: string) =>
    apiClient.post<{ success: boolean; message: string }>(`/v1/mcp/registry/${id}/test`, {}),

  // 获取MCP服务工具列表
  listTools: (id: string) =>
    apiClient.get<{ tools: MCPTool[] }>(`/v1/mcp/registry/${id}/tools`),

  // 获取MCP服务统计信息
  stats: (id: string) =>
    apiClient.get<MCPStats>(`/v1/mcp/registry/${id}/stats`),

  // 获取MCP调用日志列表
  logs: (params?: {
    conversation_id?: string;
    mcp_registry_id?: string;
    tool_name?: string;
    status?: 1 | 0 | 2;
    page?: number;
    page_size?: number;
  }) =>
    apiClient.get<{ list: MCPCallLog[]; total: number; page: number; page_size: number }>('/api/v1/mcp/logs', { params }),

  // 根据会话ID获取MCP调用日志
  logsByConversation: (conversationId: string) =>
    apiClient.get<{ list: MCPCallLog[] }>(`/v1/mcp/logs/conversation/${conversationId}`),
};