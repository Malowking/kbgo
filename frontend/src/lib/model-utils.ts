/**
 * 模型处理工具函数
 * 统一模型过滤、排序等逻辑
 */

import type { Model } from '@/types';

/**
 * 过滤和排序模型
 *
 * @param models - 原始模型列表
 * @param types - 需要的模型类型
 * @param onlyEnabled - 是否只返回启用的模型
 * @returns 过滤并排序后的模型列表
 */
export function filterAndSortModels(
  models: Model[],
  types: Array<'llm' | 'embedding' | 'rerank' | 'reranker' | 'multimodal'>,
  onlyEnabled = true
): Model[] {
  return models
    .filter(m => {
      // 类型过滤
      if (!types.includes(m.type)) return false;
      // 启用状态过滤
      if (onlyEnabled && m.enabled === false) return false;
      return true;
    })
    .map(m => ({
      ...m,
      // 统一使用 id 字段
      id: m.id || m.model_id,
    }))
    .sort((a, b) => a.name.localeCompare(b.name)); // 按名称排序
}

/**
 * 获取 LLM 模型列表（包括多模态）
 */
export function getLLMModels(models: Model[], onlyEnabled = true): Model[] {
  return filterAndSortModels(models, ['llm', 'multimodal'], onlyEnabled);
}

/**
 * 获取 Rerank 模型列表（支持 rerank 和 reranker 两种类型）
 */
export function getRerankModels(models: Model[], onlyEnabled = true): Model[] {
  return filterAndSortModels(models, ['rerank', 'reranker'], onlyEnabled);
}

/**
 * 获取 Embedding 模型列表
 */
export function getEmbeddingModels(models: Model[], onlyEnabled = true): Model[] {
  return filterAndSortModels(models, ['embedding'], onlyEnabled);
}

/**
 * 根据 ID 查找模型
 */
export function findModelById(models: Model[], modelId: string): Model | undefined {
  return models.find(m => m.id === modelId || m.model_id === modelId);
}

/**
 * 获取模型显示名称
 */
export function getModelDisplayName(models: Model[], modelId: string, fallback = '未选择模型'): string {
  const model = findModelById(models, modelId);
  return model ? model.name : fallback;
}

/**
 * 检查模型是否可用
 */
export function isModelAvailable(models: Model[], modelId: string): boolean {
  const model = findModelById(models, modelId);
  return !!model && model.enabled !== false;
}

/**
 * 按类型分组模型
 */
export function groupModelsByType(models: Model[]): Record<string, Model[]> {
  return models.reduce((groups, model) => {
    const type = model.type;
    if (!groups[type]) {
      groups[type] = [];
    }
    groups[type].push(model);
    return groups;
  }, {} as Record<string, Model[]>);
}