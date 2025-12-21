import { useState, useEffect } from 'react';
import { X, ChevronDown, ChevronUp } from 'lucide-react';
import { modelApi } from '@/services';
import type { Model } from '@/types';
import { logger } from '@/lib/logger';
import { showError, showWarning } from '@/lib/toast';

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
    max_completion_tokens: 3000,
    dimension: 1024,
    enabled: true,
    // 额外参数
    temperature: 0.7,
    topP: 0.9,
    frequencyPenalty: 0,
    presencePenalty: 0,
    stop: [] as string[],
    config: '',
  });
  const [loading, setLoading] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [stopInput, setStopInput] = useState('');

  useEffect(() => {
    if (model) {
      // 从 extra 和 config 中提取额外参数和描述
      let extraParams = {
        temperature: 0.7,
        topP: 0.9,
        frequencyPenalty: 0,
        presencePenalty: 0,
        stop: [] as string[],
        max_completion_tokens: 3000,
      };
      let dimension = 1024;
      let description = '';

      // 先从 extra 字段提取参数
      if (model.extra && typeof model.extra === 'object') {
        extraParams = {
          temperature: (model.extra as any).temperature ?? 0.7,
          topP: (model.extra as any).topP ?? 0.9,
          frequencyPenalty: (model.extra as any).frequencyPenalty ?? 0,
          presencePenalty: (model.extra as any).presencePenalty ?? 0,
          stop: (model.extra as any).stop ?? [],
          max_completion_tokens: (model.extra as any).max_completion_tokens ?? 3000,
        };
        dimension = (model.extra as any).dimension ?? 1024;
        description = (model.extra as any).description || '';
      }
      // 如果 extra 为空，尝试从 config 提取
      else if (model.config && typeof model.config === 'object') {
        extraParams = {
          temperature: (model.config as any).temperature ?? 0.7,
          topP: (model.config as any).topP ?? 0.9,
          frequencyPenalty: (model.config as any).frequencyPenalty ?? 0,
          presencePenalty: (model.config as any).presencePenalty ?? 0,
          stop: (model.config as any).stop ?? [],
          max_completion_tokens: (model.config as any).max_completion_tokens ?? 3000,
        };
        dimension = (model.config as any).dimension ?? 1024;
        description = (model.config as any).description || '';
      }

      setFormData({
        model_name: model.name,
        model_type: model.type as any,
        provider: model.provider || '',
        base_url: model.base_url || '',
        api_key: model.api_key || '',
        dimension: dimension,
        enabled: model.status === 'active' || model.enabled === true,
        config: description,
        ...extraParams,
      });
      setStopInput(extraParams.stop.join(', '));
    }
  }, [model]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.model_name.trim()) {
      showWarning('请输入模型名称');
      return;
    }

    try {
      setLoading(true);

      // 根据模型类型构建不同的配置对象
      let config: Record<string, any> = {};

      if (formData.model_type === 'llm' || formData.model_type === 'multimodal') {
        // LLM 和 Multimodal 模型的配置
        const stopArray = stopInput
          .split(',')
          .map(s => s.trim())
          .filter(s => s.length > 0);

        config = {
          temperature: formData.temperature,
          topP: formData.topP,
          frequencyPenalty: formData.frequencyPenalty,
          presencePenalty: formData.presencePenalty,
          stop: stopArray,
          max_completion_tokens: formData.max_completion_tokens,
        };
      } else if (formData.model_type === 'embedding') {
        // Embedding 模型的配置
        config = {
          dimension: formData.dimension,
        };
      }
      // Rerank 模型不需要额外配置，config 保持为空对象

      // 添加描述（如果有）
      if (formData.config.trim()) {
        config.description = formData.config.trim();
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
          extra: JSON.stringify(config),
        });
      } else {
        // Create
        await modelApi.create({
          model_name: formData.model_name.trim(),
          model_type: formData.model_type,
          provider: formData.provider.trim() || undefined,
          base_url: formData.base_url.trim() || undefined,
          api_key: formData.api_key.trim() || undefined,
          max_completion_tokens: formData.model_type === 'llm' || formData.model_type === 'multimodal' ? formData.max_completion_tokens : undefined,
          dimension: formData.model_type === 'embedding' ? formData.dimension : undefined,
          config,
          enabled: formData.enabled,
        });
      }

      onSuccess(); // 先刷新数据
      onClose(); // 再关闭模态框
    } catch (error) {
      logger.error('Failed to save model:', error);
      showError('保存失败');
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
              <option value="reranker">Reranker（重排序模型）</option>
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

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              模型描述
            </label>
            <textarea
              value={formData.config}
              onChange={(e) => setFormData({ ...formData, config: e.target.value })}
              className="input min-h-[80px]"
              placeholder="请输入模型描述信息"
            />
          </div>

          {formData.model_type === 'embedding' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                向量维度 <span className="text-red-500">*</span>
              </label>
              <input
                type="number"
                value={formData.dimension}
                onChange={(e) => setFormData({ ...formData, dimension: parseInt(e.target.value) || 0 })}
                className="input"
                min="1"
                max="10000"
                placeholder="1024"
                required
              />
              <p className="text-xs text-gray-500 mt-1">
                常见维度：768 (BERT), 1024 (小模型), 1536 (OpenAI), 3072 (大模型)
              </p>
            </div>
          )}

          {/* 额外参数按钮 - LLM 和 Multimodal 显示完整参数，Rerank 不显示 */}
          {(formData.model_type === 'llm' || formData.model_type === 'multimodal') && (
            <div className="border-t border-gray-200 pt-4">
              <button
                type="button"
                onClick={() => setShowAdvanced(!showAdvanced)}
                className="flex items-center justify-between w-full px-4 py-2 text-sm font-medium text-gray-700 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors"
              >
                <span>额外参数配置</span>
                {showAdvanced ? (
                  <ChevronUp className="w-4 h-4" />
                ) : (
                  <ChevronDown className="w-4 h-4" />
                )}
              </button>

              {/* 可折叠的额外参数表单 */}
              {showAdvanced && (
                <div className="mt-4 space-y-4 p-4 bg-gray-50 rounded-lg">
                  <div className="grid grid-cols-2 gap-4">
                    {/* Max Completion Tokens */}
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
                        placeholder="3000"
                      />
                      <p className="text-xs text-gray-500 mt-1">控制最大生成长度</p>
                    </div>

                    {/* Temperature */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Temperature
                        <span className="text-xs text-gray-500 ml-1">(0-2)</span>
                      </label>
                      <input
                        type="number"
                        value={formData.temperature}
                        onChange={(e) => setFormData({ ...formData, temperature: parseFloat(e.target.value) || 0 })}
                        className="input"
                        min="0"
                        max="2"
                        step="0.1"
                        placeholder="0.7"
                      />
                      <p className="text-xs text-gray-500 mt-1">控制输出随机性，越高越随机</p>
                    </div>

                    {/* Top P */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Top P
                        <span className="text-xs text-gray-500 ml-1">(0-1)</span>
                      </label>
                      <input
                        type="number"
                        value={formData.topP}
                        onChange={(e) => setFormData({ ...formData, topP: parseFloat(e.target.value) || 0 })}
                        className="input"
                        min="0"
                        max="1"
                        step="0.1"
                        placeholder="0.9"
                      />
                      <p className="text-xs text-gray-500 mt-1">核采样概率阈值</p>
                    </div>

                    {/* Frequency Penalty */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Frequency Penalty
                        <span className="text-xs text-gray-500 ml-1">(-2 to 2)</span>
                      </label>
                      <input
                        type="number"
                        value={formData.frequencyPenalty}
                        onChange={(e) => setFormData({ ...formData, frequencyPenalty: parseFloat(e.target.value) || 0 })}
                        className="input"
                        min="-2"
                        max="2"
                        step="0.1"
                        placeholder="0"
                      />
                      <p className="text-xs text-gray-500 mt-1">降低重复词频率</p>
                    </div>

                    {/* Presence Penalty */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Presence Penalty
                        <span className="text-xs text-gray-500 ml-1">(-2 to 2)</span>
                      </label>
                      <input
                        type="number"
                        value={formData.presencePenalty}
                        onChange={(e) => setFormData({ ...formData, presencePenalty: parseFloat(e.target.value) || 0 })}
                        className="input"
                        min="-2"
                        max="2"
                        step="0.1"
                        placeholder="0"
                      />
                      <p className="text-xs text-gray-500 mt-1">鼓励讨论新话题</p>
                    </div>
                  </div>

                  {/* Stop Sequences */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Stop Sequences
                      <span className="text-xs text-gray-500 ml-1">(用逗号分隔)</span>
                    </label>
                    <input
                      type="text"
                      value={stopInput}
                      onChange={(e) => setStopInput(e.target.value)}
                      className="input"
                      placeholder="例如：\n, ###, END"
                    />
                    <p className="text-xs text-gray-500 mt-1">模型遇到这些序列时将停止生成</p>
                  </div>
                </div>
              )}
            </div>
          )}

          {/*<div>*/}
          {/*  <label className="block text-sm font-medium text-gray-700 mb-2">*/}
          {/*    自定义配置（JSON格式）*/}
          {/*  </label>*/}
          {/*  <textarea*/}
          {/*    value={formData.config}*/}
          {/*    onChange={(e) => setFormData({ ...formData, config: e.target.value })}*/}
          {/*    className="input font-mono text-sm min-h-[100px]"*/}
          {/*    placeholder={'{\n  "description": "模型描述"\n}'}*/}
          {/*  />*/}
          {/*  <p className="text-xs text-gray-500 mt-1">可选：用于添加其他自定义字段（如 description 等）</p>*/}
          {/*</div>*/}

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
