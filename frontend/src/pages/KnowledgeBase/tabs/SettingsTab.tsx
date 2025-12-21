import { useState, useCallback } from 'react';
import { Save, Edit2 } from 'lucide-react';
import { knowledgeBaseApi } from '@/services';
import type { KnowledgeBase } from '@/types';
import { formatDate } from '@/lib/utils';
import { logger } from '@/lib/logger';
import { showSuccess, showError, showWarning } from '@/lib/toast';

interface SettingsTabProps {
  kb: KnowledgeBase;
  onUpdate: () => void;
}

export default function SettingsTab({ kb, onUpdate }: SettingsTabProps) {
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(false);

  // Form state
  const [name, setName] = useState(kb.name);
  const [description, setDescription] = useState(kb.description);
  const [category, setCategory] = useState(kb.category || '');
  const [status, setStatus] = useState(kb.status);

  const handleSave = useCallback(async () => {
    if (!name.trim()) {
      showWarning('知识库名称不能为空');
      return;
    }

    try {
      setLoading(true);
      await knowledgeBaseApi.update(kb.id, {
        name: name.trim(),
        description: description.trim(),
        category: category.trim() || undefined,
        status,
      });
      showSuccess('保存成功');
      setEditing(false);
      onUpdate();
    } catch (error) {
      logger.error('Failed to update knowledge base:', error);
      showError('保存失败');
    } finally {
      setLoading(false);
    }
  }, [kb.id, name, description, category, status, onUpdate]);

  const handleCancel = () => {
    setName(kb.name);
    setDescription(kb.description);
    setCategory(kb.category || '');
    setStatus(kb.status);
    setEditing(false);
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="bg-white rounded-lg border p-6">
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold">知识库设置</h3>
          {!editing && (
            <button
              onClick={() => setEditing(true)}
              className="flex items-center gap-2 px-4 py-2 text-sm text-blue-600 border border-blue-200 rounded-lg hover:bg-blue-50 transition-colors"
            >
              <Edit2 className="w-4 h-4" />
              编辑
            </button>
          )}
        </div>

        <div className="space-y-6">
          {/* Basic Info */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              知识库名称 *
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!editing}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-50 disabled:text-gray-600"
              placeholder="例如：技术文档库"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              描述
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={!editing}
              rows={4}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-50 disabled:text-gray-600 resize-none"
              placeholder="描述知识库的用途和内容..."
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              分类
            </label>
            <input
              type="text"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              disabled={!editing}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-50 disabled:text-gray-600"
              placeholder="例如：技术、产品、运营"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              状态
            </label>
            <select
              value={status}
              onChange={(e) => setStatus(parseInt(e.target.value) as 1 | 2)}
              disabled={!editing}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-50 disabled:text-gray-600"
            >
              <option value={1}>启用</option>
              <option value={2}>禁用</option>
            </select>
            <p className="mt-1 text-xs text-gray-500">
              禁用后，该知识库将无法在对话和Agent中使用
            </p>
          </div>

          {/* Metadata */}
          <div className="pt-6 border-t">
            <h4 className="text-sm font-medium text-gray-700 mb-4">元信息</h4>
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="text-gray-500">知识库 ID:</span>
                <p className="text-gray-900 font-mono mt-1">{kb.id}</p>
              </div>
              <div>
                <span className="text-gray-500">Collection Name:</span>
                <p className="text-gray-900 font-mono mt-1">{kb.collectionName || '-'}</p>
              </div>
              <div>
                <span className="text-gray-500">创建时间:</span>
                <p className="text-gray-900 mt-1">{formatDate(kb.createTime)}</p>
              </div>
              <div>
                <span className="text-gray-500">更新时间:</span>
                <p className="text-gray-900 mt-1">{formatDate(kb.updateTime)}</p>
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          {editing && (
            <div className="flex justify-end gap-3 pt-6 border-t">
              <button
                onClick={handleCancel}
                disabled={loading}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={handleSave}
                disabled={loading}
                className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50"
              >
                <Save className="w-4 h-4" />
                {loading ? '保存中...' : '保存'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}