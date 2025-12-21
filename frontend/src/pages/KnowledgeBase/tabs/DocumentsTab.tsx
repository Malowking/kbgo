import { useState, useEffect, useCallback, useMemo } from 'react';
import { Upload, Search, Trash2, RefreshCw, FileText, CheckSquare, Square } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { documentApi } from '@/services';
import type { Document } from '@/types';
import { formatDate, formatBytes } from '@/lib/utils';
import { logger } from '@/lib/logger';
import { showSuccess, showError } from '@/lib/toast';
import UploadModal from '../UploadModal';
import IndexModal from '@/components/IndexModal';

interface DocumentsTabProps {
  kbId: string;
}

export default function DocumentsTab({ kbId }: DocumentsTabProps) {
  const navigate = useNavigate();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showUploadModal, setShowUploadModal] = useState(false);
  const [showIndexModal, setShowIndexModal] = useState(false);
  const [showReindexModal, setShowReindexModal] = useState(false);
  const [pendingDocumentIds, setPendingDocumentIds] = useState<string[]>([]);
  const [selectedDocs, setSelectedDocs] = useState<Set<string>>(new Set());

  const fetchDocuments = useCallback(async () => {
    if (!kbId) return;

    try {
      setLoading(true);
      const response = await documentApi.list({ knowledge_id: kbId });
      setDocuments(response.data || []);
    } catch (error) {
      logger.error('Failed to fetch documents:', error);
      showError('加载文档失败');
      setDocuments([]);
    } finally {
      setLoading(false);
    }
  }, [kbId]);

  useEffect(() => {
    if (kbId) {
      fetchDocuments();
    }
  }, [kbId, fetchDocuments]);

  const handleDelete = useCallback(async (docIds: string[]) => {
    if (!window.confirm(`确定要删除选中的 ${docIds.length} 个文档吗?`)) return;

    try {
      await documentApi.delete(docIds);
      showSuccess('删除成功');
      fetchDocuments();
      setSelectedDocs(new Set());
    } catch (error) {
      logger.error('Failed to delete documents:', error);
      showError('删除失败');
    }
  }, [fetchDocuments]);

  const handleUploadSuccess = (documentIds: string[]) => {
    // 上传成功后，打开索引模态框
    setPendingDocumentIds(documentIds);
    setShowIndexModal(true);
    fetchDocuments();
  };

  const handleOpenReindex = () => {
    // 打开重新索引模态框
    setPendingDocumentIds(Array.from(selectedDocs));
    setShowReindexModal(true);
  };

  const handleIndexSuccess = useCallback(() => {
    // 索引成功后刷新文档列表
    fetchDocuments();
    setSelectedDocs(new Set());
  }, [fetchDocuments]);

  const handleSelectDoc = useCallback((docId: string) => {
    const newSelected = new Set(selectedDocs);
    if (newSelected.has(docId)) {
      newSelected.delete(docId);
    } else {
      newSelected.add(docId);
    }
    setSelectedDocs(newSelected);
  }, [selectedDocs]);

  const filteredDocuments = useMemo(() =>
    documents.filter((doc) =>
      doc.fileName.toLowerCase().includes(searchQuery.toLowerCase())
    ),
    [documents, searchQuery]
  );

  const handleSelectAll = useCallback(() => {
    if (selectedDocs.size === filteredDocuments.length && filteredDocuments.length > 0) {
      setSelectedDocs(new Set());
    } else {
      setSelectedDocs(new Set(filteredDocuments.map(doc => doc.id)));
    }
  }, [selectedDocs, filteredDocuments]);

  return (
    <div className="p-6 space-y-6">
      {/* Actions Bar */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <button
            onClick={() => setShowUploadModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            <Upload className="w-4 h-4" />
            上传文档
          </button>

          {selectedDocs.size > 0 && (
            <>
              <button
                onClick={handleOpenReindex}
                className="flex items-center gap-2 px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors"
              >
                <RefreshCw className="w-4 h-4" />
                重新索引 ({selectedDocs.size})
              </button>

              <button
                onClick={() => handleDelete(Array.from(selectedDocs))}
                className="flex items-center gap-2 px-4 py-2 border border-red-200 text-red-600 rounded-lg hover:bg-red-50 transition-colors"
              >
                <Trash2 className="w-4 h-4" />
                删除 ({selectedDocs.size})
              </button>
            </>
          )}
        </div>

        <div className="relative w-64">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-4 h-4" />
          <input
            type="text"
            placeholder="搜索文档..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      {/* Documents Table */}
      {loading ? (
        <div className="bg-white rounded-lg border p-12 text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-gray-600">加载中...</p>
        </div>
      ) : filteredDocuments.length === 0 ? (
        <div className="bg-white rounded-lg border p-12 text-center">
          <FileText className="w-12 h-12 text-gray-300 mx-auto mb-4" />
          <p className="text-gray-500">
            {searchQuery ? '没有找到匹配的文档' : '还没有上传文档，点击上方按钮上传'}
          </p>
        </div>
      ) : (
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="w-12 px-6 py-3 text-left">
                  <button onClick={handleSelectAll} className="text-gray-500 hover:text-gray-700">
                    {selectedDocs.size === filteredDocuments.length && filteredDocuments.length > 0 ? (
                      <CheckSquare className="w-5 h-5" />
                    ) : (
                      <Square className="w-5 h-5" />
                    )}
                  </button>
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  文档名称
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  类型
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  大小
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  分块数
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  状态
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  上传时间
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {filteredDocuments.map((doc) => (
                <tr key={doc.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <button
                      onClick={() => handleSelectDoc(doc.id)}
                      className="text-gray-500 hover:text-gray-700"
                    >
                      {selectedDocs.has(doc.id) ? (
                        <CheckSquare className="w-5 h-5 text-blue-600" />
                      ) : (
                        <Square className="w-5 h-5" />
                      )}
                    </button>
                  </td>
                  <td className="px-6 py-4">
                    <button
                      onClick={() => navigate(`/kb/${kbId}/document/${doc.id}`)}
                      className="flex items-center hover:bg-gray-100 rounded px-2 py-1 -mx-2 transition-colors"
                    >
                      <FileText className="w-5 h-5 text-gray-400 mr-3" />
                      <span className="text-sm font-medium text-blue-600 hover:text-blue-700 hover:underline">
                        {doc.fileName}
                      </span>
                    </button>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">{doc.fileExtension}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">{doc.file_size ? formatBytes(doc.file_size) : '-'}</td>
                  <td className="px-6 py-4 text-sm text-gray-500">{doc.chunk_count || 0}</td>
                  <td className="px-6 py-4">
                    <span
                      className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${
                        doc.status === 2
                          ? 'bg-green-100 text-green-800'
                          : doc.status === 1
                          ? 'bg-yellow-100 text-yellow-800'
                          : doc.status === 3
                          ? 'bg-red-100 text-red-800'
                          : 'bg-gray-100 text-gray-800'
                      }`}
                    >
                      {doc.status === 2 ? '已完成' : doc.status === 1 ? '索引中' : doc.status === 3 ? '失败' : '待处理'}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">{formatDate(doc.CreateTime)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Upload Modal */}
      {showUploadModal && (
        <UploadModal
          kbId={kbId}
          onClose={() => setShowUploadModal(false)}
          onSuccess={handleUploadSuccess}
        />
      )}

      {/* Index Modal */}
      {showIndexModal && pendingDocumentIds.length > 0 && (
        <IndexModal
          documentIds={pendingDocumentIds}
          knowledgeBaseId={kbId}
          onClose={() => {
            setShowIndexModal(false);
            setPendingDocumentIds([]);
          }}
          onSuccess={handleIndexSuccess}
          isReindex={false}
        />
      )}

      {/* Reindex Modal */}
      {showReindexModal && pendingDocumentIds.length > 0 && (
        <IndexModal
          documentIds={pendingDocumentIds}
          knowledgeBaseId={kbId}
          onClose={() => {
            setShowReindexModal(false);
            setPendingDocumentIds([]);
          }}
          onSuccess={handleIndexSuccess}
          isReindex={true}
        />
      )}
    </div>
  );
}