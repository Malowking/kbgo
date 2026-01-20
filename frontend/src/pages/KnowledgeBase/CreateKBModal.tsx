import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { knowledgeBaseApi, modelApi } from '@/services';
import type { KnowledgeBase, CreateKBRequest, Model } from '@/types';
import { showError } from '@/lib/toast';

interface CreateKBModalProps {
  kb?: KnowledgeBase | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateKBModal({ kb, onClose, onSuccess }: CreateKBModalProps) {
  const [formData, setFormData] = useState<CreateKBRequest>({
    name: '',
    description: '',
    category: '',
    embedding_model_id: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [embeddingModels, setEmbeddingModels] = useState<Model[]>([]);
  const [loadingModels, setLoadingModels] = useState(true);

  // 加载 embedding 模型列表
  useEffect(() => {
    const fetchEmbeddingModels = async () => {
      try {
        setLoadingModels(true);
        const response = await modelApi.list({ model_type: 'embedding' });
        setEmbeddingModels(response.models || []);
      } catch (error) {
        console.error('Failed to load embedding models:', error);
        showError('加载 Embedding 模型列表失败');
      } finally {
        setLoadingModels(false);
      }
    };

    fetchEmbeddingModels();
  }, []);

  useEffect(() => {
    if (kb) {
      setFormData({
        name: kb.name,
        description: kb.description,
        category: kb.category || '',
        embedding_model_id: kb.embeddingModelId || '',
      });
    }
  }, [kb]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.name.trim()) {
      showError('请填写名称');
      return;
    }

    if (!formData.embedding_model_id && !kb) {
      showError('请选择 Embedding 模型');
      return;
    }

    try {
      setSubmitting(true);
      if (kb) {
        await knowledgeBaseApi.update(kb.id, formData);
      } else {
        await knowledgeBaseApi.create(formData);
      }
      onSuccess(); // 先刷新数据
      onClose(); // 再关闭模态框
    } catch (error) {
      console.error('Failed to save knowledge base:', error);
      showError('保存失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md mx-4">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">
            {kb ? '编辑知识库' : '创建知识库'}
          </h2>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-gray-100"
          >
            <X className="w-5 h-5 text-gray-500" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              名称 <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="input"
              placeholder="输入知识库名称"
              required
              maxLength={50}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              描述
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="input"
              placeholder="输入知识库描述（可选）"
              rows={4}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              分类
            </label>
            <input
              type="text"
              value={formData.category}
              onChange={(e) => setFormData({ ...formData, category: e.target.value })}
              className="input"
              placeholder="输入分类（可选）"
            />
          </div>

          {/* Embedding 模型选择 */}
          {!kb && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Embedding 模型 <span className="text-red-500">*</span>
              </label>
              {loadingModels ? (
                <div className="input text-gray-400">加载模型列表中...</div>
              ) : embeddingModels.length === 0 ? (
                <div className="input text-red-500">暂无可用的 Embedding 模型</div>
              ) : (
                <select
                  value={formData.embedding_model_id}
                  onChange={(e) => setFormData({ ...formData, embedding_model_id: e.target.value })}
                  className="input"
                  required
                  disabled={!!kb}
                >
                  <option value="">请选择 Embedding 模型</option>
                  {embeddingModels.map((model) => (
                    <option key={model.model_id} value={model.model_id}>
                      {model.name}
                    </option>
                  ))}
                </select>
              )}
              <p className="mt-1 text-xs text-gray-500">
                选择后将决定向量数据库的维度，创建后不可修改
              </p>
            </div>
          )}

          {/* 如果是编辑模式，显示当前绑定的模型 */}
          {kb && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Embedding 模型
              </label>
              <div className="input bg-gray-50 text-gray-600">
                {embeddingModels.find(m => m.model_id === kb.embeddingModelId)?.name || kb.embeddingModelId}
                <span className="text-xs text-gray-400 ml-2">(创建后不可修改)</span>
              </div>
            </div>
          )}

          {/* Actions */}
          <div className="flex justify-end space-x-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="btn btn-secondary"
              disabled={submitting}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting}
            >
              {submitting ? '保存中...' : kb ? '更新' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
