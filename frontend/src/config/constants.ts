/**
 * 应用配置常量
 * 集中管理所有硬编码的配置值
 */

/** 聊天相关配置 */
export const CHAT_CONFIG = {
  /** 默认返回文档数量 */
  DEFAULT_TOP_K: 5,
  /** 默认相似度分数阈值 */
  DEFAULT_SCORE: 0.2,
  /** 默认检索模式 */
  DEFAULT_RETRIEVE_MODE: 'rrf' as const,
} as const;

/** 文档索引配置 */
export const INDEX_CONFIG = {
  /** 默认分块大小 */
  DEFAULT_CHUNK_SIZE: 1000,
  /** 默认重叠大小 */
  DEFAULT_OVERLAP: 100,
  /** 默认分隔符 */
  DEFAULT_SEPARATOR: '\n\n',
} as const;

/** API 请求配置 */
export const API_CONFIG = {
  /** 默认超时时间（30秒） */
  TIMEOUT: 30000,
  /** 文件上传超时时间（5分钟） */
  UPLOAD_TIMEOUT: 300000,
  /** 流式请求超时时间（5分钟） */
  STREAM_TIMEOUT: 300000,
} as const;

/** 用户配置 */
export const USER = {
  /**
   * 当前用户 ID - 从 localStorage 获取，如果不存在则使用默认值
   * 在实现完整的认证系统之前，可以在浏览器控制台通过以下命令设置：
   * localStorage.setItem('user_id', 'your_user_id')
   */
  get ID(): string {
    if (typeof window !== 'undefined') {
      return localStorage.getItem('user_id') || 'user_001';
    }
    return 'user_001';
  },
} as const;

/** 检索模式选项 */
export const RETRIEVE_MODES = [
  { value: 'rrf', label: 'RRF（推荐）' },
  { value: 'rerank', label: 'Rerank' },
  { value: 'milvus', label: 'Milvus' },
] as const;

/** 模型类型 */
export const MODEL_TYPES = {
  LLM: 'llm',
  EMBEDDING: 'embedding',
  RERANKER: 'reranker',
  MULTIMODAL: 'multimodal',
} as const;