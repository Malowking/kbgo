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
  model_id: string;
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

// 工具消息的 metadata 结构
export interface ToolMessageMetadata {
  tool_name: string;
  tool_args?: any;
  tool_call_id: string;
}

export interface Message {
  id: number;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  reasoning_content?: string;
  references?: Document[];
  metadata?: ToolMessageMetadata | Record<string, any>;
  create_time: string;
  tokens_used?: number;
  latency_ms?: number;
  extra?: {
    tool?: Array<{
      content: string;
      tool_call_id?: string;
      tool_name?: string;
      tool_args?: any;
    }>;
    [key: string]: any;
  };
}

// 工具配置类型
export interface ToolConfig {
  type: string; // "local_tools" or "mcp"
  enabled: boolean; // 是否启用该类型的工具
  priority?: number; // 工具优先级（可选），数字越小优先级越高
  config: Record<string, any>; // 工具配置参数
}

export interface ChatRequest {
  conv_id: string;
  question: string;
  model_id: string;
  system_prompt?: string;
  embedding_model_id?: string;
  rerank_model_id?: string;
  knowledge_id?: string;
  enable_retriever?: boolean;
  top_k?: number;
  score?: number;
  retrieve_mode?: 'simple' | 'rerank' | 'rrf';
  rerank_weight?: number;

  // 新的统一工具配置
  tools?: ToolConfig[];

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
  model_id: string;
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
  retrieve_mode?: 'simple' | 'rerank' | 'rrf';
  rerank_weight?: number;
  use_mcp?: boolean;
  mcp_service_tools?: Record<string, string[]>;
  jsonformat?: boolean;
  // NL2SQL相关配置
  enable_nl2sql?: boolean;
  nl2sql_datasource_id?: string;
  nl2sql_embedding_model_id?: string; // NL2SQL Schema向量化使用的embedding模型
  // 文件导出相关配置
  enable_file_export?: boolean;
  // Claude Skills 相关配置
  enable_claude_skills?: boolean;
  claude_skill_ids?: string[]; // 选中的 Skills IDs
  // 工具优先级配置
  knowledge_retrieval_priority?: number;
  nl2sql_priority?: number;
  mcp_priority?: number;
  file_export_priority?: number;
  claude_skills_priority?: number;
}

export interface AgentPreset {
  preset_id: string;
  user_id: string;
  preset_name: string;
  description: string;
  config: AgentConfig;
  tools?: ToolConfig[]; // 工具配置数组
  is_public: boolean;
  create_time: string;
  update_time: string;
}

export interface CreateAgentPresetRequest {
  user_id: string;
  preset_name: string;
  description: string;
  config: AgentConfig;
  tools?: ToolConfig[]; // 工具配置数组
  is_public: boolean;
}

export interface UpdateAgentPresetRequest {
  preset_id: string;
  user_id: string;
  preset_name?: string;
  description?: string;
  config?: AgentConfig;
  tools?: ToolConfig[]; // 工具配置数组
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

// Claude Skills 相关类型
export interface SkillToolParameterDef {
  type: 'string' | 'number' | 'boolean' | 'array' | 'object';
  required: boolean;
  description: string;
  default?: any;
}

export interface ClaudeSkill {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  category: string;
  tags: string;
  runtime_type: 'python' | 'node' | 'shell';
  runtime_version: string;
  requirements: string[];
  tool_name: string;
  tool_description: string;
  tool_parameters: Record<string, SkillToolParameterDef>;
  script: string;
  script_hash: string;
  metadata?: Record<string, any>;
  call_count: number;
  success_count: number;
  fail_count: number;
  avg_duration: number;
  last_used_at?: string;
  status: 0 | 1; // 0-disabled, 1-enabled
  is_public: boolean;
  owner_id: string;
  create_time: string;
  update_time: string;
}

export interface SkillItem {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  category: string;
  tags: string;
  runtime_type: 'python' | 'node' | 'shell';
  requirements: string[];
  tool_name: string;
  tool_description: string;
  call_count: number;
  success_count: number;
  avg_duration: number;
  last_used_at?: string;
  status: 0 | 1;
  is_public: boolean;
  owner_id: string;
  create_time: string;
  update_time: string;
}

export interface CreateSkillRequest {
  name: string;
  description: string;
  version?: string;
  author?: string;
  category?: string;
  tags?: string;
  runtime_type: 'python' | 'node' | 'shell';
  runtime_version?: string;
  requirements?: string[];
  tool_name: string;
  tool_description: string;
  tool_parameters?: Record<string, SkillToolParameterDef>;
  script: string;
  is_public?: boolean;
  metadata?: Record<string, any>;
}

export interface UpdateSkillRequest {
  id: string;
  name?: string;
  description?: string;
  version?: string;
  author?: string;
  category?: string;
  tags?: string;
  runtime_type?: 'python' | 'node' | 'shell';
  runtime_version?: string;
  requirements?: string[];
  tool_name?: string;
  tool_description?: string;
  tool_parameters?: Record<string, SkillToolParameterDef>;
  script?: string;
  status?: 0 | 1;
  is_public?: boolean;
  metadata?: Record<string, any>;
}

export interface SkillExecuteRequest {
  id: string;
  arguments: Record<string, any>;
}

export interface SkillExecuteResponse {
  success: boolean;
  output?: string;
  error?: string;
  duration: number;
}

export interface SkillCallLogItem {
  id: string;
  skill_id: string;
  skill_name: string;
  conversation_id?: string;
  message_id?: string;
  request_payload: string;
  response_payload?: string;
  success: boolean;
  error_message?: string;
  duration: number;
  venv_hash?: string;
  venv_cache_hit: boolean;
  create_time: string;
}

export interface SkillCategoryItem {
  name: string;
  count: number;
}
