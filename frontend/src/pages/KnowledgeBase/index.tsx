import { useState, useEffect } from 'react';
import { Plus, Search, Edit2, Trash2, Power, PowerOff } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { knowledgeBaseApi } from '@/services';
import { useAppStore } from '@/store';
import type { KnowledgeBase } from '@/types';
import { formatDate } from '@/lib/utils';
import CreateKBModal from './CreateKBModal';

export default function KnowledgeBasePage() {
  const navigate = useNavigate();
  const [kbList, setKbList] = useState<KnowledgeBase[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingKB, setEditingKB] = useState<KnowledgeBase | null>(null);
  const { setCurrentKB } = useAppStore();

  useEffect(() => {
    fetchKBList();
  }, []);

  const fetchKBList = async () => {
    try {
      setLoading(true);
      const response = await knowledgeBaseApi.list();
      setKbList(response.list || []);
    } catch (error) {
      console.error('Failed to fetch knowledge bases:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个知识库吗?')) return;

    try {
      await knowledgeBaseApi.delete(id);
      fetchKBList();
    } catch (error) {
      console.error('Failed to delete knowledge base:', error);
      alert('删除失败');
    }
  };

  const handleToggleStatus = async (kb: KnowledgeBase) => {
    try {
      const newStatus = kb.status === 1 ? 2 : 1;
      await knowledgeBaseApi.updateStatus(kb.id, newStatus);
      fetchKBList();
    } catch (error) {
      console.error('Failed to update status:', error);
      alert('状态更新失败');
    }
  };

  const handleEdit = (kb: KnowledgeBase) => {
    setEditingKB(kb);
    setShowCreateModal(true);
  };

  const handleModalClose = () => {
    setShowCreateModal(false);
    setEditingKB(null);
  };

  const filteredKBList = kbList.filter((kb) =>
    kb.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    kb.description.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">知识库管理</h1>
          <p className="mt-2 text-gray-600">
            创建和管理您的知识库
          </p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="btn btn-primary flex items-center"
        >
          <Plus className="w-5 h-5 mr-2" />
          创建知识库
        </button>
      </div>

      {/* Search */}
      <div className="card">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
          <input
            type="text"
            placeholder="搜索知识库..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="input pl-10"
          />
        </div>
      </div>

      {/* Knowledge Base List */}
      {loading ? (
        <div className="text-center py-12">
          <div className="inline-block w-8 h-8 border-4 border-primary-600 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-gray-600">加载中...</p>
        </div>
      ) : filteredKBList.length === 0 ? (
        <div className="card text-center py-12">
          <p className="text-gray-500">
            {searchQuery ? '没有找到匹配的知识库' : '还没有知识库，点击上方按钮创建一个'}
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredKBList.map((kb) => (
            <div
              key={kb.id}
              className="card hover:shadow-md transition-shadow cursor-pointer"
              onClick={() => {
                setCurrentKB(kb);
                navigate(`/knowledge-base/${kb.id}`);
              }}
            >
              <div className="flex items-start justify-between mb-4">
                <div className="flex-1">
                  <h3 className="text-lg font-semibold text-gray-900 mb-1">
                    {kb.name}
                  </h3>
                  {kb.category && (
                    <span className="inline-block px-2 py-1 text-xs font-medium text-primary-700 bg-primary-50 rounded">
                      {kb.category}
                    </span>
                  )}
                </div>
                <div className="flex space-x-2" onClick={(e) => e.stopPropagation()}>
                  <button
                    onClick={() => handleToggleStatus(kb)}
                    className={`p-1.5 rounded hover:bg-gray-100 ${
                      kb.status === 1 ? 'text-green-600' : 'text-gray-400'
                    }`}
                    title={kb.status === 1 ? '禁用' : '启用'}
                  >
                    {kb.status === 1 ? <Power className="w-4 h-4" /> : <PowerOff className="w-4 h-4" />}
                  </button>
                  <button
                    onClick={() => handleEdit(kb)}
                    className="p-1.5 rounded hover:bg-gray-100 text-blue-600"
                    title="编辑"
                  >
                    <Edit2 className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleDelete(kb.id)}
                    className="p-1.5 rounded hover:bg-gray-100 text-red-600"
                    title="删除"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              <p className="text-gray-600 text-sm mb-4 line-clamp-2">
                {kb.description}
              </p>

              <div className="flex items-center justify-between text-xs text-gray-500 pt-4 border-t border-gray-100">
                <span>创建于 {formatDate(kb.createTime)}</span>
                <span className={`px-2 py-1 rounded ${
                  kb.status === 1
                    ? 'bg-green-100 text-green-700'
                    : 'bg-gray-100 text-gray-700'
                }`}>
                  {kb.status === 1 ? '启用' : '禁用'}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Create/Edit Modal */}
      {showCreateModal && (
        <CreateKBModal
          kb={editingKB}
          onClose={handleModalClose}
          onSuccess={fetchKBList}
        />
      )}
    </div>
  );
}