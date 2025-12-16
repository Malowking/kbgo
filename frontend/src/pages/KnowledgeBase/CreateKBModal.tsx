import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { knowledgeBaseApi } from '@/services';
import type { KnowledgeBase, CreateKBRequest } from '@/types';

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
  });
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (kb) {
      setFormData({
        name: kb.name,
        description: kb.description,
        category: kb.category || '',
      });
    }
  }, [kb]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.name.trim() || !formData.description.trim()) {
      alert('请填写名称和描述');
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
      alert('保存失败');
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
              minLength={3}
              maxLength={50}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              描述 <span className="text-red-500">*</span>
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="input"
              placeholder="输入知识库描述"
              rows={4}
              required
              minLength={3}
              maxLength={200}
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
              maxLength={50}
            />
          </div>

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