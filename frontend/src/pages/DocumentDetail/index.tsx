import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, FileText, Calendar, AlertCircle } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { documentApi } from '@/services';
import type { Chunk } from '@/types';
import { formatDate } from '@/lib/utils';
import { showError } from '@/lib/toast';

interface ExtData {
  chunk_index?: number;
}

export default function DocumentDetail() {
  const { kbId, docId } = useParams<{ kbId: string; docId: string }>();
  const navigate = useNavigate();
  const [chunks, setChunks] = useState<Chunk[]>([]);
  const [loading, setLoading] = useState(false);
  const [documentName, setDocumentName] = useState('加载中...');
  const [currentPage, setCurrentPage] = useState(1);
  const [totalChunks, setTotalChunks] = useState(0);
  const pageSize = 20; // 增加每页显示数量

  useEffect(() => {
    if (docId) {
      fetchChunks();
      fetchDocumentInfo();
    }
  }, [docId, currentPage]);

  const fetchDocumentInfo = async () => {
    if (!docId || !kbId) return;

    try {
      // 获取文档列表，找到当前文档
      const response = await documentApi.list({ knowledge_id: kbId });
      const currentDoc = response.data?.find(doc => doc.id === docId);
      if (currentDoc) {
        setDocumentName(currentDoc.fileName);
      } else {
        setDocumentName('未知文档');
      }
    } catch (error) {
      console.error('Failed to fetch document info:', error);
      setDocumentName('未知文档');
    }
  };

  const fetchChunks = async () => {
    if (!docId) return;

    try {
      setLoading(true);
      const response = await documentApi.getChunks({
        knowledge_doc_id: docId,
        page: currentPage,
        size: pageSize,
      });

      // 后端已经按 chunk_order 排序好了，直接使用
      setChunks(response.data);
      setTotalChunks(response.total || 0);
    } catch (error) {
      console.error('Failed to fetch chunks:', error);
      showError('加载分块失败');
    } finally {
      setLoading(false);
    }
  };

  const getChunkIndex = (chunk: Chunk): number => {
    try {
      if (chunk.ext) {
        const extData: ExtData = JSON.parse(chunk.ext);
        return extData.chunk_index ?? -1;
      }
    } catch (e) {
      console.error('Failed to parse ext field:', e);
    }
    return -1;
  };

  const handleBack = () => {
    if (kbId) {
      navigate(`/knowledge-base/${kbId}`);
    } else {
      navigate(-1);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center space-x-4">
            <button
              onClick={handleBack}
              className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
            >
              <ArrowLeft className="w-5 h-5 text-gray-600" />
            </button>
            <div>
              <h1 className="text-2xl font-semibold text-gray-900">文档分块详情</h1>
              <p className="text-sm text-gray-500 mt-1">{documentName}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
        {loading ? (
          <div className="bg-white rounded-lg border p-8 text-center">
            <div className="inline-block w-6 h-6 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
            <p className="mt-3 text-sm text-gray-600">加载中...</p>
          </div>
        ) : chunks.length === 0 ? (
          <div className="bg-white rounded-lg border p-8 text-center">
            <AlertCircle className="w-10 h-10 text-gray-300 mx-auto mb-3" />
            <p className="text-sm text-gray-500">该文档还没有分块数据</p>
          </div>
        ) : (
          <div className="space-y-2">
            {/* Stats */}
            <div className="bg-white rounded border px-3 py-2">
              <div className="flex items-center space-x-2">
                <FileText className="w-4 h-4 text-blue-600" />
                <span className="text-xs font-medium text-gray-700">
                  共 {totalChunks} 个分块
                </span>
              </div>
            </div>

            {/* Chunks List */}
            {chunks.map((chunk) => {
              const chunkIndex = getChunkIndex(chunk);
              return (
                <div
                  key={chunk.id}
                  className="bg-white rounded border hover:shadow transition-shadow"
                >
                  <div className="p-3">
                    {/* Chunk Header */}
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center space-x-2">
                        <div className="flex items-center justify-center w-7 h-7 bg-blue-100 text-blue-600 rounded font-semibold text-xs">
                          {chunkIndex >= 0 ? chunkIndex + 1 : '?'}
                        </div>
                        <div className="flex items-center space-x-1.5">
                          <Calendar className="w-3 h-3 text-gray-400" />
                          <span className="text-xs text-gray-500">
                            {formatDate(chunk.createTime)}
                          </span>
                        </div>
                      </div>

                      <span
                        className={`px-1.5 py-0.5 text-xs font-semibold rounded ${
                          chunk.status === 1
                            ? 'bg-green-100 text-green-800'
                            : 'bg-gray-100 text-gray-800'
                        }`}
                      >
                        {chunk.status === 1 ? '已索引' : '未索引'}
                      </span>
                    </div>

                    {/* Chunk Content */}
                    <div>
                      <div className="text-sm text-gray-700 bg-gray-50 p-1.5 rounded border break-words whitespace-pre-wrap">
                        <ReactMarkdown
                          remarkPlugins={[remarkGfm]}
                          components={{
                            // Images
                            img({ src, alt, ...props }) {
                              return (
                                <div className="my-0.5">
                                  <img
                                    src={src}
                                    alt={alt}
                                    className="max-w-full h-auto rounded shadow-sm"
                                    loading="lazy"
                                    {...props}
                                  />
                                  {alt && <p className="text-xs text-gray-500 mt-0.5 text-center italic">{alt}</p>}
                                </div>
                              );
                            },
                            // Paragraphs
                            p({ children, ...props }) {
                              return (
                                <span className="block my-0 leading-[1.2] break-words" {...props}>
                                  {children}
                                </span>
                              );
                            },
                            // Links
                            a({ href, children, ...props }) {
                              return (
                                <a
                                  href={href}
                                  className="text-blue-600 hover:text-blue-800 underline break-all"
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  {...props}
                                >
                                  {children}
                                </a>
                              );
                            },
                            // Code
                            code({ inline, children, ...props }: any) {
                              if (inline) {
                                return <code className="px-1 py-0.5 rounded bg-gray-200 text-gray-700 text-xs font-mono break-all" {...props}>{children}</code>;
                              }
                              return <code className="block p-1.5 bg-gray-200 rounded text-xs font-mono overflow-x-auto whitespace-pre-wrap break-words leading-tight" {...props}>{children}</code>;
                            },
                            // Headings
                            h1({ children, ...props }) {
                              return <h1 className="text-base font-bold mt-1.5 mb-0.5 break-words" {...props}>{children}</h1>;
                            },
                            h2({ children, ...props }) {
                              return <h2 className="text-sm font-bold mt-1 mb-0.5 break-words" {...props}>{children}</h2>;
                            },
                            h3({ children, ...props }) {
                              return <h3 className="text-sm font-semibold mt-1 mb-0 break-words" {...props}>{children}</h3>;
                            },
                          }}
                        >
                          {chunk.content || '(空内容)'}
                        </ReactMarkdown>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}

            {/* Pagination */}
            {totalChunks > pageSize && (
              <div className="bg-white rounded border px-3 py-2 flex items-center justify-between sticky bottom-0">
                <div className="text-xs text-gray-600">
                  {(currentPage - 1) * pageSize + 1}-{Math.min(currentPage * pageSize, totalChunks)} / {totalChunks}
                </div>
                <div className="flex items-center space-x-2">
                  <button
                    onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                    disabled={currentPage === 1}
                    className="px-2 py-1 text-xs rounded border hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    上一页
                  </button>
                  <span className="text-xs text-gray-600">
                    {currentPage}/{Math.ceil(totalChunks / pageSize)}
                  </span>
                  <button
                    onClick={() => setCurrentPage(p => Math.min(Math.ceil(totalChunks / pageSize), p + 1))}
                    disabled={currentPage >= Math.ceil(totalChunks / pageSize)}
                    className="px-2 py-1 text-xs rounded border hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    下一页
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}