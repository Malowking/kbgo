import { apiClient } from './api';
import { handleSSEStream, handleSSEStreamWithFormData, ToolCallInfo, LLMIterationInfo } from '@/lib/sse-client';
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
  AgentPreset,
  AgentPresetItem,
  CreateAgentPresetRequest,
  UpdateAgentPresetRequest,
  AgentChatRequest,
  AgentChatResponse,
} from '@/types';

// 知识库 API
export const knowledgeBaseApi = {
  // 获取知识库列表
  list: (params?: { name?: string; status?: 1 | 2; category?: string }) =>
    apiClient.get<{ list: KnowledgeBase[] }>('/api/v1/kb', { params }),

  // 获取单个知识库
  get: (id: string) =>
    apiClient.get<KnowledgeBase>(`/api/v1/kb/${id}`),

  // 创建知识库
  create: (data: CreateKBRequest) =>
    apiClient.post<{ id: string }>('/api/v1/kb', data),

  // 更新知识库
  update: (id: string, data: Partial<CreateKBRequest & { status: 1 | 2 }>) =>
    apiClient.put<void>(`/api/v1/kb/${id}`, data),

  // 删除知识库
  delete: (id: string) =>
    apiClient.delete<void>(`/api/v1/kb/${id}`),

  // 更新知识库状态
  updateStatus: (id: string, status: 1 | 2) =>
    apiClient.patch<void>(`/api/v1/kb/${id}/status`, { status }),
};

// 文档 API
export const documentApi = {
  // 上传文件
  upload: (formData: FormData) =>
    apiClient.upload<{ document_id: string; status: string; message: string }>('/api/v1/upload', formData),

  // 索引文档
  index: (data: {
    document_ids: string[];
    embedding_model_id: string;
    chunk_size?: number;
    overlap_size?: number;
    separator?: string;
  }) =>
    apiClient.post<{ message: string }>('/api/v1/index', data),

  // 获取文档列表
  list: (params: { knowledge_id: string; page?: number; page_size?: number }) =>
    apiClient.get<{ data: Document[]; total?: number; page?: number; size?: number }>('/api/v1/documents', { params }),

  // 删除文档 - 后端只支持单个文档删除，参数名是 document_id (query/form parameter)
  delete: (documentIds: string[]) => {
    // 依次删除每个文档
    return Promise.all(
      documentIds.map(id =>
        apiClient.delete<void>(`/api/v1/documents?document_id=${encodeURIComponent(id)}`)
      )
    );
  },

  // 重新索引 - 后端只支持单个文档，参数名是 document_id
  reindex: (documentIds: string[]) => {
    return Promise.all(
      documentIds.map(id =>
        apiClient.post<void>(`/api/v1/documents/reindex?document_id=${encodeURIComponent(id)}`)
      )
    );
  },

  // 获取分块列表
  getChunks: (params: { knowledge_doc_id: string; page?: number; size?: number }) =>
    apiClient.get<{ data: Chunk[]; total?: number; page?: number; size?: number }>('/api/v1/chunks', { params }),
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
    apiClient.get<ConversationDetailRes>(`/api/v1/conversations/${convId}`),

  // 删除会话
  delete: (convId: string) =>
    apiClient.delete<{ message: string }>(`/api/v1/conversations/${convId}`),

  // 更新会话
  update: (convId: string, data: { title?: string; status?: string; tags?: string[]; metadata?: Record<string, any> }) =>
    apiClient.put<{ message: string }>(`/api/v1/conversations/${convId}`, data),

  // 批量删除会话
  batchDelete: (convIds: string[]) =>
    apiClient.post<{ deleted_count: number; failed_convs?: string[]; message: string }>('/api/v1/conversations/batch/delete', { conv_ids: convIds }),
};

// 聊天 API
export const chatApi = {
  // 发送消息
  send: (data: ChatRequest) =>
    apiClient.post<ChatResponse>('/api/v1/chat', data),

  // 流式聊天（使用 SSE 客户端）
  sendStream: async (
    data: ChatRequest & { files?: File[] },
    onMessage: (chunk: string, reasoningChunk?: string, references?: any[]) => void,
    onError?: (error: Error) => void
  ) => {
    const hasFiles = data.files && data.files.length > 0;

    // 准备回调函数
    let lastReferences: any[] | undefined;

    const handleChunk = (content: string) => {
      onMessage(content, '', lastReferences);
    };

    const handleReasoning = (reasoning: string) => {
      onMessage('', reasoning, lastReferences);
    };

    const handleReferences = (references: any[]) => {
      lastReferences = references;
      onMessage('', '', references);
    };

    if (hasFiles) {
      // 使用 FormData 上传文件
      const formData = new FormData();
      formData.append('conv_id', data.conv_id || '');
      formData.append('question', data.question);
      formData.append('model_id', data.model_id);
      formData.append('stream', 'true');

      if (data.embedding_model_id) formData.append('embedding_model_id', data.embedding_model_id);
      if (data.rerank_model_id) formData.append('rerank_model_id', data.rerank_model_id);
      if (data.knowledge_id) formData.append('knowledge_id', data.knowledge_id);
      if (data.enable_retriever !== undefined) formData.append('enable_retriever', data.enable_retriever.toString());
      if (data.top_k !== undefined) formData.append('top_k', data.top_k.toString());
      if (data.score !== undefined) formData.append('score', data.score.toString());
      if (data.retrieve_mode) formData.append('retrieve_mode', data.retrieve_mode);
      if (data.rerank_weight !== undefined) formData.append('rerank_weight', data.rerank_weight.toString());
      if (data.tools) formData.append('tools', JSON.stringify(data.tools));

      // 添加文件
      data.files!.forEach(file => {
        formData.append('files', file);
      });

      await handleSSEStreamWithFormData('/api/v1/chat', formData, {
        onChunk: handleChunk,
        onReasoning: handleReasoning,
        onReferences: handleReferences,
        onError,
      });
    } else {
      // 使用 JSON 格式
      await handleSSEStream('/api/v1/chat', { ...data, stream: true }, {
        onChunk: handleChunk,
        onReasoning: handleReasoning,
        onReferences: handleReferences,
        onError,
      });
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
    apiClient.get<{ model: Model }>(`/api/v1/model/${modelId}`),

  // 创建模型
  create: (data: CreateModelRequest) =>
    apiClient.post<{ success: boolean; message: string; model_id: string }>('/api/v1/model/register', data),

  // 更新模型
  update: (modelId: string, data: Omit<UpdateModelRequest, 'model_id'>) =>
    apiClient.put<{ success: boolean; message: string }>(`/api/v1/model/${modelId}`, data),

  // 删除模型
  delete: (modelId: string) =>
    apiClient.delete<{ success: boolean; message: string }>(`/api/v1/model/${modelId}`),

  // 重新加载模型配置
  reload: () =>
    apiClient.post<void>('/api/v1/model/reload'),

  // 获取重写模型
  getRewriteModel: () =>
    apiClient.get<{ rewrite_model: Model | null; configured: boolean }>('/api/v1/model/rewrite'),

  // 设置重写模型
  setRewriteModel: (modelId: string) =>
    apiClient.post<{ success: boolean; message: string }>('/api/v1/model/rewrite', { model_id: modelId }),
};

// MCP API
export const mcpApi = {
  // 获取MCP服务列表
  list: (params?: { page?: number; page_size?: number; status?: 1 | 0 }) =>
    apiClient.get<{ list: MCPRegistry[]; total: number; page: number; page_size: number }>('/api/v1/mcp/registry', { params }),

  // 获取单个MCP服务
  get: (id: string) =>
    apiClient.get<MCPRegistry>(`/api/v1/mcp/registry/${id}`),

  // 创建MCP服务
  create: (data: CreateMCPRequest) =>
    apiClient.post<{ id: string }>('/api/v1/mcp/registry', data),

  // 更新MCP服务
  update: (id: string, data: UpdateMCPRequest) =>
    apiClient.put<void>(`/api/v1/mcp/registry/${id}`, data),

  // 删除MCP服务
  delete: (id: string) =>
    apiClient.delete<void>(`/api/v1/mcp/registry/${id}`),

  // 更新MCP服务状态
  updateStatus: (id: string, status: 1 | 0) =>
    apiClient.patch<void>(`/api/v1/mcp/registry/${id}/status`, { status }),

  // 测试MCP服务连接
  test: (id: string) =>
    apiClient.post<{ success: boolean; message: string }>(`/api/v1/mcp/registry/${id}/test`, {}),

  // 获取MCP服务工具列表
  listTools: (id: string) =>
    apiClient.get<{ tools: MCPTool[] }>(`/api/v1/mcp/registry/${id}/tools`),

  // 获取MCP服务统计信息
  stats: (id: string) =>
    apiClient.get<MCPStats>(`/api/v1/mcp/registry/${id}/stats`),

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
    apiClient.get<{ list: MCPCallLog[] }>(`/api/v1/mcp/logs/conversation/${conversationId}`),
};

// Agent API
export const agentApi = {
  // 创建Agent预设
  create: (data: CreateAgentPresetRequest) =>
    apiClient.post<{ preset_id: string }>('/api/v1/agent/preset', data),

  // 更新Agent预设
  update: (presetId: string, data: Omit<UpdateAgentPresetRequest, 'preset_id'>) =>
    apiClient.put<{ success: boolean }>(`/api/v1/agent/preset/${presetId}`, data),

  // 获取Agent预设详情
  get: (presetId: string) =>
    apiClient.get<AgentPreset>(`/api/v1/agent/preset/${presetId}`),

  // 获取Agent预设列表
  list: (params?: { user_id?: string; is_public?: boolean; page?: number; page_size?: number }) =>
    apiClient.get<{ list: AgentPresetItem[]; total: number; page: number }>('/api/v1/agent/presets', { params }),

  // 删除Agent预设
  delete: (presetId: string, userId: string) =>
    apiClient.delete<{ success: boolean }>(`/api/v1/agent/preset/${presetId}`, { data: { user_id: userId } }),

  // Agent对话
  chat: (data: AgentChatRequest) =>
    apiClient.post<AgentChatResponse>('/api/v1/agent/chat', data),

  // Agent流式对话（使用 SSE 客户端）
  chatStream: async (
    data: AgentChatRequest & { files?: File[] },
    onMessage: (chunk: string, reasoningChunk?: string, references?: Document[]) => void,
    onError?: (error: Error) => void,
    // 新增：工具调用相关回调
    callbacks?: {
      onToolCallStart?: (toolCall: ToolCallInfo) => void;
      onToolCallEnd?: (toolCall: ToolCallInfo) => void;
      onLLMIteration?: (iteration: LLMIterationInfo) => void;
      onThinking?: (thinking: string) => void;
    }
  ) => {
    const hasFiles = data.files && data.files.length > 0;

    // 准备回调函数
    let lastReferences: any[] | undefined;

    const handleChunk = (content: string) => {
      onMessage(content, '', lastReferences);
    };

    const handleReasoning = (reasoning: string) => {
      onMessage('', reasoning, lastReferences);
    };

    const handleReferences = (references: any[]) => {
      lastReferences = references;
      onMessage('', '', references);
    };

    if (hasFiles) {
      // 使用 FormData 上传文件
      const formData = new FormData();
      formData.append('preset_id', data.preset_id);
      formData.append('user_id', data.user_id);
      formData.append('question', data.question);
      formData.append('stream', 'true');

      if (data.conv_id) formData.append('conv_id', data.conv_id);

      // 添加文件
      data.files!.forEach(file => {
        formData.append('files', file);
      });

      await handleSSEStreamWithFormData('/api/v1/agent/chat', formData, {
        onChunk: handleChunk,
        onReasoning: handleReasoning,
        onReferences: handleReferences,
        onToolCallStart: callbacks?.onToolCallStart,
        onToolCallEnd: callbacks?.onToolCallEnd,
        onLLMIteration: callbacks?.onLLMIteration,
        onThinking: callbacks?.onThinking,
        onError,
      });
    } else {
      // 使用 JSON 格式
      await handleSSEStream('/api/v1/agent/chat', { ...data, stream: true }, {
        onChunk: handleChunk,
        onReasoning: handleReasoning,
        onReferences: handleReferences,
        onToolCallStart: callbacks?.onToolCallStart,
        onToolCallEnd: callbacks?.onToolCallEnd,
        onLLMIteration: callbacks?.onLLMIteration,
        onThinking: callbacks?.onThinking,
        onError,
      });
    }
  },
};

// NL2SQL API
export const nl2sqlApi = {
  // 获取数据源列表
  listDatasources: () =>
    apiClient.get<{ list: any[]; total: number }>('/api/v1/nl2sql/datasources'),

  // 创建数据源
  createDatasource: (data: {
    name: string;
    type: string;
    db_type?: string;
    config: Record<string, any>;
    created_by?: string;
    embedding_model_id: string;
  }) =>
    apiClient.post<{ id: string }>('/api/v1/nl2sql/datasources', data),

  // 上传CSV/Excel文件
  uploadFile: (file: File, name: string, createdBy: string, displayName?: string) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('name', name);
    formData.append('created_by', createdBy);
    if (displayName) {
      formData.append('display_name', displayName);
    }
    return apiClient.post<{
      datasource_id: string;
      file_path: string;
      status: string;
      message: string;
    }>('/api/v1/nl2sql/upload-file', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    });
  },

  // 添加表到现有数据源
  addTable: (datasourceId: string, file: File, displayName?: string) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('datasource_id', datasourceId);
    if (displayName) {
      formData.append('display_name', displayName);
    }
    return apiClient.post<{
      datasource_id: string;
      table_name: string;
      row_count: number;
      status: string;
      message: string;
    }>(`/api/v1/nl2sql/datasources/${datasourceId}/tables`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    });
  },

  // 删除数据源
  deleteDatasource: (id: string) =>
    apiClient.delete<{ success: boolean }>(`/api/v1/nl2sql/datasources/${id}`),

  // 删除表
  deleteTable: (datasourceId: string, tableId: string) =>
    apiClient.delete<{ success: boolean; message: string }>(`/api/v1/nl2sql/datasources/${datasourceId}/tables/${tableId}`),

  // 解析数据源Schema
  parseSchema: (datasourceId: string, data: { llm_model_id: string }) =>
    apiClient.post<{ task_id: string }>(`/api/v1/nl2sql/datasources/${datasourceId}/parse`, data),

  // 获取数据源Schema信息
  getSchema: (datasourceId: string) =>
    apiClient.get<any>(`/api/v1/nl2sql/datasources/${datasourceId}/schema`),

  // 执行NL2SQL查询
  query: (data: {
    datasource_id: string;
    question: string;
    session_id?: string;
    llm_model_id: string;
  }) =>
    apiClient.post<{
      query_log_id: string;
      sql: string;
      result?: {
        columns: string[];
        data: any[];
        row_count: number;
      };
      explanation?: string;
      error?: string;
    }>('/api/v1/nl2sql/query', data),
};