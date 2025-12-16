import { useState, useEffect, useRef } from 'react';
import { Send, Paperclip, Plus, List } from 'lucide-react';
import { chatApi, conversationApi, knowledgeBaseApi, modelApi } from '@/services';
import { generateId } from '@/lib/utils';
import type { Message, KnowledgeBase, Model } from '@/types';
import ChatMessage from './ChatMessage';
import ConversationSidebar from './ConversationSidebar';

export default function Chat() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [currentConvId, setCurrentConvId] = useState<string>('');
  const [kbList, setKbList] = useState<KnowledgeBase[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [showSidebar, setShowSidebar] = useState(true);

  // Settings
  const [selectedKB, setSelectedKB] = useState<string>('');
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [enableRetriever, setEnableRetriever] = useState(true);
  const [stream, setStream] = useState(true);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetchKBList();
    fetchModels();
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const fetchKBList = async () => {
    try {
      const response = await knowledgeBaseApi.list();
      setKbList(response.list || []);
    } catch (error) {
      console.error('Failed to fetch knowledge bases:', error);
    }
  };

  const fetchModels = async () => {
    try {
      const response = await modelApi.list();
      const llmModels = response.models?.filter(m => m.type === 'llm').map(m => ({
        ...m,
        id: m.model_id,
      })) || [];
      setModels(llmModels as Model[]);
      if (llmModels.length > 0 && !selectedModel) {
        setSelectedModel(llmModels[0].id || llmModels[0].model_id);
      }
    } catch (error) {
      console.error('Failed to fetch models:', error);
    }
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const handleNewConversation = () => {
    setMessages([]);
    setCurrentConvId(generateId());
  };

  const handleSend = async () => {
    if (!input.trim() || !selectedModel) {
      alert('请输入消息并选择模型');
      return;
    }

    const convId = currentConvId || generateId();
    if (!currentConvId) {
      setCurrentConvId(convId);
    }

    const userMessage: Message = {
      id: Date.now(),
      role: 'user',
      content: input,
      create_time: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, userMessage]);
    const currentInput = input; // 保存输入内容
    setInput('');
    setLoading(true);

    try {
      if (stream) {
        // 流式响应 - 使用累积内容避免状态更新问题
        const assistantMessageId = Date.now() + 1;
        let accumulatedContent = '';

        setMessages((prev) => [
          ...prev,
          {
            id: assistantMessageId,
            role: 'assistant',
            content: '',
            create_time: new Date().toISOString(),
          },
        ]);

        await chatApi.sendStream(
          {
            conv_id: convId,
            question: currentInput,
            model_id: selectedModel,
            knowledge_id: selectedKB,
            enable_retriever: enableRetriever && !!selectedKB,
            stream: true,
          },
          (chunk) => {
            accumulatedContent += chunk;
            setMessages((prev) => {
              const newMessages = [...prev];
              const lastIndex = newMessages.length - 1;
              if (newMessages[lastIndex]?.id === assistantMessageId) {
                newMessages[lastIndex] = {
                  ...newMessages[lastIndex],
                  content: accumulatedContent,
                };
              }
              return newMessages;
            });
          },
          (error) => {
            console.error('Stream error:', error);
            alert('发送失败: ' + error.message);
            setLoading(false);
          }
        );
      } else {
        // 非流式响应
        const response = await chatApi.send({
          conv_id: convId,
          question: currentInput,
          model_id: selectedModel,
          knowledge_id: selectedKB,
          enable_retriever: enableRetriever && !!selectedKB,
          stream: false,
        });

        const assistantMessage: Message = {
          id: Date.now() + 1,
          role: 'assistant',
          content: response.answer,
          reasoning_content: response.reasoning_content,
          create_time: new Date().toISOString(),
        };

        setMessages((prev) => [...prev, assistantMessage]);
      }
    } catch (error) {
      console.error('Failed to send message:', error);
      alert('发送失败: ' + (error as Error).message);
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

  const handleLoadConversation = async (convId: string) => {
    try {
      setLoading(true);
      const conversation = await conversationApi.get(convId);
      setCurrentConvId(convId);
      setMessages(conversation.messages || []);
    } catch (error) {
      console.error('Failed to load conversation:', error);
      alert('加载对话失败: ' + (error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteConversation = async (convId: string) => {
    try {
      await conversationApi.delete(convId);
      if (convId === currentConvId) {
        handleNewConversation();
      }
    } catch (error) {
      console.error('Failed to delete conversation:', error);
    }
  };

  return (
    <div className="flex h-[calc(100vh-120px)] gap-6">
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
        <div className="px-6 py-3 border-b border-gray-200 bg-gray-50">
          <div className="flex items-center space-x-4">
            <div className="flex-1">
              <select
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
                className="input text-sm"
              >
                <option value="">选择模型</option>
                {models.map((model) => (
                  <option key={model.id} value={model.id}>
                    {model.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="flex-1">
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
            </div>

            <label className="flex items-center space-x-2 text-sm">
              <input
                type="checkbox"
                checked={enableRetriever}
                onChange={(e) => setEnableRetriever(e.target.checked)}
                disabled={!selectedKB}
                className="rounded border-gray-300 text-primary-600"
              />
              <span className="text-gray-700">启用检索</span>
            </label>

            <label className="flex items-center space-x-2 text-sm">
              <input
                type="checkbox"
                checked={stream}
                onChange={(e) => setStream(e.target.checked)}
                className="rounded border-gray-300 text-primary-600"
              />
              <span className="text-gray-700">流式输出</span>
            </label>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
              <p className="text-lg">开始新的对话</p>
              <p className="text-sm mt-2">选择模型和知识库，然后发送消息</p>
            </div>
          ) : (
            messages.map((message) => (
              <ChatMessage key={message.id} message={message} />
            ))
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Input Area */}
        <div className="px-6 py-4 border-t border-gray-200">
          <div className="flex items-end space-x-3">
            <button
              className="p-2 rounded hover:bg-gray-100 text-gray-600"
              title="附件"
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
    </div>
  );
}