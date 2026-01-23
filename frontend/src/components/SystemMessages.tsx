import { useState, useEffect } from 'react';
import { Bell, AlertCircle, X } from 'lucide-react';
import { modelApi } from '@/services';

interface SystemMessage {
  id: string;
  type: 'warning' | 'error' | 'info';
  title: string;
  message: string;
}

export default function SystemMessages() {
  const [messages, setMessages] = useState<SystemMessage[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    checkSystemStatus();
  }, []);

  const checkSystemStatus = async () => {
    try {
      setLoading(true);
      const response = await modelApi.list();
      const models = response.models || [];

      const newMessages: SystemMessage[] = [];

      // 检查是否有LLM模型
      const llmModels = models.filter(m => m.type === 'llm' && m.enabled);
      if (llmModels.length === 0) {
        newMessages.push({
          id: 'no-llm',
          type: 'error',
          title: '缺少LLM模型',
          message: '系统未配置LLM模型，对话功能将无法使用。请前往模型管理页面添加LLM模型。'
        });
      }

      // 检查是否有embedding模型
      const embeddingModels = models.filter(m => m.type === 'embedding' && m.enabled);
      if (embeddingModels.length === 0) {
        newMessages.push({
          id: 'no-embedding',
          type: 'warning',
          title: '缺少Embedding模型',
          message: '系统未配置Embedding模型，知识库功能将无法正常使用。请前往模型管理页面添加Embedding模型。'
        });
      }

      // 检查是否有reranker模型
      const rerankerModels = models.filter(m => (m.type === 'reranker' || m.type === 'rerank') && m.enabled);
      if (rerankerModels.length === 0) {
        newMessages.push({
          id: 'no-reranker',
          type: 'warning',
          title: '缺少Reranker模型',
          message: '系统未配置Reranker模型，检索重排序功能将无法使用。建议添加Reranker模型以提升检索准确度。'
        });
      }

      // 检查是否有重写模型配置
      const rewriteResponse = await modelApi.getRewriteModel();
      if (!rewriteResponse.configured) {
        newMessages.push({
          id: 'no-rewrite',
          type: 'info',
          title: '未配置查询重写模型',
          message: '系统未配置查询重写模型，多轮对话中的指代消解功能将无法使用。建议在模型管理页面配置一个LLM作为重写模型。'
        });
      }

      setMessages(newMessages);
    } catch (error) {
      console.error('Failed to check system status:', error);
    } finally {
      setLoading(false);
    }
  };

  const dismissMessage = (id: string) => {
    setMessages(messages.filter(m => m.id !== id));
  };

  const messageCount = messages.length;

  return (
    <div className="relative">
      {/* Message Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="relative p-2 rounded-lg hover:bg-gray-100 transition-colors"
        title="系统消息"
      >
        <Bell className="w-5 h-5 text-gray-600" />
        {messageCount > 0 && (
          <span className="absolute top-0 right-0 w-5 h-5 bg-red-500 text-white text-xs font-bold rounded-full flex items-center justify-center">
            {messageCount}
          </span>
        )}
      </button>

      {/* Messages Dropdown */}
      {isOpen && (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => setIsOpen(false)}
          />
          <div className="absolute right-0 mt-2 w-96 bg-white rounded-lg shadow-lg border border-gray-200 z-50 max-h-[600px] overflow-y-auto">
            <div className="p-4 border-b border-gray-200">
              <div className="flex items-center justify-between">
                <h3 className="text-lg font-semibold text-gray-900">系统消息</h3>
                <button
                  onClick={() => setIsOpen(false)}
                  className="p-1 rounded hover:bg-gray-100"
                >
                  <X className="w-4 h-4 text-gray-500" />
                </button>
              </div>
            </div>

            <div className="divide-y divide-gray-200">
              {loading ? (
                <div className="p-8 text-center">
                  <div className="inline-block w-6 h-6 border-4 border-primary-600 border-t-transparent rounded-full animate-spin"></div>
                  <p className="mt-2 text-sm text-gray-600">检查中...</p>
                </div>
              ) : messageCount === 0 ? (
                <div className="p-8 text-center">
                  <Bell className="w-12 h-12 text-gray-300 mx-auto mb-3" />
                  <p className="text-sm text-gray-500">没有系统消息</p>
                </div>
              ) : (
                messages.map((message) => (
                  <div
                    key={message.id}
                    className={`p-4 ${
                      message.type === 'warning'
                        ? 'bg-yellow-50'
                        : message.type === 'error'
                        ? 'bg-red-50'
                        : 'bg-blue-50'
                    }`}
                  >
                    <div className="flex items-start space-x-3">
                      <AlertCircle
                        className={`w-5 h-5 mt-0.5 flex-shrink-0 ${
                          message.type === 'warning'
                            ? 'text-yellow-600'
                            : message.type === 'error'
                            ? 'text-red-600'
                            : 'text-blue-600'
                        }`}
                      />
                      <div className="flex-1 min-w-0">
                        <h4 className="text-sm font-semibold text-gray-900 mb-1">
                          {message.title}
                        </h4>
                        <p className="text-sm text-gray-700">{message.message}</p>
                      </div>
                      <button
                        onClick={() => dismissMessage(message.id)}
                        className="p-1 rounded hover:bg-white/50 flex-shrink-0"
                        title="关闭"
                      >
                        <X className="w-4 h-4 text-gray-500" />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>

            {messageCount > 0 && (
              <div className="p-4 border-t border-gray-200 bg-gray-50">
                <button
                  onClick={checkSystemStatus}
                  className="w-full text-sm text-primary-600 hover:text-primary-700 font-medium"
                >
                  重新检查
                </button>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}