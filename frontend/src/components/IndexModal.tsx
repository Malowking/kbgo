import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { documentApi, modelApi } from '@/services';
import type { Model } from '@/types';

interface IndexModalProps {
  documentIds: string[];
  onClose: () => void;
  onSuccess: () => void;
  isReindex?: boolean;
}

export default function IndexModal({ documentIds, onClose, onSuccess, isReindex = false }: IndexModalProps) {
  const [embeddingModels, setEmbeddingModels] = useState<Model[]>([]);
  const [selectedModelId, setSelectedModelId] = useState('');
  const [chunkSize, setChunkSize] = useState(1000);
  const [overlapSize, setOverlapSize] = useState(100);
  const [separator, setSeparator] = useState('');
  const [useSeparator, setUseSeparator] = useState(false);
  const [indexing, setIndexing] = useState(false);

  // 加载 Embedding 模型列表
  useEffect(() => {
    const loadEmbeddingModels = async () => {
      try {
        const response = await modelApi.list({ model_type: 'embedding' });
        setEmbeddingModels(response.models || []);
        // 默认选择第一个模型
        if (response.models && response.models.length > 0) {
          setSelectedModelId(response.models[0].model_id);
        }
      } catch (error) {
        console.error('Failed to load embedding models:', error);
      }
    };

    loadEmbeddingModels();
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!selectedModelId) {
      alert('请选择 Embedding 模型');
      return;
    }

    if (useSeparator && !separator.trim()) {
      alert('请输入自定义分隔符');
      return;
    }

    try {
      setIndexing(true);

      const indexData: any = {
        document_ids: documentIds,
        embedding_model_id: selectedModelId,
        chunk_size: chunkSize,
        overlap_size: overlapSize,
      };

      // 只有选择使用自定义分隔符时才传递 separator 参数
      if (useSeparator && separator.trim()) {
        indexData.separator = separator.trim();
      }

      await documentApi.index(indexData);

      alert(isReindex ? '文档重新索引成功' : '文档索引成功');
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to index documents:', error);
      alert(isReindex ? '重新索引失败' : '索引失败');
    } finally {
      setIndexing(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">
            {isReindex ? '重新索引文档' : '索引文档'}
          </h2>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-gray-100"
          >
            <X className="w-5 h-5 text-gray-500" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {/* 选择的文档数量 */}
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <p className="text-sm text-blue-800">
              已选择 <span className="font-semibold">{documentIds.length}</span> 个文档进行{isReindex ? '重新' : ''}索引
            </p>
          </div>

          {/* Embedding 模型选择 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Embedding 模型 <span className="text-red-500">*</span>
            </label>
            <select
              value={selectedModelId}
              onChange={(e) => setSelectedModelId(e.target.value)}
              className="input"
              required
            >
              {embeddingModels.length === 0 ? (
                <option value="">暂无可用的 Embedding 模型</option>
              ) : (
                embeddingModels.map((model) => (
                  <option key={model.model_id} value={model.model_id}>
                    {model.name}
                  </option>
                ))
              )}
            </select>
            <p className="mt-1 text-xs text-gray-500">
              选择用于文档向量化的 Embedding 模型
            </p>
          </div>

          {/* 分块参数 */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                分块大小
              </label>
              <input
                type="number"
                value={chunkSize}
                onChange={(e) => setChunkSize(Number(e.target.value))}
                className="input"
                min="100"
                max="2000"
              />
              <p className="mt-1 text-xs text-gray-500">
                默认 1000 个字符
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                分块重叠
              </label>
              <input
                type="number"
                value={overlapSize}
                onChange={(e) => setOverlapSize(Number(e.target.value))}
                className="input"
                min="0"
                max="500"
              />
              <p className="mt-1 text-xs text-gray-500">
                默认 100 个字符
              </p>
            </div>
          </div>

          {/* 自定义分隔符选项 */}
          <div>
            <div className="flex items-center mb-2">
              <input
                type="checkbox"
                id="useSeparator"
                checked={useSeparator}
                onChange={(e) => setUseSeparator(e.target.checked)}
                className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
              />
              <label htmlFor="useSeparator" className="ml-2 text-sm font-medium text-gray-700">
                使用自定义分隔符
              </label>
            </div>

            {useSeparator && (
              <div>
                <input
                  type="text"
                  value={separator}
                  onChange={(e) => setSeparator(e.target.value)}
                  className="input"
                  placeholder="例如: \n\n 或 --- 等"
                />
                <p className="mt-1 text-xs text-gray-500">
                  自定义文档分割的分隔符，留空则使用默认分割策略
                </p>
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
            <button
              type="button"
              onClick={onClose}
              className="btn btn-secondary"
              disabled={indexing}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={indexing || embeddingModels.length === 0}
            >
              {indexing ? (isReindex ? '重新索引中...' : '索引中...') : (isReindex ? '重新索引' : '开始索引')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
