import { useState, useEffect } from 'react';
import { Upload, Search, Trash2, RefreshCw, FileText } from 'lucide-react';
import { documentApi, knowledgeBaseApi } from '@/services';
import { useAppStore } from '@/store';
import type { Document, KnowledgeBase } from '@/types';
import { formatDate, formatBytes } from '@/lib/utils';
import UploadModal from './UploadModal';

export default function Documents() {
  const [documents, setDocuments] = useState<Document[]>([]);
  const [kbList, setKbList] = useState<KnowledgeBase[]>([]);
  const [selectedKB, setSelectedKB] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showUploadModal, setShowUploadModal] = useState(false);
  const [selectedDocs, setSelectedDocs] = useState<Set<string>>(new Set());
  const { currentKB } = useAppStore();

  useEffect(() => {
    fetchKBList();
  }, []);

  useEffect(() => {
    if (currentKB) {
      setSelectedKB(currentKB.id);
    }
  }, [currentKB]);

  useEffect(() => {
    if (selectedKB) {
      fetchDocuments();
    }
  }, [selectedKB]);

  const fetchKBList = async () => {
    try {
      const response = await knowledgeBaseApi.list();
      setKbList(response.list || []);
    } catch (error) {
      console.error('Failed to fetch knowledge bases:', error);
    }
  };

  const fetchDocuments = async () => {
    if (!selectedKB) return;

    try {
      setLoading(true);
      const response = await documentApi.list({ knowledge_id: selectedKB });
      setDocuments(response.data || []);
    } catch (error) {
      console.error('Failed to fetch documents:', error);
      setDocuments([]); // 错误时清空列表
      alert('加载文档失败: ' + (error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (docIds: string[]) => {
    if (!confirm(`确定要删除选中的 ${docIds.length} 个文档吗?`)) return;

    try {
      await documentApi.delete(docIds);
      fetchDocuments();
      setSelectedDocs(new Set());
    } catch (error) {
      console.error('Failed to delete documents:', error);
      alert('删除失败');
    }
  };

  const handleReindex = async (docIds: string[]) => {
    if (!confirm(`确定要重新索引选中的 ${docIds.length} 个文档吗?`)) return;

    try {
      await documentApi.reindex(docIds);
      alert('重新索引任务已提交');
      fetchDocuments();
    } catch (error) {
      console.error('Failed to reindex documents:', error);
      alert('重新索引失败');
    }
  };

  const handleSelectDoc = (docId: string) => {
    const newSelected = new Set(selectedDocs);
    if (newSelected.has(docId)) {
      newSelected.delete(docId);
    } else {
      newSelected.add(docId);
    }
    setSelectedDocs(newSelected);
  };

  const handleSelectAll = () => {
    if (selectedDocs.size === filteredDocuments.length) {
      setSelectedDocs(new Set());
    } else {
      setSelectedDocs(new Set(filteredDocuments.map(doc => doc.id)));
    }
  };

  const filteredDocuments = documents.filter((doc) =>
    doc.fileName.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">文档管理</h1>
          <p className="mt-2 text-gray-600">
            上传、管理和索引您的文档
          </p>
        </div>
        <button
          onClick={() => setShowUploadModal(true)}
          className="btn btn-primary flex items-center"
          disabled={!selectedKB}
        >
          <Upload className="w-5 h-5 mr-2" />
          上传文档
        </button>
      </div>

      {/* Filters */}
      <div className="card space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            选择知识库
          </label>
          <select
            value={selectedKB}
            onChange={(e) => setSelectedKB(e.target.value)}
            className="input"
          >
            <option value="">请选择知识库</option>
            {kbList.map((kb) => (
              <option key={kb.id} value={kb.id}>
                {kb.name}
              </option>
            ))}
          </select>
        </div>

        <div className="relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
          <input
            type="text"
            placeholder="搜索文档..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="input pl-10"
          />
        </div>
      </div>

      {/* Batch Actions */}
      {selectedDocs.size > 0 && (
        <div className="card flex items-center justify-between">
          <span className="text-sm text-gray-600">
            已选中 {selectedDocs.size} 个文档
          </span>
          <div className="flex space-x-2">
            <button
              onClick={() => handleReindex(Array.from(selectedDocs))}
              className="btn btn-secondary flex items-center text-sm"
            >
              <RefreshCw className="w-4 h-4 mr-2" />
              重新索引
            </button>
            <button
              onClick={() => handleDelete(Array.from(selectedDocs))}
              className="btn btn-danger flex items-center text-sm"
            >
              <Trash2 className="w-4 h-4 mr-2" />
              删除
            </button>
          </div>
        </div>
      )}

      {/* Document List */}
      {!selectedKB ? (
        <div className="card text-center py-12">
          <p className="text-gray-500">请先选择一个知识库</p>
        </div>
      ) : loading ? (
        <div className="text-center py-12">
          <div className="inline-block w-8 h-8 border-4 border-primary-600 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-gray-600">加载中...</p>
        </div>
      ) : filteredDocuments.length === 0 ? (
        <div className="card text-center py-12">
          <FileText className="w-16 h-16 text-gray-300 mx-auto mb-4" />
          <p className="text-gray-500">
            {searchQuery ? '没有找到匹配的文档' : '还没有文档，点击上方按钮上传文档'}
          </p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                <th className="px-6 py-3 text-left">
                  <input
                    type="checkbox"
                    checked={selectedDocs.size === filteredDocuments.length && filteredDocuments.length > 0}
                    onChange={handleSelectAll}
                    className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  />
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  文档名称
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  类型
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  大小
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  分块数
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  状态
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  创建时间
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  操作
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {filteredDocuments.map((doc) => (
                <tr key={doc.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <input
                      type="checkbox"
                      checked={selectedDocs.has(doc.id)}
                      onChange={() => handleSelectDoc(doc.id)}
                      className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    />
                  </td>
                  <td className="px-6 py-4 text-sm font-medium text-gray-900">
                    {doc.fileName}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-600">
                    {doc.fileExtension}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-600">
                    {doc.file_size ? formatBytes(doc.file_size) : '-'}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-600">
                    {doc.chunk_count || 0}
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs rounded ${
                      doc.status === 2
                        ? 'bg-green-100 text-green-700'
                        : doc.status === 1
                        ? 'bg-yellow-100 text-yellow-700'
                        : doc.status === 3
                        ? 'bg-red-100 text-red-700'
                        : 'bg-gray-100 text-gray-700'
                    }`}>
                      {doc.status === 2 ? '已完成' : doc.status === 1 ? '索引中' : doc.status === 3 ? '失败' : '待处理'}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-600">
                    {formatDate(doc.CreateTime)}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex space-x-2">
                      <button
                        onClick={() => handleReindex([doc.id])}
                        className="p-1 text-blue-600 hover:bg-blue-50 rounded"
                        title="重新索引"
                      >
                        <RefreshCw className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete([doc.id])}
                        className="p-1 text-red-600 hover:bg-red-50 rounded"
                        title="删除"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Upload Modal */}
      {showUploadModal && (
        <UploadModal
          kbId={selectedKB}
          onClose={() => setShowUploadModal(false)}
          onSuccess={fetchDocuments}
        />
      )}
    </div>
  );
}