import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, Server, CheckCircle, XCircle, Plus, Edit2, Trash2, Sparkles } from 'lucide-react';
import { modelApi } from '@/services';
import type { Model } from '@/types';
import CreateModelModal from './CreateModelModal';
import { logger } from '@/lib/logger';
import { showSuccess, showError, showInfo } from '@/lib/toast';
import { useConfirm } from '@/hooks/useConfirm';

export default function Models() {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [filterType, setFilterType] = useState<string>('all');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingModel, setEditingModel] = useState<Model | null>(null);
  const [rewriteModel, setRewriteModel] = useState<Model | null>(null);
  const { confirm, ConfirmDialog } = useConfirm();

  const fetchModels = useCallback(async () => {
    try {
      setLoading(true);
      const response = await modelApi.list();
      // 添加 id 别名，方便使用，并使用 enabled 字段判断状态
      const modelsWithId = (response.models || []).map(m => ({
        ...m,
        id: m.model_id,
        status: m.enabled ? 'active' : 'inactive',
      })).sort((a, b) => a.name.localeCompare(b.name));
      setModels(modelsWithId as Model[]);
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchRewriteModel = useCallback(async () => {
    try {
      const response = await modelApi.getRewriteModel();
      setRewriteModel(response.rewrite_model);
    } catch (error) {
      logger.error('Failed to fetch rewrite model:', error);
    }
  }, []);

  useEffect(() => {
    fetchModels();
    fetchRewriteModel();
  }, [fetchModels, fetchRewriteModel]);

  const handleReload = useCallback(async () => {
    const confirmed = await confirm({
      message: '确定要重新加载模型配置吗?',
      type: 'warning'
    });
    if (!confirmed) return;

    try {
      setReloading(true);
      await modelApi.reload();
      await fetchModels();
      showSuccess('模型配置已重新加载');
    } catch (error) {
      logger.error('Failed to reload models:', error);
      showError('重新加载失败');
    } finally {
      setReloading(false);
    }
  }, [fetchModels, confirm]);

  const handleDelete = useCallback(async (id: string, name: string) => {
    const confirmed = await confirm({
      message: `确定要删除模型 "${name}" 吗？\n\n注意：如果该模型正被知识库或Agent使用，将无法删除。`,
      type: 'danger'
    });
    if (!confirmed) return;

    try {
      const response = await modelApi.delete(id);

      // 检查响应中的success字段
      if (response.success === false) {
        // 后端返回了无法删除的提示
        showInfo(response.message || '无法删除该模型');
      } else {
        // 删除成功
        showSuccess('删除成功');
      }

      fetchModels();
    } catch (error: any) {
      logger.error('Failed to delete model:', error);
      // 显示详细错误信息
      const errorMessage = error.response?.data?.message || error.message || '删除失败';
      showError(`删除失败：${errorMessage}`);
    }
  }, [fetchModels, confirm]);

  const handleEdit = useCallback((model: Model) => {
    setEditingModel(model);
    setShowCreateModal(true);
  }, []);

  const handleModalClose = useCallback(() => {
    setShowCreateModal(false);
    setEditingModel(null);
  }, []);

  const handleSetRewriteModel = useCallback(async (modelId: string, modelName: string) => {
    const confirmed = await confirm({
      message: `确定要将 "${modelName}" 设为查询重写模型吗？\n\n重写模型用于对话中的指代消解，建议选择小参数量的非思考模型以获得更快的响应速度。`,
      type: 'info'
    });
    if (!confirmed) return;

    try {
      await modelApi.setRewriteModel(modelId);
      await fetchRewriteModel();
      await fetchModels();
      showSuccess('重写模型设置成功');
    } catch (error: any) {
      logger.error('Failed to set rewrite model:', error);
      showError(`设置失败: ${error.message || '未知错误'}`);
    }
  }, [fetchModels, fetchRewriteModel, confirm]);

  const isRewriteModel = (modelId: string) => {
    return rewriteModel?.model_id === modelId;
  };

  const getModelDescription = (model: Model): string => {
    try {
      if (model.extra && typeof model.extra === 'object' && 'description' in model.extra) {
        return model.extra.description as string;
      }
      if (model.config && typeof model.config === 'object' && 'description' in model.config) {
        return model.config.description as string;
      }
    } catch (e) {
      logger.error('Failed to parse model description:', e);
    }
    return '';
  };

  const filteredModels = filterType === 'all'
    ? models
    : models.filter(m => m.type === filterType);

  const modelTypeColors = {
    llm: 'bg-blue-100 text-blue-700',
    embedding: 'bg-green-100 text-green-700',
    reranker: 'bg-purple-100 text-purple-700',
    multimodal: 'bg-orange-100 text-orange-700',
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">模型管理</h1>
          <p className="mt-2 text-gray-600">
            管理和配置 AI 模型
          </p>
        </div>
        <div className="flex space-x-3">
          <button
            onClick={handleReload}
            disabled={reloading}
            className="btn btn-secondary flex items-center"
          >
            <RefreshCw className={`w-5 h-5 mr-2 ${reloading ? 'animate-spin' : ''}`} />
            重新加载配置
          </button>
          <button
            onClick={() => setShowCreateModal(true)}
            className="btn btn-primary flex items-center"
          >
            <Plus className="w-5 h-5 mr-2" />
            添加模型
          </button>
        </div>
      </div>

      {/* Filter */}
      <div className="card">
        <div className="flex items-center space-x-2">
          <span className="text-sm font-medium text-gray-700">筛选类型:</span>
          <div className="flex space-x-2">
            {[
              { value: 'all', label: '全部' },
              { value: 'llm', label: 'LLM' },
              { value: 'embedding', label: 'Embedding' },
              { value: 'reranker', label: 'Reranker' },
              { value: 'multimodal', label: 'VLM' },
            ].map((type) => (
              <button
                key={type.value}
                onClick={() => setFilterType(type.value)}
                className={`px-3 py-1 rounded-lg text-sm font-medium transition-colors ${
                  filterType === type.value
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                }`}
              >
                {type.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
        {[
          { label: '总模型数', value: models.length, bgColor: 'bg-blue-100', iconColor: 'text-blue-600' },
          { label: 'LLM', value: models.filter(m => m.type === 'llm').length, bgColor: 'bg-blue-100', iconColor: 'text-blue-600' },
          { label: 'Embedding', value: models.filter(m => m.type === 'embedding').length, bgColor: 'bg-green-100', iconColor: 'text-green-600' },
          { label: 'Rerank', value: models.filter(m => m.type === 'reranker').length, bgColor: 'bg-purple-100', iconColor: 'text-purple-600' },
          { label: 'VLM', value: models.filter(m => m.type === 'multimodal').length, bgColor: 'bg-orange-100', iconColor: 'text-orange-600' },
        ].map((stat, idx) => (
          <div key={idx} className="card">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-600">{stat.label}</p>
                <p className="text-2xl font-bold text-gray-900 mt-1">{stat.value}</p>
              </div>
              <div className={`w-12 h-12 rounded-full ${stat.bgColor} flex items-center justify-center`}>
                <Server className={`w-6 h-6 ${stat.iconColor}`} />
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Rewrite Model Card */}
      {rewriteModel ? (
        <div className="card bg-gradient-to-r from-purple-50 to-pink-50 border-purple-200">
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <div className="flex items-center space-x-2 mb-2">
                <Sparkles className="w-5 h-5 text-purple-600" />
                <h3 className="text-lg font-semibold text-gray-900">查询重写模型</h3>
              </div>
              <p className="text-sm text-gray-600 mb-3">
                用于对话中的指代消解和查询补全，提升多轮对话的理解能力
              </p>
              <div className="flex items-center space-x-4">
                <div>
                  <span className="text-xs text-gray-500">当前模型：</span>
                  <span className="ml-2 text-sm font-medium text-gray-900">{rewriteModel.name}</span>
                </div>
                <div>
                  <span className="text-xs text-gray-500">提供商：</span>
                  <span className="ml-2 text-sm text-gray-700">{rewriteModel.provider}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="card bg-gray-50 border-gray-200 border-dashed">
          <div className="flex items-center space-x-3">
            <Sparkles className="w-5 h-5 text-gray-400" />
            <div>
              <h3 className="text-sm font-medium text-gray-700">未配置查询重写模型</h3>
              <p className="text-xs text-gray-500 mt-1">
                查询重写用于对话中的指代消解。在下方LLM模型列表中选择一个模型设为重写模型。
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Models List */}
      {loading ? (
        <div className="text-center py-12">
          <div className="inline-block w-8 h-8 border-4 border-primary-600 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-gray-600">加载中...</p>
        </div>
      ) : filteredModels.length === 0 ? (
        <div className="card text-center py-12">
          <p className="text-gray-500">
            {filterType === 'all' ? '没有配置的模型' : `没有 ${filterType} 类型的模型`}
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {filteredModels.map((model) => {
            const description = getModelDescription(model);
            const isRewrite = isRewriteModel(model.id || model.model_id);

            return (
            <div key={model.id} className={`card hover:shadow-md transition-shadow ${isRewrite ? 'ring-2 ring-purple-300 bg-purple-50/30' : ''}`}>
              <div className="flex items-start justify-between mb-4">
                <div className="flex-1">
                  <div className="flex items-center space-x-2 mb-2">
                    <h3 className="text-lg font-semibold text-gray-900">
                      {model.name}
                    </h3>
                    {model.status === 'active' ? (
                      <CheckCircle className="w-5 h-5 text-green-600" />
                    ) : (
                      <XCircle className="w-5 h-5 text-gray-400" />
                    )}
                    {isRewrite && (
                      <span className="flex items-center space-x-1 px-2 py-1 bg-purple-100 text-purple-700 rounded text-xs font-medium">
                        <Sparkles className="w-3 h-3" />
                        <span>重写模型</span>
                      </span>
                    )}
                  </div>
                  <div className="flex items-center space-x-2 mb-2">
                    <span className={`px-2 py-1 text-xs font-medium rounded ${
                      modelTypeColors[model.type as keyof typeof modelTypeColors] || 'bg-gray-100 text-gray-700'
                    }`}>
                      {model.type.toUpperCase()}
                    </span>
                    <span className="text-sm text-gray-600">
                      {model.provider}
                    </span>
                  </div>
                  {description && (
                    <p className="text-sm text-gray-600 mt-2 line-clamp-2">
                      {description}
                    </p>
                  )}
                </div>

                {/* Actions */}
                <div className="flex space-x-2">
                  {model.type === 'llm' && model.status === 'active' && !isRewrite && (
                    <button
                      onClick={() => handleSetRewriteModel(model.id || model.model_id, model.name)}
                      className="p-2 rounded hover:bg-purple-100 text-purple-600"
                      title="设为重写模型"
                    >
                      <Sparkles className="w-4 h-4" />
                    </button>
                  )}
                  <button
                    onClick={() => handleEdit(model)}
                    className="p-2 rounded hover:bg-gray-100 text-blue-600"
                    title="编辑"
                  >
                    <Edit2 className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleDelete(model.id || model.model_id, model.name)}
                    className="p-2 rounded hover:bg-gray-100 text-red-600"
                    title="删除"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between py-2 border-t border-gray-100">
                  <span className="text-gray-600">模型 ID:</span>
                  <span className="text-gray-900 font-mono text-xs">{model.id}</span>
                </div>

                {model.base_url && (
                  <div className="flex items-center justify-between py-2 border-t border-gray-100">
                    <span className="text-gray-600">Base URL:</span>
                    <span className="text-gray-900 text-xs truncate max-w-[200px]">
                      {model.base_url}
                    </span>
                  </div>
                )}

                <div className="flex items-center justify-between py-2 border-t border-gray-100">
                  <span className="text-gray-600">状态:</span>
                  <span className={`px-2 py-1 rounded text-xs font-medium ${
                    model.status === 'active'
                      ? 'bg-green-100 text-green-700'
                      : 'bg-gray-100 text-gray-700'
                  }`}>
                    {model.status === 'active' ? '活跃' : '禁用'}
                  </span>
                </div>

                {model.config && Object.keys(model.config).length > 0 && (
                  <details className="py-2 border-t border-gray-100">
                    <summary className="cursor-pointer text-gray-600 hover:text-gray-900">
                      配置详情
                    </summary>
                    <pre className="mt-2 p-2 bg-gray-50 rounded text-xs overflow-x-auto">
                      {JSON.stringify(model.config, null, 2)}
                    </pre>
                  </details>
                )}
              </div>
            </div>
            );
          })}
        </div>
      )}

      {/* Create/Edit Modal */}
      {showCreateModal && (
        <CreateModelModal
          model={editingModel}
          onClose={handleModalClose}
          onSuccess={fetchModels}
        />
      )}

      {/* Confirm Dialog */}
      <ConfirmDialog />
    </div>
  );
}
