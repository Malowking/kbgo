import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { modelApi } from '@/services';
import type { Model } from '@/types';

interface CreateModelModalProps {
  model?: Model | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateModelModal({ model, onClose, onSuccess }: CreateModelModalProps) {
  const [formData, setFormData] = useState({
    model_name: '',
    model_type: 'llm' as 'llm' | 'embedding' | 'rerank' | 'multimodal' | 'image' | 'video' | 'audio',
    provider: '',
    base_url: '',
    api_key: '',
    max_completion_tokens: 0,
    dimension: 0,
    enabled: true,
    config: '',
  });
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (model) {
      setFormData({
        model_name: model.name,
        model_type: model.type as any,
        provider: model.provider || '',
        base_url: model.base_url || '',
        api_key: model.api_key || '',
        max_completion_tokens: 0,
        dimension: 0,
        enabled: model.status === 'active',
        config: model.config ? JSON.stringify(model.config, null, 2) : '',
      });
    }
  }, [model]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.model_name.trim()) {
      alert('请输入模型名称');
      return;
    }

    try {
      setLoading(true);

      let config: Record<string, any> | undefined;
      if (formData.config.trim()) {
        try {
          config = JSON.parse(formData.config);
        } catch {
          alert('配置格式错误，请输入有效的JSON');
          return;
        }
      }

      if (model) {
        // Update
        const modelId = model.id || model.model_id;
        await modelApi.update(modelId, {
          model_name: formData.model_name.trim(),
          model_type: formData.model_type,
          provider: formData.provider.trim() || undefined,
          base_url: formData.base_url.trim() || undefined,
          api_key: formData.api_key.trim() || undefined,
          enabled: formData.enabled,
          extra: config ? JSON.stringify(config) : undefined,
        });
      } else {
        // Create
        await modelApi.create({
          model_name: formData.model_name.trim(),
          model_type: formData.model_type,
          provider: formData.provider.trim() || undefined,
          base_url: formData.base_url.trim() || undefined,
          api_key: formData.api_key.trim() || undefined,
          max_completion_tokens: formData.max_completion_tokens || undefined,
          dimension: formData.dimension || undefined,
          config,
          enabled: formData.enabled,
        });
      }

      onSuccess(); // 先刷新数据
      onClose(); // 再关闭模态框
    } catch (error) {
      console.error('Failed to save model:', error);
      alert('保存失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">
            {model ? '编辑模型' : '添加模型'}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              模型名称 *
            </label>
            <input
              type="text"
              value={formData.model_name}
              onChange={(e) => setFormData({ ...formData, model_name: e.target.value })}
              className="input"
              placeholder="例如：gpt-4"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              模型类型 *
            </label>
            <select
              value={formData.model_type}
              onChange={(e) => setFormData({ ...formData, model_type: e.target.value as any })}
              className="input"
              required
            >
              <option value="llm">LLM（大语言模型）</option>
              <option value="embedding">Embedding（向量模型）</option>
              <option value="rerank">Rerank（重排序模型）</option>
              <option value="multimodal">Multimodal（多模态模型）</option>
              <option value="image">Image（图像模型）</option>
              <option value="video">Video（视频模型）</option>
              <option value="audio">Audio（音频模型）</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Provider
            </label>
            <input
              type="text"
              value={formData.provider}
              onChange={(e) => setFormData({ ...formData, provider: e.target.value })}
              className="input"
              placeholder="例如：openai, ollama, groq"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Base URL
            </label>
            <input
              type="url"
              value={formData.base_url}
              onChange={(e) => setFormData({ ...formData, base_url: e.target.value })}
              className="input"
              placeholder="https://api.openai.com/v1"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              API Key
            </label>
            <input
              type="password"
              value={formData.api_key}
              onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
              className="input"
              placeholder="sk-..."
            />
          </div>

          {!model && formData.model_type === 'llm' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                最大输出Token数
              </label>
              <input
                type="number"
                value={formData.max_completion_tokens}
                onChange={(e) => setFormData({ ...formData, max_completion_tokens: parseInt(e.target.value) || 0 })}
                className="input"
                min="0"
                placeholder="例如：4096"
              />
            </div>
          )}

          {!model && formData.model_type === 'embedding' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                向量维度
              </label>
              <input
                type="number"
                value={formData.dimension}
                onChange={(e) => setFormData({ ...formData, dimension: parseInt(e.target.value) || 0 })}
                className="input"
                min="0"
                placeholder="例如：1536"
              />
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              自定义配置（JSON格式）
            </label>
            <textarea
              value={formData.config}
              onChange={(e) => setFormData({ ...formData, config: e.target.value })}
              className="input font-mono text-sm min-h-[100px]"
              placeholder={'{\n  "temperature": 0.7,\n  "top_p": 0.9\n}'}
            />
          </div>

          <div className="flex items-center">
            <input
              type="checkbox"
              id="enabled"
              checked={formData.enabled}
              onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
              className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
            />
            <label htmlFor="enabled" className="ml-2 text-sm text-gray-700">
              启用此模型
            </label>
          </div>

          <div className="flex justify-end space-x-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="btn btn-secondary"
              disabled={loading}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={loading}
            >
              {loading ? '保存中...' : model ? '更新' : '添加'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
