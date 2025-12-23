// 知识库相关类型
export interface KnowledgeBase {
  id: string;
  name: string;
  description: string;
  category?: string;
  collectionName?: string;
  embeddingModelId: string; // 绑定的 embedding 模型 ID
  status: 1 | 2; // 1-enabled, 2-disabled
  createTime: string;
  updateTime: string;
}

export interface CreateKBRequest {
  name: string;
  description: string;
  category?: string;
  embedding_model_id: string; // 必填：绑定的 embedding 模型 ID
}

// 文档相关类型
export interface Document {
  id: string;
  knowledgeId: string;
  fileName: string;
  fileExtension: string;
  collectionName?: string;
  sha256?: string;
  rustfsBucket?: string;
  rustfsLocation?: string;
  localFilePath?: string;
  status: number; // 0=pending, 1=indexing, 2=active, 3=failed
  CreateTime: string;
  UpdateTime: string;

  // 兼容旧字段名（可选）
  name?: string; // alias for fileName
  file_type?: string; // alias for fileExtension
  file_size?: number;
  chunk_count?: number;
  created_at?: string; // alias for CreateTime
  updated_at?: string; // alias for UpdateTime
}

export interface Chunk {
  id: string;
  knowledgeDocId: string;
  content: string;
  ext: string; // JSON string containing metadata like chunk_index
  collectionName: string;
  status: number;
  createTime: string;
  updateTime: string;
}

// 对话相关类型
export interface Conversation {
  conv_id: string;
  title: string;
  model_name: string;
  conversation_type: string;
  status: string;
  message_count: number;
  last_message: string;
  last_message_time: string;
  create_time: string;
  update_time: string;
  tags?: string[];
  metadata?: Record<string, any>;
}

export interface Message {
  id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  reasoning_content?: string;
  references?: Document[];
  create_time: string;
  tokens_used?: number;
  latency_ms?: number;
}

export interface ChatRequest {
  conv_id: string;
  question: string;
  model_id: string;
  embedding_model_id?: string;
  rerank_model_id?: string;
  knowledge_id?: string;
  enable_retriever?: boolean;
  top_k?: number;
  score?: number;
  retrieve_mode?: 'milvus' | 'rerank' | 'rrf';
  rerank_weight?: number;
  use_mcp?: boolean;
  mcp_service_tools?: Record<string, string[]>;
  stream?: boolean;
  jsonformat?: boolean;
}

export interface ChatResponse {
  answer: string;
  reasoning_content?: string;
  references?: Document[];
  mcp_results?: MCPResult[];
}

export interface MCPResult {
  service_name: string;
  tool_name: string;
  content: string;
}

// 模型相关类型
export interface Model {
  model_id: string;
  id?: string; // 别名，方便使用
  name: string;
  type: 'llm' | 'embedding' | 'rerank' | 'multimodal' | 'reranker';
  provider: string;
  version?: string;
  api_key?: string;
  base_url?: string;
  status?: 'active' | 'inactive';
  enabled?: boolean;
  extra?: Record<string, any>;
  config?: Record<string, any>;
}

export interface CreateModelRequest {
  model_name: string;
  model_type: 'llm' | 'embedding' | 'rerank' | 'multimodal' | 'image' | 'video' | 'audio';
  provider?: string;
  base_url?: string;
  api_key?: string;
  max_completion_tokens?: number;
  dimension?: number;
  config?: Record<string, any>;
  enabled?: boolean;
}

export interface UpdateModelRequest {
  model_id: string;
  model_name?: string;
  model_type?: string;
  provider?: string;
  version?: string;
  base_url?: string;
  api_key?: string;
  enabled?: boolean;
  extra?: string;
}

// API 响应类型
export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data: T;
}

export interface PaginationParams {
  page?: number;
  page_size?: number;
}

export interface PaginatedResponse<T> {
  list: T[];
  total: number;
  page: number;
  page_size: number;
}

// 会话详情响应类型
export interface ConversationDetailRes {
  conv_id: string;
  user_id: string;
  title: string;
  model_name: string;
  conversation_type: string;
  status: string;
  message_count: number;
  messages: Message[];
  create_time: string;
  update_time: string;
  tags?: string[];
  metadata?: Record<string, any>;
}

// MCP 相关类型
export interface MCPRegistry {
  id: string;
  name: string;
  description: string;
  endpoint: string;
  api_key?: string;
  headers?: Record<string, string>;
  timeout: number;
  status: 1 | 0; // 1-enabled, 0-disabled
  tools?: MCPTool[];
  create_time: string;
  update_time: string;
}

export interface MCPTool {
  name: string;
  description: string;
  inputSchema?: {
    type: string;
    properties?: Record<string, any>;
    required?: string[];
  };
}

export interface CreateMCPRequest {
  name: string;
  description: string;
  endpoint: string;
  api_key?: string;
  headers?: Record<string, string>;
  timeout?: number;
}

export interface UpdateMCPRequest extends CreateMCPRequest {
  id: string;
}

export interface MCPCallLog {
  id: string;
  conversation_id: string;
  mcp_registry_id: string;
  mcp_service_name: string;
  tool_name: string;
  request_payload: any;
  response_payload: any;
  status: 1 | 0 | 2; // 1-success, 0-failed, 2-timeout
  error_message?: string;
  duration: number;
  create_time: string;
}

export interface MCPStats {
  total_calls: number;
  success_calls: number;
  failed_calls: number;
  avg_duration: number;
  success_rate: number;
}

// Agent 相关类型
export interface AgentConfig {
  model_id: string;
  system_prompt?: string;
  embedding_model_id?: string;
  rerank_model_id?: string;
  knowledge_id?: string;
  enable_retriever?: boolean;
  top_k?: number;
  score?: number;
  retrieve_mode?: 'milvus' | 'rerank' | 'rrf';
  rerank_weight?: number;
  use_mcp?: boolean;
  mcp_service_tools?: Record<string, string[]>;
  jsonformat?: boolean;
}

export interface AgentPreset {
  preset_id: string;
  user_id: string;
  preset_name: string;
  description: string;
  config: AgentConfig;
  is_public: boolean;
  create_time: string;
  update_time: string;
}

export interface CreateAgentPresetRequest {
  user_id: string;
  preset_name: string;
  description: string;
  config: AgentConfig;
  is_public: boolean;
}

export interface UpdateAgentPresetRequest {
  preset_id: string;
  user_id: string;
  preset_name?: string;
  description?: string;
  config?: AgentConfig;
  is_public?: boolean;
}

export interface AgentPresetItem {
  preset_id: string;
  preset_name: string;
  description: string;
  is_public: boolean;
  create_time: string;
  update_time: string;
}

export interface AgentChatRequest {
  preset_id: string;
  user_id: string;
  question: string;
  conv_id?: string;
  stream?: boolean;
}

export interface AgentDoc {
  document_id: string;
  chunk_id: string;
  content: string;
  score: number;
}

export interface AgentChatResponse {
  conv_id: string;
  answer: string;
  reasoning_content?: string;
  references?: AgentDoc[];
  mcp_results?: MCPResult[];
}