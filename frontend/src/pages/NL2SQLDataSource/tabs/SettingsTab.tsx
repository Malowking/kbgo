import { Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { nl2sqlApi, modelApi } from '@/services';
import { showError, showSuccess } from '@/lib/toast';
import { useConfirm } from '@/hooks/useConfirm';
import { useState, useEffect } from 'react';
import type { Model } from '@/types';

interface DataSource {
  id: string;
  name: string;
  type: string;
  db_type?: string;
  status: string;
  create_time: string;
  embedding_model_id: string;
}

interface SettingsTabProps {
  datasource: DataSource;
  onUpdate: () => void;
}

export default function SettingsTab({ datasource }: SettingsTabProps) {
  const navigate = useNavigate();
  const { confirm, ConfirmDialog } = useConfirm();
  const [embeddingModelName, setEmbeddingModelName] = useState<string>('');
  const [loadingModel, setLoadingModel] = useState(true);

  // 加载 embedding 模型名称
  useEffect(() => {
    const fetchModelName = async () => {
      if (!datasource.embedding_model_id) {
        setLoadingModel(false);
        return;
      }

      try {
        setLoadingModel(true);
        const response = await modelApi.list({ model_type: 'embedding' });
        const model = response.models?.find(
          (m: Model) => m.model_id === datasource.embedding_model_id
        );
        if (model) {
          setEmbeddingModelName(model.name);
        } else {
          setEmbeddingModelName(datasource.embedding_model_id); // 如果找不到，显示 ID
        }
      } catch (error) {
        console.error('Failed to load embedding model:', error);
        setEmbeddingModelName(datasource.embedding_model_id); // 出错时显示 ID
      } finally {
        setLoadingModel(false);
      }
    };

    fetchModelName();
  }, [datasource.embedding_model_id]);

  const handleDelete = async () => {
    const confirmed = await confirm({
      message: '确定要删除这个数据源吗？相关的 Schema 信息也会被删除。',
      type: 'danger',
    });
    if (!confirmed) return;

    try {
      await nl2sqlApi.deleteDatasource(datasource.id);
      showSuccess('删除成功');
      navigate('/nl2sql-datasource');
    } catch (error) {
      console.error('Failed to delete datasource:', error);
      showError('删除失败');
    }
  };

  return (
    <div className="p-6 space-y-6">
      {/* Basic Info */}
      <div className="bg-white rounded-lg border p-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-4">基本信息</h3>
        <div className="space-y-3">
          <div className="flex items-center justify-between py-3 border-b">
            <span className="text-sm font-medium text-gray-700">数据源名称</span>
            <span className="text-sm text-gray-900">{datasource.name}</span>
          </div>
          <div className="flex items-center justify-between py-3 border-b">
            <span className="text-sm font-medium text-gray-700">数据源类型</span>
            <span className="text-sm text-gray-900">
              {datasource.type === 'jdbc'
                ? `JDBC (${datasource.db_type})`
                : datasource.type.toUpperCase()}
            </span>
          </div>
          <div className="flex items-center justify-between py-3 border-b">
            <span className="text-sm font-medium text-gray-700">状态</span>
            <span
              className={`px-2 py-1 text-xs rounded ${
                datasource.status === 'active'
                  ? 'bg-green-100 text-green-700'
                  : 'bg-gray-100 text-gray-700'
              }`}
            >
              {datasource.status === 'active' ? '活跃' : datasource.status}
            </span>
          </div>
          <div className="flex items-center justify-between py-3 border-b">
            <span className="text-sm font-medium text-gray-700">Embedding模型</span>
            <span className="text-sm text-gray-900">
              {loadingModel ? (
                <span className="text-gray-400">加载中...</span>
              ) : (
                embeddingModelName || '未设置'
              )}
            </span>
          </div>
          <div className="flex items-center justify-between py-3">
            <span className="text-sm font-medium text-gray-700">创建时间</span>
            <span className="text-sm text-gray-900">
              {new Date(datasource.create_time).toLocaleString()}
            </span>
          </div>
        </div>
      </div>

      {/* Danger Zone */}
      <div className="bg-white rounded-lg border border-red-200 p-6">
        <h3 className="text-lg font-semibold text-red-700 mb-4">危险操作</h3>
        <div className="space-y-4">
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <h4 className="text-sm font-medium text-gray-900 mb-1">删除数据源</h4>
              <p className="text-sm text-gray-600">
                删除此数据源将同时删除所有相关的Schema元数据、向量索引和查询日志。此操作不可撤销。
              </p>
            </div>
            <button
              onClick={handleDelete}
              className="ml-4 flex items-center gap-2 px-4 py-2 bg-red-500 text-white rounded-lg hover:bg-red-600 transition-colors"
            >
              <Trash2 className="w-4 h-4" />
              删除数据源
            </button>
          </div>
        </div>
      </div>

      <ConfirmDialog />
    </div>
  );
}