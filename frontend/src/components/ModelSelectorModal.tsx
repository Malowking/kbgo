import { useState, useEffect } from 'react';
import { X, Search, Info } from 'lucide-react';
import { modelApi } from '@/services';
import type { Model } from '@/types';

interface ModelSelectorModalProps {
  onClose: () => void;
  onSelect: (model: Model) => void;
  currentModelId?: string;
  modelTypes?: Array<'llm' | 'multimodal'>;
}

export default function ModelSelectorModal({
  onClose,
  onSelect,
  currentModelId,
  modelTypes = ['llm', 'multimodal'],
}: ModelSelectorModalProps) {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedType, setSelectedType] = useState<'all' | 'llm' | 'multimodal'>('all');

  useEffect(() => {
    fetchModels();
  }, []);

  const fetchModels = async () => {
    try {
      setLoading(true);
      const response = await modelApi.list();
      // 过滤出 LLM 和多模态模型，并统一 ID 字段
      const filteredModels = response.models?.filter(m =>
        modelTypes.includes(m.type as 'llm' | 'multimodal')
      ).map(m => ({
        ...m,
        id: m.id || m.model_id, // 统一使用 id 字段
      })) || [];
      setModels(filteredModels);
    } catch (error) {
      console.error('Failed to fetch models:', error);
      alert('加载模型列表失败');
    } finally {
      setLoading(false);
    }
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
      console.error('Failed to parse model description:', e);
    }
    return '';
  };

  const filteredModels = models.filter((model) => {
    const matchesSearch = model.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      model.provider.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesType = selectedType === 'all' || model.type === selectedType;
    return matchesSearch && matchesType;
  });

  const handleSelectModel = (model: Model) => {
    onSelect(model);
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-4xl mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 sticky top-0 bg-white z-10">
          <h2 className="text-xl font-semibold text-gray-900">选择模型</h2>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-gray-100"
          >
            <X className="w-5 h-5 text-gray-500" />
          </button>
        </div>

        {/* Search and Filter */}
        <div className="p-6 border-b border-gray-200 bg-gray-50">
          <div className="flex gap-4">
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-4 h-4" />
              <input
                type="text"
                placeholder="搜索模型名称或提供商..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-10 pr-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <div className="flex gap-2">
              <button
                onClick={() => setSelectedType('all')}
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                  selectedType === 'all'
                    ? 'bg-blue-600 text-white'
                    : 'bg-white border text-gray-700 hover:bg-gray-50'
                }`}
              >
                全部
              </button>
              <button
                onClick={() => setSelectedType('llm')}
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                  selectedType === 'llm'
                    ? 'bg-blue-600 text-white'
                    : 'bg-white border text-gray-700 hover:bg-gray-50'
                }`}
              >
                LLM
              </button>
              <button
                onClick={() => setSelectedType('multimodal')}
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                  selectedType === 'multimodal'
                    ? 'bg-blue-600 text-white'
                    : 'bg-white border text-gray-700 hover:bg-gray-50'
                }`}
              >
                多模态
              </button>
            </div>
          </div>
        </div>

        {/* Model List */}
        <div className="p-6">
          {loading ? (
            <div className="text-center py-12">
              <div className="inline-block w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
              <p className="mt-4 text-gray-600">加载中...</p>
            </div>
          ) : filteredModels.length === 0 ? (
            <div className="text-center py-12">
              <Info className="w-12 h-12 text-gray-300 mx-auto mb-4" />
              <p className="text-gray-500">
                {searchQuery ? '没有找到匹配的模型' : '暂无可用模型'}
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {filteredModels.map((model) => {
                const description = getModelDescription(model);
                const isSelected = currentModelId === (model.id || model.model_id);

                return (
                  <button
                    key={model.model_id}
                    onClick={() => handleSelectModel(model)}
                    className={`text-left p-4 rounded-lg border-2 transition-all ${
                      isSelected
                        ? 'border-blue-600 bg-blue-50'
                        : 'border-gray-200 hover:border-blue-300 hover:bg-gray-50'
                    }`}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div className="flex-1">
                        <h3 className="font-semibold text-gray-900">{model.name}</h3>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-600">
                            {model.provider}
                          </span>
                          <span className={`text-xs px-2 py-0.5 rounded ${
                            model.type === 'llm'
                              ? 'bg-blue-100 text-blue-700'
                              : 'bg-purple-100 text-purple-700'
                          }`}>
                            {model.type === 'llm' ? 'LLM' : '多模态'}
                          </span>
                          {model.version && (
                            <span className="text-xs text-gray-500">v{model.version}</span>
                          )}
                        </div>
                      </div>
                      {isSelected && (
                        <div className="w-6 h-6 rounded-full bg-blue-600 flex items-center justify-center flex-shrink-0">
                          <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                          </svg>
                        </div>
                      )}
                    </div>

                    {description && (
                      <p className="text-sm text-gray-600 mt-2 line-clamp-2">
                        {description}
                      </p>
                    )}

                    {model.enabled === false && (
                      <div className="mt-2">
                        <span className="text-xs px-2 py-1 rounded bg-red-100 text-red-700">
                          已禁用
                        </span>
                      </div>
                    )}
                  </button>
                );
              })}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-6 border-t border-gray-200 bg-gray-50">
          <button
            onClick={onClose}
            className="btn btn-secondary w-full"
          >
            取消
          </button>
        </div>
      </div>
    </div>
  );
}