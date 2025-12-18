import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, FileText, Calendar, AlertCircle } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { documentApi } from '@/services';
import type { Chunk } from '@/types';
import { formatDate } from '@/lib/utils';

interface ExtData {
  chunk_index?: number;
}

export default function DocumentDetail() {
  const { kbId, docId } = useParams<{ kbId: string; docId: string }>();
  const navigate = useNavigate();
  const [chunks, setChunks] = useState<Chunk[]>([]);
  const [loading, setLoading] = useState(false);
  const [documentName, setDocumentName] = useState('加载中...');

  useEffect(() => {
    if (docId) {
      fetchChunks();
      fetchDocumentInfo();
    }
  }, [docId]);

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
        page: 1,
        page_size: 1000, // 获取所有分块
      });

      // 解析 ext 字段并按 chunk_index 排序
      const chunksWithIndex = response.data.map((chunk) => {
        let chunkIndex = 999999; // 默认值，用于没有 index 的分块
        try {
          if (chunk.ext) {
            const extData: ExtData = JSON.parse(chunk.ext);
            if (extData.chunk_index !== undefined) {
              chunkIndex = extData.chunk_index;
            }
          }
        } catch (e) {
          console.error('Failed to parse ext field:', e);
        }
        return { ...chunk, parsedIndex: chunkIndex };
      });

      // 按 chunk_index 排序
      chunksWithIndex.sort((a, b) => a.parsedIndex - b.parsedIndex);

      setChunks(chunksWithIndex);
    } catch (error) {
      console.error('Failed to fetch chunks:', error);
      alert('加载分块失败');
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
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {loading ? (
          <div className="bg-white rounded-lg border p-12 text-center">
            <div className="inline-block w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
            <p className="mt-4 text-gray-600">加载中...</p>
          </div>
        ) : chunks.length === 0 ? (
          <div className="bg-white rounded-lg border p-12 text-center">
            <AlertCircle className="w-12 h-12 text-gray-300 mx-auto mb-4" />
            <p className="text-gray-500">该文档还没有分块数据</p>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Stats */}
            <div className="bg-white rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <FileText className="w-5 h-5 text-blue-600" />
                  <span className="text-sm font-medium text-gray-700">
                    共 {chunks.length} 个分块
                  </span>
                </div>
              </div>
            </div>

            {/* Chunks List */}
            {chunks.map((chunk) => {
              const chunkIndex = getChunkIndex(chunk);
              return (
                <div
                  key={chunk.id}
                  className="bg-white rounded-lg border hover:shadow-md transition-shadow"
                >
                  <div className="p-6">
                    {/* Chunk Header */}
                    <div className="flex items-start justify-between mb-4">
                      <div className="flex items-center space-x-3">
                        <div className="flex items-center justify-center w-10 h-10 bg-blue-100 text-blue-600 rounded-lg font-semibold">
                          {chunkIndex >= 0 ? chunkIndex + 1 : '?'}
                        </div>
                        <div>
                          <div className="flex items-center space-x-2">
                            <Calendar className="w-4 h-4 text-gray-400" />
                            <span className="text-xs text-gray-500">
                              {formatDate(chunk.createTime)}
                            </span>
                          </div>
                        </div>
                      </div>

                      <span
                        className={`px-2 py-1 text-xs font-semibold rounded-full ${
                          chunk.status === 1
                            ? 'bg-green-100 text-green-800'
                            : 'bg-gray-100 text-gray-800'
                        }`}
                      >
                        {chunk.status === 1 ? '已索引' : '未索引'}
                      </span>
                    </div>

                    {/* Chunk Content */}
                    <div className="mt-4">
                      <div className="text-sm text-gray-700 prose max-w-none bg-gray-50 p-4 rounded-lg border">
                        <ReactMarkdown
                          remarkPlugins={[remarkGfm]}
                          components={{
                            // Images
                            img({ src, alt, ...props }) {
                              return (
                                <div className="my-2">
                                  <img
                                    src={src}
                                    alt={alt}
                                    className="max-w-full h-auto rounded-lg shadow-md"
                                    loading="lazy"
                                    {...props}
                                  />
                                  {alt && <p className="text-sm text-gray-500 mt-1 text-center italic">{alt}</p>}
                                </div>
                              );
                            },
                            // Paragraphs
                            p({ children, ...props }) {
                              return (
                                <p className="my-1 leading-relaxed" {...props}>
                                  {children}
                                </p>
                              );
                            },
                            // Code
                            code({ inline, children, ...props }: any) {
                              if (inline) {
                                return <code className="px-1 py-0.5 rounded bg-gray-200 text-gray-700 text-xs font-mono" {...props}>{children}</code>;
                              }
                              return <code className="block p-2 bg-gray-200 rounded text-xs font-mono overflow-x-auto" {...props}>{children}</code>;
                            },
                            // Headings
                            h1({ children, ...props }) {
                              return <h1 className="text-xl font-bold mt-4 mb-2" {...props}>{children}</h1>;
                            },
                            h2({ children, ...props }) {
                              return <h2 className="text-lg font-bold mt-3 mb-2" {...props}>{children}</h2>;
                            },
                            h3({ children, ...props }) {
                              return <h3 className="text-base font-bold mt-2 mb-1" {...props}>{children}</h3>;
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
          </div>
        )}
      </div>
    </div>
  );
}