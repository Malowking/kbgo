import React, { useState, useEffect, useRef } from 'react';
import { Send, Bot, User, ArrowLeft, Plus, Loader2, Paperclip } from 'lucide-react';
import { agentApi } from '@/services';
import { generateId } from '@/lib/utils';
import type { AgentPresetItem } from '@/types';
import { useNavigate } from 'react-router-dom';
import MessageContent from '@/components/MessageContent';

interface Message {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  reasoning_content?: string;
  references?: any[];
  mcp_results?: any[];
  timestamp: string;
}

interface Conversation {
  conv_id: string;
  preset_id: string;
  preset_name: string;
  messages: Message[];
}

export default function AgentChat() {
  const navigate = useNavigate();
  const [presets, setPresets] = useState<AgentPresetItem[]>([]);
  const [selectedPreset, setSelectedPreset] = useState<string>('');
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [currentConvId, setCurrentConvId] = useState<string>('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [attachedFiles, setAttachedFiles] = useState<File[]>([]);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const USER_ID = 'user_001'; // TODO: Get from auth context

  useEffect(() => {
    fetchPresets();
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // 当选择的 Agent 变化时，清空对话历史
  useEffect(() => {
    if (selectedPreset) {
      setConversations([]);
      setMessages([]);
      setCurrentConvId('');
    }
  }, [selectedPreset]);

  const fetchPresets = async () => {
    try {
      const response = await agentApi.list({ user_id: USER_ID, page: 1, page_size: 100 });
      setPresets(response.list || []);

      // Auto-select first preset if available
      if (response.list && response.list.length > 0 && !selectedPreset) {
        setSelectedPreset(response.list[0].preset_id);
      }
    } catch (error) {
      console.error('Failed to fetch agent presets:', error);
    }
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const handleSelectPreset = (presetId: string) => {
    setSelectedPreset(presetId);
    setCurrentConvId('');
    setMessages([]);
  };

  const handleNewConversation = () => {
    if (!selectedPreset) {
      alert('请先选择一个 Agent');
      return;
    }

    const newConvId = generateId();
    const preset = presets.find(p => p.preset_id === selectedPreset);

    const newConv: Conversation = {
      conv_id: newConvId,
      preset_id: selectedPreset,
      preset_name: preset?.preset_name || 'Agent',
      messages: [],
    };

    setConversations(prev => [newConv, ...prev]);
    setCurrentConvId(newConvId);
    setMessages([]);
  };

  const handleSend = async () => {
    if (!input.trim() || !selectedPreset) {
      alert('请输入消息并选择 Agent');
      return;
    }

    const convId = currentConvId || generateId();
    if (!currentConvId) {
      setCurrentConvId(convId);

      // Create conversation entry
      const preset = presets.find(p => p.preset_id === selectedPreset);
      const newConv: Conversation = {
        conv_id: convId,
        preset_id: selectedPreset,
        preset_name: preset?.preset_name || 'Agent',
        messages: [],
      };
      setConversations(prev => [newConv, ...prev]);
    }

    // 使用时间戳 + 随机数确保唯一性
    const userMessageId = Date.now() * 1000 + Math.floor(Math.random() * 1000);
    const assistantMessageId = userMessageId + 1;

    const userMessage: Message = {
      id: userMessageId,
      role: 'user',
      content: input,
      timestamp: new Date().toISOString(),
    };

    setMessages(prev => [...prev, userMessage]);
    const currentInput = input;
    const currentFiles = attachedFiles;
    setInput('');
    setAttachedFiles([]); // 清空文件列表
    setLoading(true);

    try {
      // 默认使用流式输出
      let accumulatedContent = '';
      let accumulatedReasoning = '';

      setMessages(prev => [
        ...prev,
        {
          id: assistantMessageId,
          role: 'assistant',
          content: '',
          reasoning_content: '',
          timestamp: new Date().toISOString(),
        },
      ]);

      await agentApi.chatStream(
        {
          preset_id: selectedPreset,
          user_id: USER_ID,
          question: currentInput,
          conv_id: convId,
          stream: true,
          files: currentFiles, // 传递文件
        },
        (chunk, reasoningChunk, references) => {
          // 累积内容和思考过程
          accumulatedContent += chunk;
          if (reasoningChunk) {
            accumulatedReasoning += reasoningChunk;
          }

          setMessages(prev =>
            prev.map(msg =>
              msg.id === assistantMessageId
                ? {
                    ...msg,
                    content: accumulatedContent,
                    reasoning_content: accumulatedReasoning || undefined,
                    references: references || msg.references
                  }
                : msg
            )
          );
        },
        (error) => {
          console.error('Stream error:', error);
          setMessages(prev =>
            prev.map(msg =>
              msg.id === assistantMessageId
                ? { ...msg, content: '抱歉，发生了错误：' + error.message }
                : msg
            )
          );
        }
      );
    } catch (error: any) {
      console.error('Failed to send message:', error);
      const errorMessage: Message = {
        id: Date.now() + 1,
        role: 'assistant',
        content: '抱歉，发送消息失败：' + (error.message || '未知错误'),
        timestamp: new Date().toISOString(),
      };
      setMessages(prev => [...prev, errorMessage]);
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
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

  const selectedPresetName = presets.find(p => p.preset_id === selectedPreset)?.preset_name || 'Agent';

  return (
    <div className="h-screen flex bg-gray-50">
      {/* Sidebar */}
      <div className="w-64 border-r bg-white flex flex-col">
        {/* Header */}
        <div className="p-4 border-b">
          <button
            onClick={() => navigate('/agent-builder')}
            className="flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 mb-3"
          >
            <ArrowLeft className="w-4 h-4" />
            返回 Agent 构建器
          </button>
          <button
            onClick={handleNewConversation}
            className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            <Plus className="w-4 h-4" />
            新对话
          </button>
        </div>

        {/* Agent Selection */}
        <div className="p-4 border-b">
          <label className="block text-sm font-medium text-gray-700 mb-2">
            选择 Agent
          </label>
          <select
            value={selectedPreset}
            onChange={(e) => handleSelectPreset(e.target.value)}
            className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm"
          >
            <option value="">请选择</option>
            {presets.map((preset) => (
              <option key={preset.preset_id} value={preset.preset_id}>
                {preset.preset_name}
              </option>
            ))}
          </select>
        </div>

        {/* Conversations List */}
        <div className="flex-1 overflow-auto p-4">
          <h3 className="text-sm font-medium text-gray-700 mb-2">对话历史</h3>
          {conversations.length === 0 ? (
            <p className="text-sm text-gray-500">暂无对话</p>
          ) : (
            <div className="space-y-2">
              {conversations.map((conv) => (
                <button
                  key={conv.conv_id}
                  onClick={() => {
                    setCurrentConvId(conv.conv_id);
                    setMessages(conv.messages);
                    setSelectedPreset(conv.preset_id);
                  }}
                  className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
                    currentConvId === conv.conv_id
                      ? 'bg-blue-50 text-blue-700'
                      : 'hover:bg-gray-50'
                  }`}
                >
                  <div className="font-medium truncate">{conv.preset_name}</div>
                  <div className="text-xs text-gray-500">
                    {conv.messages.length} 条消息
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Chat Area */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <div className="border-b bg-white px-6 py-4">
          <div className="flex items-center gap-3">
            <Bot className="w-6 h-6 text-blue-500" />
            <div>
              <h1 className="text-xl font-bold">{selectedPresetName}</h1>
              {selectedPreset && (
                <p className="text-sm text-gray-500">
                  {presets.find(p => p.preset_id === selectedPreset)?.description || ''}
                </p>
              )}
            </div>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-auto p-6 space-y-4">
          {messages.length === 0 ? (
            <div className="text-center py-12">
              <Bot className="w-16 h-16 text-gray-300 mx-auto mb-4" />
              <p className="text-gray-500 mb-2">开始与 {selectedPresetName} 对话</p>
              <p className="text-sm text-gray-400">在下方输入框中输入您的问题</p>
            </div>
          ) : (
            <>
              {messages.map((message) => (
                <div
                  key={message.id}
                  className={`flex gap-3 ${
                    message.role === 'user' ? 'justify-end' : 'justify-start'
                  }`}
                >
                  {message.role === 'assistant' && (
                    <div className="flex-shrink-0">
                      <div className="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center">
                        <Bot className="w-5 h-5 text-white" />
                      </div>
                    </div>
                  )}

                  <div
                    className={`max-w-2xl rounded-lg px-4 py-3 ${
                      message.role === 'user'
                        ? 'bg-blue-500 text-white'
                        : 'bg-white border'
                    }`}
                  >
                    {message.role === 'user' ? (
                      <div className="whitespace-pre-wrap break-words">{message.content}</div>
                    ) : (
                      <MessageContent
                        content={message.content}
                        reasoningContent={message.reasoning_content}
                        isStreaming={loading && messages[messages.length - 1]?.id === message.id}
                      />
                    )}

                    {message.references && message.references.length > 0 && (() => {
                      // 按文档名称分组片段
                      const docGroups = message.references.reduce((groups: Record<string, any[]>, ref: any) => {
                        const metadata = ref.metadata || {};
                        const documentName = metadata.document_name || '未知文档';
                        if (!groups[documentName]) {
                          groups[documentName] = [];
                        }
                        groups[documentName].push(ref);
                        return groups;
                      }, {});

                      return (
                        <div className="mt-2 p-2 bg-blue-50 border border-blue-200 rounded">
                          <p className="text-xs font-medium text-blue-900 mb-1.5 flex items-center gap-1">
                            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                            知识检索结果 ({message.references.length} 个片段，来自 {Object.keys(docGroups).length} 个文档)
                          </p>
                          <div className="space-y-1">
                            {Object.entries(docGroups).map(([docName, chunks]: [string, any[]], docIdx: number) => {
                              const [isExpanded, setIsExpanded] = React.useState(false);

                              return (
                                <div key={docIdx} className="bg-white border border-blue-100 rounded">
                                  {/* 文档头部 - 可点击展开/收起 */}
                                  <button
                                    onClick={() => setIsExpanded(!isExpanded)}
                                    className="w-full px-2 py-1.5 flex items-center justify-between hover:bg-blue-50 transition-colors text-left"
                                  >
                                    <div className="flex items-center gap-1.5 flex-1 min-w-0">
                                      <svg
                                        className={`w-3 h-3 text-blue-600 flex-shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                      >
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                      </svg>
                                      <span className="text-xs font-medium text-blue-900 truncate">{docName}</span>
                                    </div>
                                    <span className="text-xs text-gray-600 ml-2 flex-shrink-0">{chunks.length} 个片段</span>
                                  </button>

                                  {/* 片段列表 - 展开时显示 */}
                                  {isExpanded && (
                                    <div className="border-t border-blue-100 p-1.5 space-y-1">
                                      {chunks.map((ref: any, chunkIdx: number) => {
                                        const metadata = ref.metadata || {};
                                        const chunkIndex = metadata.chunk_index !== undefined ? metadata.chunk_index : '?';
                                        const score = ref.score ? (ref.score * 100).toFixed(1) : '0';

                                        return (
                                          <div
                                            key={chunkIdx}
                                            className="p-1.5 bg-gray-50 rounded text-xs"
                                          >
                                            <div className="flex items-center justify-between mb-0.5">
                                              <span className="text-gray-700 font-medium text-xs">片段 #{chunkIndex}</span>
                                              <span className="text-blue-600 font-medium text-xs">{score}%</span>
                                            </div>
                                            <p className="text-gray-600 text-xs line-clamp-2 leading-relaxed">{ref.content}</p>
                                          </div>
                                        );
                                      })}
                                    </div>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        </div>
                      );
                    })()}

                    {message.mcp_results && message.mcp_results.length > 0 && (
                      <div className="mt-3 pt-3 border-t border-gray-200">
                        <p className="text-sm font-medium text-gray-700 mb-2">
                          工具调用结果
                        </p>
                        <div className="space-y-2">
                          {message.mcp_results.map((result: any, idx: number) => (
                            <div
                              key={idx}
                              className="text-sm p-2 bg-green-50 rounded border border-green-200"
                            >
                              <p className="font-medium text-green-900">
                                {result.service_name} - {result.tool_name}
                              </p>
                              <p className="text-gray-700 mt-1 line-clamp-3">{result.content}</p>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    <p className="text-xs text-gray-400 mt-2">
                      {new Date(message.timestamp).toLocaleTimeString()}
                    </p>
                  </div>

                  {message.role === 'user' && (
                    <div className="flex-shrink-0">
                      <div className="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center">
                        <User className="w-5 h-5 text-gray-600" />
                      </div>
                    </div>
                  )}
                </div>
              ))}

              {loading && (
                <div className="flex gap-3">
                  <div className="flex-shrink-0">
                    <div className="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center">
                      <Bot className="w-5 h-5 text-white" />
                    </div>
                  </div>
                  <div className="bg-white border rounded-lg px-4 py-3">
                    <Loader2 className="w-5 h-5 animate-spin text-blue-500" />
                  </div>
                </div>
              )}

              <div ref={messagesEndRef} />
            </>
          )}
        </div>

        {/* Input */}
        <div className="border-t bg-white p-4">
          <div className="max-w-4xl mx-auto">
            {!selectedPreset ? (
              <div className="text-center text-gray-500 py-4">
                请先选择一个 Agent
              </div>
            ) : (
              <div>
                {/* File Attachments Preview */}
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

                {/* Input Area */}
                <div className="flex items-end space-x-3">
                  {/* Hidden File Input */}
                  <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    onChange={handleFileChange}
                    className="hidden"
                    accept=".pdf,.doc,.docx,.txt,.md,.jpg,.jpeg,.png,.gif,.bmp,.webp,.mp3,.wav,.mp4,.avi"
                  />

                  {/* File Attach Button */}
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
                      onKeyDown={handleKeyDown}
                      placeholder="输入消息... (Shift+Enter 换行，Enter 发送)"
                      disabled={loading}
                      className="w-full px-4 py-3 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-50 resize-none"
                      rows={3}
                    />
                  </div>

                  <button
                    onClick={handleSend}
                    disabled={loading || !input.trim()}
                    className="px-6 py-3 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                  >
                    {loading ? (
                      <Loader2 className="w-5 h-5 animate-spin" />
                    ) : (
                      <Send className="w-5 h-5" />
                    )}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
