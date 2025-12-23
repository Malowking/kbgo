import { useState, useEffect, useRef, useCallback } from 'react';
import { Send, Paperclip, Plus, List, Settings, ChevronDown } from 'lucide-react';
import { chatApi, conversationApi, knowledgeBaseApi, modelApi, mcpApi } from '@/services';
import { generateId } from '@/lib/utils';
import type { Message, KnowledgeBase, Model, MCPRegistry } from '@/types';
import ChatMessage from './ChatMessage';
import ConversationSidebar from './ConversationSidebar';
import ModelSelectorModal from '@/components/ModelSelectorModal';
import { logger } from '@/lib/logger';
import { showError, showWarning } from '@/lib/toast';
import { CHAT_CONFIG } from '@/config/constants';
import { getLLMModels, getRerankModels } from '@/lib/model-utils';

export default function Chat() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [currentConvId, setCurrentConvId] = useState<string>('');
  const [kbList, setKbList] = useState<KnowledgeBase[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [rerankModels, setRerankModels] = useState<Model[]>([]);
  const [showSidebar, setShowSidebar] = useState(true);
  const [attachedFiles, setAttachedFiles] = useState<File[]>([]);
  const [showAdvancedSettings, setShowAdvancedSettings] = useState(false);
  const [showModelSelector, setShowModelSelector] = useState(false);

  // Settings
  const [selectedKB, setSelectedKB] = useState<string>('');
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [selectedRerankModel, setSelectedRerankModel] = useState<string>('');
  const [enableRetriever, setEnableRetriever] = useState(false);
  const [topK, setTopK] = useState<number>(CHAT_CONFIG.DEFAULT_TOP_K);
  const [score, setScore] = useState<number>(CHAT_CONFIG.DEFAULT_SCORE);
  const [retrieveMode, setRetrieveMode] = useState<'milvus' | 'rerank' | 'rrf'>(CHAT_CONFIG.DEFAULT_RETRIEVE_MODE);
  const [rerankWeight, setRerankWeight] = useState<number>(1.0);
  const [useMCP, setUseMCP] = useState(false);
  const [mcpServices, setMcpServices] = useState<MCPRegistry[]>([]);
  const [selectedMCPService, setSelectedMCPService] = useState<string>('');

  // 当选择知识库时，自动启用检索并展开高级设置
  useEffect(() => {
    if (selectedKB) {
      setEnableRetriever(true);
      setShowAdvancedSettings(true);
    } else {
      setEnableRetriever(false);
    }
  }, [selectedKB]);

  // 当选择MCP服务时，自动启用MCP并展开高级设置
  useEffect(() => {
    if (selectedMCPService) {
      setUseMCP(true);
      setShowAdvancedSettings(true);
    } else {
      setUseMCP(false);
    }
  }, [selectedMCPService]);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchKBList();
    fetchModels();
    fetchMCPServices();
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const fetchKBList = useCallback(async () => {
    try {
      const response = await knowledgeBaseApi.list();
      setKbList(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch knowledge bases:', error);
    }
  }, []);

  const fetchModels = useCallback(async () => {
    try {
      const response = await modelApi.list();

      // 使用工具函数获取 LLM 和多模态模型（仅显示启用的模型）
      const llmAndMultimodalModels = getLLMModels(response.models || [], true);
      setModels(llmAndMultimodalModels);
      if (llmAndMultimodalModels.length > 0 && !selectedModel) {
        setSelectedModel(llmAndMultimodalModels[0].id || llmAndMultimodalModels[0].model_id);
      }

      // 使用工具函数获取 Rerank 模型（仅显示启用的模型）
      const rerankModelsList = getRerankModels(response.models || [], true);
      setRerankModels(rerankModelsList);
      if (rerankModelsList.length > 0 && !selectedRerankModel) {
        setSelectedRerankModel(rerankModelsList[0].id || rerankModelsList[0].model_id);
      }
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    }
  }, [selectedModel, selectedRerankModel]);

  const fetchMCPServices = useCallback(async () => {
    try {
      const response = await mcpApi.list({ status: 1 }); // 只获取启用的服务
      setMcpServices(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch MCP services:', error);
    }
  }, []);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const handleNewConversation = () => {
    setMessages([]);
    setCurrentConvId(generateId());
  };

  const handleSend = useCallback(async () => {
    if (!input.trim() || !selectedModel) {
      showWarning('请输入消息并选择模型');
      return;
    }

    const convId = currentConvId || generateId();
    if (!currentConvId) {
      setCurrentConvId(convId);
    }

    // 使用时间戳 + 随机数确保唯一性
    const userMessageId = Date.now() * 1000 + Math.floor(Math.random() * 1000);
    const assistantMessageId = userMessageId + 1;

    const userMessage: Message = {
      id: userMessageId,
      role: 'user',
      content: input,
      create_time: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, userMessage]);
    const currentInput = input; // 保存输入内容
    const currentFiles = attachedFiles; // 保存附件
    setInput('');
    setAttachedFiles([]); // 清空附件列表
    setLoading(true);

    try {
      // 默认使用流式响应
      let accumulatedContent = '';
      let accumulatedReasoning = '';
      let receivedReferences: any[] | undefined;

      setMessages((prev) => [
        ...prev,
        {
          id: assistantMessageId,
          role: 'assistant',
          content: '',
          reasoning_content: '',
          create_time: new Date().toISOString(),
        },
      ]);

      await chatApi.sendStream(
        {
          conv_id: convId,
          question: currentInput,
          model_id: selectedModel,
          rerank_model_id: selectedRerankModel,
          knowledge_id: selectedKB,
          enable_retriever: enableRetriever && !!selectedKB,
          top_k: topK,
          score: score,
          retrieve_mode: retrieveMode,
          rerank_weight: rerankWeight,
          use_mcp: useMCP && !!selectedMCPService,
          stream: true,
          files: currentFiles, // 传递文件
        },
        (chunk, reasoningChunk, references) => {
          // 累积内容和思考过程
          accumulatedContent += chunk;
          if (reasoningChunk) {
            accumulatedReasoning += reasoningChunk;
          }
          // 保存references（通常在最后一条消息中返回）
          if (references) {
            receivedReferences = references;
          }

          setMessages((prev) => {
            const newMessages = [...prev];
            const lastIndex = newMessages.length - 1;
            if (newMessages[lastIndex]?.id === assistantMessageId) {
              newMessages[lastIndex] = {
                ...newMessages[lastIndex],
                content: accumulatedContent,
                reasoning_content: accumulatedReasoning || undefined,
                references: receivedReferences,
              };
            }
            return newMessages;
          });
        },
        (error) => {
          logger.error('Stream error:', error);
          showError('发送失败: ' + error.message);
          setLoading(false);
        }
      );
    } catch (error) {
      logger.error('Failed to send message:', error);
      showError('发送失败: ' + (error as Error).message);
    } finally {
      setLoading(false);
    }
  }, [input, selectedModel, currentConvId, attachedFiles, selectedRerankModel, selectedKB, enableRetriever, topK, score, retrieveMode, rerankWeight, useMCP, selectedMCPService]);

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleFileClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files.length > 0) {
      setAttachedFiles(Array.from(files));
    }
  };

  const handleRemoveFile = (index: number) => {
    setAttachedFiles(prev => prev.filter((_, i) => i !== index));
  };

  const handleLoadConversation = useCallback(async (convId: string) => {
    try {
      setLoading(true);
      const conversation = await conversationApi.get(convId);
      setCurrentConvId(convId);
      setMessages(conversation.messages || []);
    } catch (error) {
      logger.error('Failed to load conversation:', error);
      showError('加载对话失败: ' + (error as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  const handleDeleteConversation = useCallback(async (convId: string) => {
    try {
      await conversationApi.delete(convId);
      if (convId === currentConvId) {
        handleNewConversation();
      }
    } catch (error) {
      logger.error('Failed to delete conversation:', error);
    }
  }, [currentConvId]);

  const handleModelSelect = (model: Model) => {
    setSelectedModel(model.id || model.model_id);
  };

  const getSelectedModelName = (): string => {
    const model = models.find(m => (m.id || m.model_id) === selectedModel);
    return model ? model.name : '选择模型';
  };

  return (
    <div className="flex h-screen bg-gray-50 gap-6 p-6">
      {/* Conversation Sidebar */}
      {showSidebar && (
        <ConversationSidebar
          currentConvId={currentConvId}
          onLoadConversation={handleLoadConversation}
          onDeleteConversation={handleDeleteConversation}
          onNewConversation={handleNewConversation}
        />
      )}

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col bg-white rounded-lg shadow-sm border border-gray-200">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setShowSidebar(!showSidebar)}
              className="p-2 rounded hover:bg-gray-100"
            >
              <List className="w-5 h-5 text-gray-600" />
            </button>
            <h2 className="text-lg font-semibold text-gray-900">
              {currentConvId ? '对话中' : '新对话'}
            </h2>
          </div>

          <button
            onClick={handleNewConversation}
            className="btn btn-secondary flex items-center text-sm"
          >
            <Plus className="w-4 h-4 mr-2" />
            新对话
          </button>
        </div>

        {/* Settings Bar */}
        <div className="border-b border-gray-200 bg-gray-50">
          <div className="px-6 py-3 flex items-center justify-between space-x-4">
            <div className="flex-1">
              <button
                onClick={() => setShowModelSelector(true)}
                className="w-full flex items-center justify-between px-4 py-2 border rounded-lg bg-white hover:bg-gray-50 transition-colors text-left"
              >
                <span className="text-sm text-gray-900">{getSelectedModelName()}</span>
                <ChevronDown className="w-4 h-4 text-gray-500" />
              </button>
            </div>

            <button
              onClick={() => setShowAdvancedSettings(!showAdvancedSettings)}
              className="flex items-center space-x-2 px-4 py-2 border rounded-lg bg-white hover:bg-gray-50 transition-colors text-sm"
              title="高级设置"
            >
              <Settings className="w-4 h-4 text-gray-600" />
              <span className="text-gray-700">高级设置</span>
            </button>
          </div>

          {/* Advanced Settings */}
          {showAdvancedSettings && (
            <div className="px-6 py-4 border-t border-gray-200 bg-white space-y-6">
              {/* 知识库配置 */}
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-3">知识库配置</h3>
                <div>
                  <label className="block text-xs text-gray-600 mb-1">选择知识库</label>
                  <select
                    value={selectedKB}
                    onChange={(e) => setSelectedKB(e.target.value)}
                    className="input text-sm"
                  >
                    <option value="">不使用知识库</option>
                    {kbList.map((kb) => (
                      <option key={kb.id} value={kb.id}>
                        {kb.name}
                      </option>
                    ))}
                  </select>
                  <p className="text-xs text-gray-500 mt-1">选择知识库后将自动启用知识检索</p>
                </div>
              </div>

              {/* 检索参数配置 - 只在启用检索时显示 */}
              {enableRetriever && selectedKB && (
                <div>
                  <h3 className="text-sm font-medium text-gray-700 mb-3">检索参数配置</h3>

                  {/* 模型选择行 */}
                  <div className="mb-4">
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Rerank 模型</label>
                      <select
                        value={selectedRerankModel}
                        onChange={(e) => setSelectedRerankModel(e.target.value)}
                        className="input text-sm"
                        disabled={retrieveMode === 'milvus'}
                      >
                        {rerankModels.length === 0 && (
                          <option value="">无可用的 Rerank 模型</option>
                        )}
                        {rerankModels.map((model) => (
                          <option key={model.id} value={model.id}>
                            {model.name}
                          </option>
                        ))}
                      </select>
                      <p className="text-xs text-gray-500 mt-1">用于结果重排序（Milvus模式不需要，Embedding 模型自动使用知识库绑定的模型）</p>
                    </div>
                  </div>

                  {/* 参数配置行 */}
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Top K</label>
                      <input
                        type="number"
                        value={topK}
                        onChange={(e) => setTopK(Number(e.target.value))}
                        className="input text-sm"
                        min="1"
                        max="20"
                      />
                      <p className="text-xs text-gray-500 mt-1">返回文档数量（默认5）</p>
                    </div>

                    <div>
                      <label className="block text-xs text-gray-600 mb-1">相似度分数</label>
                      <input
                        type="number"
                        value={score}
                        onChange={(e) => setScore(Number(e.target.value))}
                        className="input text-sm"
                        min="0"
                        max="1"
                        step="0.1"
                      />
                      <p className="text-xs text-gray-500 mt-1">默认0.2（RRF模式时不重要）</p>
                    </div>

                    <div>
                      <label className="block text-xs text-gray-600 mb-1">检索模式</label>
                      <select
                        value={retrieveMode}
                        onChange={(e) => setRetrieveMode(e.target.value as 'milvus' | 'rerank' | 'rrf')}
                        className="input text-sm"
                      >
                        <option value="rrf">RRF（推荐）</option>
                        <option value="rerank">Rerank</option>
                        <option value="milvus">Milvus</option>
                      </select>
                      <p className="text-xs text-gray-500 mt-1">检索策略选择</p>
                    </div>
                  </div>

                  {/* Rerank权重配置 - 只在rerank模式下显示 */}
                  {retrieveMode === 'rerank' && (
                    <div className="mt-4 pt-4 border-t border-gray-100">
                      <label className="block text-xs text-gray-600 mb-2">
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
                </div>
              )}

              {/* MCP 配置 */}
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-3">MCP 服务配置</h3>
                <div>
                  <label className="block text-xs text-gray-600 mb-1">选择 MCP 服务</label>
                  <select
                    value={selectedMCPService}
                    onChange={(e) => setSelectedMCPService(e.target.value)}
                    className="input text-sm"
                  >
                    <option value="">不使用 MCP 服务</option>
                    {mcpServices.map((service) => (
                      <option key={service.id} value={service.id}>
                        {service.name} - {service.description || '无描述'}
                      </option>
                    ))}
                  </select>
                  <p className="text-xs text-gray-500 mt-1">选择 MCP 服务后将自动启用</p>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6 messages-container">
          {messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
              <p className="text-lg">开始新的对话</p>
              <p className="text-sm mt-2">选择模型和知识库，然后发送消息</p>
            </div>
          ) : (
            messages.map((message, index) => (
              <ChatMessage
                key={message.id}
                message={message}
                isStreaming={loading && index === messages.length - 1}
              />
            ))
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Input Area */}
        <div className="px-6 py-4 border-t border-gray-200">
          {/* Attached Files Preview */}
          {attachedFiles.length > 0 && (
            <div className="mb-3 flex flex-wrap gap-2">
              {attachedFiles.map((file, index) => (
                <div
                  key={index}
                  className="flex items-center gap-2 px-3 py-2 bg-blue-50 border border-blue-200 rounded-lg text-sm"
                >
                  <Paperclip className="w-4 h-4 text-blue-600" />
                  <span className="text-blue-900">{file.name}</span>
                  <button
                    onClick={() => handleRemoveFile(index)}
                    className="ml-2 text-blue-600 hover:text-blue-800"
                  >
                    ×
                  </button>
                </div>
              ))}
            </div>
          )}

          <div className="flex items-end space-x-3">
            {/* Hidden File Input */}
            <input
              ref={fileInputRef}
              type="file"
              multiple
              onChange={handleFileChange}
              className="hidden"
              accept=".pdf,.doc,.docx,.txt,.md,.jpg,.jpeg,.png"
            />

            <button
              onClick={handleFileClick}
              className="p-2 rounded hover:bg-gray-100 text-gray-600"
              title="附件"
              disabled={loading}
            >
              <Paperclip className="w-5 h-5" />
            </button>

            <div className="flex-1">
              <textarea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyPress={handleKeyPress}
                placeholder="输入消息... (Shift+Enter 换行，Enter 发送)"
                className="input resize-none"
                rows={3}
                disabled={loading}
              />
            </div>

            <button
              onClick={handleSend}
              disabled={loading || !input.trim() || !selectedModel}
              className="btn btn-primary h-[88px]"
            >
              {loading ? (
                <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
              ) : (
                <Send className="w-5 h-5" />
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Model Selector Modal */}
      {showModelSelector && (
        <ModelSelectorModal
          onClose={() => setShowModelSelector(false)}
          onSelect={handleModelSelect}
          currentModelId={selectedModel}
          modelTypes={['llm', 'multimodal']}
        />
      )}
    </div>
  );
}
