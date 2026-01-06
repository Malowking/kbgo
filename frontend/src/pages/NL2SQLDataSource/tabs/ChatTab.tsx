import { useState, useRef, useEffect } from 'react';
import { Send, Loader2, Database, Code, Table as TableIcon, AlertCircle, Download } from 'lucide-react';
import { nl2sqlApi, modelApi } from '@/services';
import { showError, showSuccess } from '@/lib/toast';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  sql?: string;
  result?: {
    columns: string[];
    data: any[];
    row_count: number;
  };
  explanation?: string;
  error?: string;
  timestamp: Date;
}

interface ChatTabProps {
  datasource: {
    id: string;
    name: string;
    type: string;
    db_type?: string;
    embedding_model_id: string;
  };
}

export default function ChatTab({ datasource }: ChatTabProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [llmModel, setLlmModel] = useState<string>('');
  const [models, setModels] = useState<any[]>([]);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetchModels();
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const fetchModels = async () => {
    try {
      const response = await modelApi.list();
      const llmModels = response.models?.filter((m: any) => m.type === 'llm') || [];
      setModels(llmModels);

      // 选择第一个可用的LLM模型
      if (llmModels.length > 0) {
        setLlmModel(llmModels[0].model_id);
      } else {
        showError('没有可用的LLM模型，请先在模型管理中添加LLM模型');
      }
    } catch (error) {
      console.error('Failed to fetch models:', error);
      showError('获取模型列表失败');
    }
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const handleSend = async () => {
    if (!input.trim() || loading) return;

    if (!llmModel) {
      showError('请先选择一个LLM模型');
      return;
    }

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input.trim(),
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput('');
    setLoading(true);

    try {
      console.log('Sending query with params:', {
        datasource_id: datasource.id,
        question: userMessage.content,
        llm_model_id: llmModel,
      });

      const response = await nl2sqlApi.query({
        datasource_id: datasource.id,
        question: userMessage.content,
        session_id: undefined,
        llm_model_id: llmModel,
      });

      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: response.explanation || '查询完成',
        sql: response.sql,
        result: response.result,
        explanation: response.explanation,
        error: response.error,
        timestamp: new Date(),
      };

      setMessages((prev) => [...prev, assistantMessage]);

      if (response.error) {
        showError(response.error);
      }
    } catch (error: any) {
      console.error('Query failed:', error);
      console.error('Error details:', {
        message: error.message,
        response: error.response,
        status: error.response?.status,
        data: error.response?.data,
      });
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: '查询失败',
        error: error.response?.data?.message || error.message || '未知错误',
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, errorMessage]);
      showError('查询失败: ' + (error.response?.data?.message || error.message));
    } finally {
      setLoading(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const exportToCSV = (result: { columns: string[]; data: any[] }) => {
    const csvContent = [
      result.columns.join(','),
      ...result.data.map((row) =>
        result.columns.map((col) => {
          const value = row[col];
          // 处理包含逗号或引号的值
          if (typeof value === 'string' && (value.includes(',') || value.includes('"'))) {
            return `"${value.replace(/"/g, '""')}"`;
          }
          return value ?? '';
        }).join(',')
      ),
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `nl2sql_result_${Date.now()}.csv`;
    link.click();
    showSuccess('导出成功');
  };

  const clearChat = () => {
    setMessages([]);
    showSuccess('对话已清空');
  };

  return (
    <div className="h-full flex flex-col bg-white">
      {/* Header */}
      <div className="border-b px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Database className="w-5 h-5 text-blue-500" />
            <div>
              <h2 className="text-lg font-semibold text-gray-900">测试对话</h2>
              <p className="text-sm text-gray-600">使用自然语言查询数据源</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* Model Selector */}
            <div className="flex items-center gap-2">
              <label className="text-sm text-gray-600">LLM模型:</label>
              <select
                value={llmModel}
                onChange={(e) => setLlmModel(e.target.value)}
                className="px-3 py-1.5 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
              >
                {models.length === 0 ? (
                  <option value="">没有llm模型</option>
                ) : (
                  models.map((model) => (
                    <option key={model.model_id} value={model.model_id}>
                      {model.name}
                    </option>
                  ))
                )}
              </select>
            </div>

            {/* Clear Button */}
            {messages.length > 0 && (
              <button
                onClick={clearChat}
                className="px-3 py-1.5 text-sm text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors"
              >
                清空对话
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
        {messages.length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center text-center">
            <Database className="w-16 h-16 text-gray-300 mb-4" />
            <h3 className="text-lg font-medium text-gray-900 mb-2">开始测试对话</h3>
            <p className="text-sm text-gray-600 max-w-md">
              输入自然语言问题，系统将自动生成SQL查询并执行。
              <br />
              例如："查询所有用户"、"统计订单总数"等
            </p>
          </div>
        ) : (
          messages.map((message) => (
            <div
              key={message.id}
              className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-3xl rounded-lg p-4 ${
                  message.role === 'user'
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 text-gray-900'
                }`}
              >
                {/* User Message */}
                {message.role === 'user' && (
                  <div>
                    <p className="whitespace-pre-wrap">{message.content}</p>
                    <p className="text-xs opacity-75 mt-2">
                      {message.timestamp.toLocaleTimeString()}
                    </p>
                  </div>
                )}

                {/* Assistant Message */}
                {message.role === 'assistant' && (
                  <div className="space-y-3">
                    {/* Error */}
                    {message.error && (
                      <div className="flex items-start gap-2 p-3 bg-red-50 border border-red-200 rounded-lg">
                        <AlertCircle className="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5" />
                        <div className="flex-1">
                          <p className="text-sm font-medium text-red-900">查询失败</p>
                          <p className="text-sm text-red-700 mt-1">{message.error}</p>
                        </div>
                      </div>
                    )}

                    {/* SQL */}
                    {message.sql && (
                      <div className="space-y-2">
                        <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
                          <Code className="w-4 h-4" />
                          生成的SQL
                        </div>
                        <pre className="bg-gray-900 text-gray-100 p-3 rounded-lg overflow-x-auto text-sm">
                          <code>{message.sql}</code>
                        </pre>
                      </div>
                    )}

                    {/* Explanation */}
                    {message.explanation && (
                      <div className="space-y-2">
                        <p className="text-sm text-gray-700">{message.explanation}</p>
                      </div>
                    )}

                    {/* Result Table */}
                    {message.result && message.result.data.length > 0 && (
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
                            <TableIcon className="w-4 h-4" />
                            查询结果 ({message.result.row_count} 行)
                          </div>
                          <button
                            onClick={() => exportToCSV(message.result!)}
                            className="flex items-center gap-1 px-2 py-1 text-xs text-gray-600 hover:text-gray-900 hover:bg-gray-200 rounded transition-colors"
                          >
                            <Download className="w-3 h-3" />
                            导出CSV
                          </button>
                        </div>
                        <div className="overflow-x-auto border border-gray-200 rounded-lg">
                          <table className="min-w-full divide-y divide-gray-200">
                            <thead className="bg-gray-50">
                              <tr>
                                {message.result.columns.map((col) => (
                                  <th
                                    key={col}
                                    className="px-4 py-2 text-left text-xs font-medium text-gray-700 uppercase tracking-wider"
                                  >
                                    {col}
                                  </th>
                                ))}
                              </tr>
                            </thead>
                            <tbody className="bg-white divide-y divide-gray-200">
                              {message.result.data.slice(0, 10).map((row, idx) => (
                                <tr key={idx} className="hover:bg-gray-50">
                                  {message.result!.columns.map((col) => (
                                    <td
                                      key={col}
                                      className="px-4 py-2 text-sm text-gray-900 whitespace-nowrap"
                                    >
                                      {row[col] !== null && row[col] !== undefined
                                        ? String(row[col])
                                        : '-'}
                                    </td>
                                  ))}
                                </tr>
                              ))}
                            </tbody>
                          </table>
                          {message.result.data.length > 10 && (
                            <div className="px-4 py-2 text-xs text-gray-600 bg-gray-50 border-t">
                              显示前 10 行，共 {message.result.row_count} 行
                            </div>
                          )}
                        </div>
                      </div>
                    )}

                    {/* No Results */}
                    {message.result && message.result.data.length === 0 && !message.error && (
                      <div className="text-sm text-gray-600">查询成功，但没有返回数据</div>
                    )}

                    <p className="text-xs text-gray-500 mt-2">
                      {message.timestamp.toLocaleTimeString()}
                    </p>
                  </div>
                )}
              </div>
            </div>
          ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t px-6 py-4 bg-white">
        <div className="flex gap-3">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyPress={handleKeyPress}
            placeholder="输入自然语言问题，例如：查询所有用户..."
            className="flex-1 px-4 py-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
            rows={2}
            disabled={loading}
          />
          <button
            onClick={handleSend}
            disabled={loading || !input.trim() || !llmModel}
            className="px-6 py-3 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center gap-2"
          >
            {loading ? (
              <>
                <Loader2 className="w-5 h-5 animate-spin" />
                查询中...
              </>
            ) : (
              <>
                <Send className="w-5 h-5" />
                发送
              </>
            )}
          </button>
        </div>
        <p className="text-xs text-gray-500 mt-2">
          按 Enter 发送，Shift + Enter 换行
        </p>
      </div>
    </div>
  );
}
