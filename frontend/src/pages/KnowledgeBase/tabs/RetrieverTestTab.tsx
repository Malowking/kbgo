import { useState, useEffect, useCallback } from 'react';
import { Search, PlayCircle, Settings as SettingsIcon, Loader2 } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { modelApi } from '@/services';
import { getRerankModels } from '@/lib/model-utils';
import { logger } from '@/lib/logger';
import { showError, showWarning } from '@/lib/toast';
import type { Model } from '@/types';

interface RetrieverTestTabProps {
  kbId: string;
}

interface Document {
  content: string;
  score: number;
  metadata?: Record<string, any>;
}

export default function RetrieverTestTab({ kbId }: RetrieverTestTabProps) {
  const [question, setQuestion] = useState('');
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<Document[]>([]);

  // Models
  const [rerankModels, setRerankModels] = useState<Model[]>([]);

  // Parameters
  const [rerankModelId, setRerankModelId] = useState('');
  const [topK, setTopK] = useState(5);
  const [score, setScore] = useState(0.3);
  const [retrieveMode, setRetrieveMode] = useState<'milvus' | 'rerank' | 'rrf'>('rerank');
  const [rerankWeight, setRerankWeight] = useState(1.0);
  const [enableRewrite, setEnableRewrite] = useState(false);
  const [rewriteAttempts, setRewriteAttempts] = useState(3);

  const fetchModels = useCallback(async () => {
    try {
      const response = await modelApi.list();
      const allModels = response.models || [];

      // 使用工具函数获取启用的 rerank 模型
      const rerank = getRerankModels(allModels, true);
      setRerankModels(rerank);

      // Set default rerank model
      if (rerank.length > 0 && !rerankModelId) {
        setRerankModelId(rerank[0].model_id);
      }
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    }
  }, [rerankModelId]);

  useEffect(() => {
    fetchModels();
  }, [fetchModels]);

  const handleTest = useCallback(async () => {
    if (!question.trim()) {
      showWarning('请输入查询问题');
      return;
    }

    try {
      setLoading(true);
      setResults([]);

      const requestBody = {
        question: question.trim(),
        knowledge_id: kbId,
        rerank_model_id: rerankModelId || undefined,
        top_k: topK,
        score: score,
        retrieve_mode: retrieveMode,
        rerank_weight: rerankWeight,
        enable_rewrite: enableRewrite,
        rewrite_attempts: enableRewrite ? rewriteAttempts : undefined,
      };

      const response = await fetch('/api/v1/retriever', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = await response.json();

      if (data.code === 0) {
        // 如果有文档结果，设置结果；如果没有，设置空数组
        setResults(data.data?.document || []);
      } else {
        throw new Error(data.message || '召回测试失败');
      }
    } catch (error: any) {
      logger.error('Retriever test failed:', error);
      showError('召回测试失败: ' + (error.message || '未知错误'));
    } finally {
      setLoading(false);
    }
  }, [question, kbId, rerankModelId, topK, score, retrieveMode, rerankWeight, enableRewrite, rewriteAttempts]);

  return (
    <div className="p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
      {/* Left Panel - Configuration */}
      <div className="lg:col-span-1 space-y-6">
        {/* Query Input */}
        <div className="bg-white rounded-lg border p-4">
          <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Search className="w-5 h-5" />
            查询问题
          </h3>
          <textarea
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="输入要测试的查询问题..."
            className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
            rows={4}
          />
        </div>

        {/* Parameters */}
        <div className="bg-white rounded-lg border p-4">
          <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <SettingsIcon className="w-5 h-5" />
            召回参数
          </h3>

          <div className="space-y-4">
            {/* Info about embedding model */}
            <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
              <p className="text-xs text-blue-800">
                💡 Embedding 模型将自动使用知识库创建时绑定的模型
              </p>
            </div>

            {/* Retrieve Mode */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                检索模式
              </label>
              <select
                value={retrieveMode}
                onChange={(e) => setRetrieveMode(e.target.value as any)}
                className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="milvus">Milvus (向量召回)</option>
                <option value="rerank">Rerank (重排序)</option>
                <option value="rrf">RRF (融合排序)</option>
              </select>
            </div>

            {/* Rerank Model */}
            {(retrieveMode === 'rerank' || retrieveMode === 'rrf') && (
              <>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Rerank 模型
                  </label>
                  <select
                    value={rerankModelId}
                    onChange={(e) => setRerankModelId(e.target.value)}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  >
                    <option value="">选择模型</option>
                    {rerankModels.map((model) => (
                      <option key={model.model_id} value={model.model_id}>
                        {model.name}
                      </option>
                    ))}
                  </select>
                </div>

                {/* Rerank Weight Slider - Only for rerank mode */}
                {retrieveMode === 'rerank' && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Rerank 权重: {(rerankWeight * 100).toFixed(0)}%
                      <span className="text-xs text-gray-500 ml-2">
                        (BM25: {((1 - rerankWeight) * 100).toFixed(0)}%)
                      </span>
                    </label>
                    <input
                      type="range"
                      value={rerankWeight}
                      onChange={(e) => setRerankWeight(parseFloat(e.target.value))}
                      min={0}
                      max={1}
                      step={0.05}
                      className="w-full"
                    />
                    <div className="flex justify-between text-xs text-gray-500 mt-1">
                      <span>纯BM25</span>
                      <span>混合</span>
                      <span>纯Rerank</span>
                    </div>
                    <div className="mt-2 text-xs text-gray-600 bg-gray-50 rounded p-2">
                      {rerankWeight === 1.0 && '🔹 当前使用纯 Rerank 语义检索'}
                      {rerankWeight === 0.0 && '🔹 当前使用纯 BM25 关键词检索'}
                      {rerankWeight > 0 && rerankWeight < 1 && `🔹 混合检索：${(rerankWeight * 100).toFixed(0)}% Rerank + ${((1 - rerankWeight) * 100).toFixed(0)}% BM25`}
                    </div>
                  </div>
                )}
              </>
            )}

            {/* Top K */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Top K: {topK}
              </label>
              <input
                type="range"
                value={topK}
                onChange={(e) => setTopK(parseInt(e.target.value))}
                min={1}
                max={20}
                className="w-full"
              />
              <div className="flex justify-between text-xs text-gray-500 mt-1">
                <span>1</span>
                <span>20</span>
              </div>
            </div>

            {/* Score Threshold */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                相似度阈值: {score.toFixed(2)}
              </label>
              <input
                type="range"
                value={score}
                onChange={(e) => setScore(parseFloat(e.target.value))}
                min={0}
                max={1}
                step={0.05}
                className="w-full"
              />
              <div className="flex justify-between text-xs text-gray-500 mt-1">
                <span>0.0</span>
                <span>1.0</span>
              </div>
            </div>

            {/* Query Rewrite */}
            <div className="pt-4 border-t">
              <div className="flex items-center gap-2 mb-3">
                <input
                  type="checkbox"
                  id="enable_rewrite"
                  checked={enableRewrite}
                  onChange={(e) => setEnableRewrite(e.target.checked)}
                  className="w-4 h-4 text-blue-500 rounded"
                />
                <label htmlFor="enable_rewrite" className="text-sm font-medium text-gray-700">
                  启用查询重写
                </label>
              </div>

              {enableRewrite && (
                <div className="space-y-3">
                  <div className="bg-purple-50 border border-purple-200 rounded-lg p-3">
                    <p className="text-xs text-purple-800">
                      💡 查询重写将使用在「模型管理」页面配置的重写模型。如未配置重写模型，将跳过重写逻辑。
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      重写尝试次数
                    </label>
                    <input
                      type="number"
                      value={rewriteAttempts}
                      onChange={(e) => setRewriteAttempts(parseInt(e.target.value) || 3)}
                      min={1}
                      max={10}
                      className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Test Button */}
          <button
            onClick={handleTest}
            disabled={loading || !question.trim()}
            className="w-full mt-6 flex items-center justify-center gap-2 px-4 py-3 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? (
              <>
                <Loader2 className="w-5 h-5 animate-spin" />
                测试中...
              </>
            ) : (
              <>
                <PlayCircle className="w-5 h-5" />
                开始测试
              </>
            )}
          </button>
        </div>
      </div>

      {/* Right Panel - Results */}
      <div className="lg:col-span-2">
        <div className="bg-white rounded-lg border p-4">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">召回结果</h3>
            {results.length > 0 && (
              <span className="text-sm text-gray-500">
                共 {results.length} 条结果
              </span>
            )}
          </div>

          {loading ? (
            <div className="text-center py-12">
              <Loader2 className="w-8 h-8 animate-spin text-blue-500 mx-auto mb-4" />
              <p className="text-gray-600">正在召回文档...</p>
            </div>
          ) : results.length === 0 ? (
            <div className="text-center py-12">
              <Search className="w-12 h-12 text-gray-300 mx-auto mb-4" />
              <p className="text-gray-400">
                无召回结果
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {results.map((doc, index) => (
                <div key={index} className="border rounded-lg p-4 hover:shadow-sm transition-shadow">
                  <div className="flex items-start justify-between mb-2">
                    <span className="text-sm font-medium text-gray-700">
                      文档 #{index + 1}
                    </span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-gray-500">
                        相似度:
                      </span>
                      <span
                        className={`px-2 py-1 text-xs font-semibold rounded ${
                          doc.score >= 0.8
                            ? 'bg-green-100 text-green-800'
                            : doc.score >= 0.5
                            ? 'bg-yellow-100 text-yellow-800'
                            : 'bg-red-100 text-red-800'
                        }`}
                      >
                        {doc.score.toFixed(2)}
                      </span>
                    </div>
                  </div>

                  <div className="text-sm text-gray-700 prose max-w-none mb-3">
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
                            return <code className="px-1 py-0.5 rounded bg-gray-100 text-red-600 text-xs font-mono" {...props}>{children}</code>;
                          }
                          return <code className="block p-2 bg-gray-100 rounded text-xs font-mono overflow-x-auto" {...props}>{children}</code>;
                        },
                      }}
                    >
                      {doc.content}
                    </ReactMarkdown>
                  </div>

                  {doc.metadata && Object.keys(doc.metadata).length > 0 && (
                    <div className="pt-3 border-t">
                      <p className="text-xs font-medium text-gray-500 mb-2">元数据:</p>
                      <div className="flex flex-wrap gap-2">
                        {Object.entries(doc.metadata).map(([key, value]) => (
                          <span
                            key={key}
                            className="px-2 py-1 text-xs bg-gray-100 text-gray-700 rounded"
                          >
                            {key}: {String(value)}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
