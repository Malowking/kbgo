import { useState, useEffect, useRef, useCallback } from 'react';
import { Send, Bot, User, ArrowLeft, Plus, Loader2, Paperclip, MessageSquare, Trash2 } from 'lucide-react';
import { agentApi, conversationApi } from '@/services';
import { generateId, formatDate, truncate } from '@/lib/utils';
import type { AgentPresetItem } from '@/types';
import { useNavigate } from 'react-router-dom';
import MessageContent from '@/components/MessageContent';
import ReferencesList from '@/components/ReferencesList';
import ToolCallStatus from '@/components/ToolCallStatus';
import ThinkingProcess from '@/components/ThinkingProcess';
import MultiTurnDisplay from '@/components/MultiTurnDisplay';
import { ToolCallInfo, LLMIterationInfo } from '@/lib/sse-client';
import { logger } from '@/lib/logger';
import { showError, showWarning } from '@/lib/toast';
import { USER } from '@/config/constants';

interface Message {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  reasoning_content?: string;
  references?: any[];
  mcp_results?: any[];
  timestamp: string;
  toolCalls?: ToolCallInfo[];
  iteration?: LLMIterationInfo;
  thinking?: string;
}

interface Conversation {
  conv_id: string;
  preset_id: string;
  preset_name: string;
  title?: string;
  last_message?: string;
  last_message_time?: string;
  message_count: number;
  create_time?: string;
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

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  // 获取指定Agent的对话列表
  const fetchConversations = useCallback(async (presetId: string) => {
    try {
      const response = await conversationApi.list({
        conversation_type: 'agent',
        agent_preset_id: presetId, // 直接使用后端筛选
        page: 1,
        page_size: 100,
      });

      // 转换为前端格式
      const formattedConvs: Conversation[] = response.conversations.map((conv: any) => ({
        conv_id: conv.conv_id,
        preset_id: presetId,
        preset_name: conv.title || 'Agent',
        title: conv.title,
        last_message: conv.last_message,
        last_message_time: conv.last_message_time || conv.create_time,
        message_count: conv.message_count || 0,
        create_time: conv.create_time,
        messages: [], // 消息会在选择对话时加载
      }));

      setConversations(formattedConvs);
      return formattedConvs;
    } catch (error) {
      logger.error('Failed to fetch conversations:', error);
      return [];
    }
  }, []);

  const handleSelectPreset = useCallback(async (presetId: string) => {
    setSelectedPreset(presetId);
    setMessages([]);

    // 获取该Agent的对话列表
    const convs = await fetchConversations(presetId);

    // 如果没有对话，自动创建一个新对话
    if (convs.length === 0) {
      const newConvId = generateId();
      const preset = presets.find(p => p.preset_id === presetId);

      try {
        // 调用后端API创建对话
        await conversationApi.createAgentConversation({
          conv_id: newConvId,
          preset_id: presetId,
          user_id: USER.ID,
        });

        const newConv: Conversation = {
          conv_id: newConvId,
          preset_id: presetId,
          preset_name: preset?.preset_name || 'Agent',
          message_count: 0,
          messages: [],
        };

        setConversations([newConv]);
        setCurrentConvId(newConvId);
      } catch (error: any) {
        logger.error('Failed to create conversation:', error);
        showError('创建对话失败: ' + (error.message || '未知错误'));
        setCurrentConvId('');
      }
    } else {
      // 如果有对话，选择最新的一个
      setCurrentConvId(convs[0].conv_id);
    }
  }, [fetchConversations, presets]);

  const fetchPresets = useCallback(async () => {
    try {
      const response = await agentApi.list({ user_id: USER.ID, page: 1, page_size: 100 });
      setPresets(response.list || []);
      // 不再自动选择第一个preset，让用户手动选择
    } catch (error) {
      logger.error('Failed to fetch agent presets:', error);
    }
  }, []);

  useEffect(() => {
    fetchPresets();
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleNewConversation = async () => {
    if (!selectedPreset) {
      showWarning('请先选择一个 Agent');
      return;
    }

    const newConvId = generateId();
    const preset = presets.find(p => p.preset_id === selectedPreset);

    try {
      // 调用后端API创建对话
      await conversationApi.createAgentConversation({
        conv_id: newConvId,
        preset_id: selectedPreset,
        user_id: USER.ID,
      });

      const newConv: Conversation = {
        conv_id: newConvId,
        preset_id: selectedPreset,
        preset_name: preset?.preset_name || 'Agent',
        message_count: 0,
        messages: [],
      };

      setConversations(prev => [newConv, ...prev]);
      setCurrentConvId(newConvId);
      setMessages([]);
    } catch (error: any) {
      logger.error('Failed to create conversation:', error);
      showError('创建对话失败: ' + (error.message || '未知错误'));
    }
  };

  const handleSend = useCallback(async () => {
    if (!input.trim() || !selectedPreset) {
      showWarning('请输入消息并选择 Agent');
      return;
    }

    // 确保有conv_id
    if (!currentConvId) {
      showWarning('请先点击"新对话"按钮创建对话');
      return;
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
      // 工具调用状态管理
      const toolCallsMap = new Map<string, ToolCallInfo>();

      setMessages(prev => [
        ...prev,
        {
          id: assistantMessageId,
          role: 'assistant',
          content: '',
          reasoning_content: '',
          timestamp: new Date().toISOString(),
          toolCalls: [],
        },
      ]);

      await agentApi.chatStream(
        {
          preset_id: selectedPreset,
          user_id: USER.ID,
          question: currentInput,
          conv_id: currentConvId,
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
                    references: references || msg.references,
                    // 当开始接收内容时，清除迭代和思考状态
                    iteration: undefined,
                    thinking: undefined,
                  }
                : msg
            )
          );
        },
        (error) => {
          logger.error('Stream error:', error);
          showError('发送失败: ' + error.message);
          setMessages(prev =>
            prev.map(msg =>
              msg.id === assistantMessageId
                ? {
                    ...msg,
                    content: '抱歉，发生了错误：' + error.message,
                    iteration: undefined,
                    thinking: undefined,
                  }
                : msg
            )
          );
        },
        {
          onToolCallStart: (toolCall: ToolCallInfo) => {
            toolCallsMap.set(toolCall.tool_id, toolCall);
            setMessages(prev =>
              prev.map(msg =>
                msg.id === assistantMessageId
                  ? {
                      ...msg,
                      toolCalls: Array.from(toolCallsMap.values()),
                    }
                  : msg
              )
            );
          },
          onToolCallEnd: (toolCall: ToolCallInfo) => {
            const existing = toolCallsMap.get(toolCall.tool_id);
            if (existing) {
              toolCallsMap.set(toolCall.tool_id, { ...existing, ...toolCall });
              setMessages(prev =>
                prev.map(msg =>
                  msg.id === assistantMessageId
                    ? {
                        ...msg,
                        toolCalls: Array.from(toolCallsMap.values()),
                      }
                    : msg
                )
              );
            }
          },
          onLLMIteration: (iteration: LLMIterationInfo) => {
            setMessages(prev =>
              prev.map(msg =>
                msg.id === assistantMessageId
                  ? {
                      ...msg,
                      iteration,
                    }
                  : msg
              )
            );
          },
          onThinking: (thinking: string) => {
            setMessages(prev =>
              prev.map(msg =>
                msg.id === assistantMessageId
                  ? {
                      ...msg,
                      thinking,
                    }
                  : msg
              )
            );
          },
        }
      );
    } catch (error: any) {
      logger.error('Failed to send message:', error);
      showError('发送失败: ' + (error.message || '未知错误'));
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
  }, [input, selectedPreset, currentConvId, attachedFiles, presets]);

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

  const handleDeleteConversation = async (e: React.MouseEvent, convId: string) => {
    e.stopPropagation();
    if (!confirm('确定要删除这个对话吗?')) return;

    try {
      await conversationApi.delete(convId);
      // 从列表中移除
      setConversations(prev => prev.filter(c => c.conv_id !== convId));
      // 如果删除的是当前对话，清空消息
      if (currentConvId === convId) {
        setCurrentConvId('');
        setMessages([]);
      }
    } catch (error) {
      logger.error('Failed to delete conversation:', error);
      showError('删除对话失败');
    }
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
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {conversations.length === 0 ? (
            <div className="text-center py-8 text-gray-500 text-sm">
              还没有对话记录
            </div>
          ) : (
            conversations.map((conv) => (
              <div
                key={conv.conv_id}
                onClick={async () => {
                  setCurrentConvId(conv.conv_id);
                  setSelectedPreset(conv.preset_id);

                  // 从后端加载该对话的消息历史
                  try {
                    const detail = await conversationApi.get(conv.conv_id);
                    // 转换消息格式 - 过滤掉 tool 角色的消息，因为它们会被附加到 assistant 消息的 extra 字段中
                    const loadedMessages: Message[] = detail.messages
                      .filter((msg: any) => msg.role !== 'tool')
                      .map((msg: any, index: number) => {
                        // 解析 extra 字段中的工具调用信息
                        let toolCalls: ToolCallInfo[] | undefined;
                        if (msg.extra && msg.extra.tool && Array.isArray(msg.extra.tool)) {
                          toolCalls = msg.extra.tool.map((tool: any) => ({
                            tool_id: tool.tool_call_id || `tool_${index}`,
                            tool_name: tool.tool_name || 'unknown',
                            arguments: tool.tool_args,
                            result: tool.content,
                            status: 'success' as const,
                          }));
                        }

                        return {
                          id: Date.now() + index,
                          role: msg.role,
                          content: msg.content,
                          reasoning_content: msg.reasoning_content,
                          references: msg.references,
                          timestamp: msg.create_time || new Date().toISOString(),
                          toolCalls,
                        };
                      });
                    setMessages(loadedMessages);
                  } catch (error) {
                    logger.error('Failed to load conversation messages:', error);
                    showError('加载对话消息失败');
                    setMessages([]);
                  }
                }}
                className={`p-3 rounded-lg cursor-pointer transition-colors group ${
                  conv.conv_id === currentConvId
                    ? 'bg-primary-50 border border-primary-200'
                    : 'hover:bg-gray-50'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center space-x-2 mb-1">
                      <MessageSquare className="w-4 h-4 text-gray-400 flex-shrink-0" />
                      <h3 className="text-sm font-medium text-gray-900 truncate">
                        {conv.create_time ? formatDate(conv.create_time) : '新对话'}
                      </h3>
                    </div>
                    <p className="text-xs text-gray-500 line-clamp-2">
                      {truncate(conv.last_message || '暂无消息', 60)}
                    </p>
                    <div className="flex items-center justify-between mt-2">
                      <span className="text-xs text-gray-400">
                        {conv.last_message_time ? formatDate(conv.last_message_time) : ''}
                      </span>
                      <span className="text-xs text-gray-400">
                        {conv.message_count} 条消息
                      </span>
                    </div>
                  </div>

                  <button
                    onClick={(e) => handleDeleteConversation(e, conv.conv_id)}
                    className="ml-2 p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-red-100 text-red-600 transition-opacity"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            ))
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
          {!selectedPreset ? (
            <div className="text-center py-12">
              <Bot className="w-16 h-16 text-gray-300 mx-auto mb-4" />
              <p className="text-gray-500 mb-2 text-lg font-medium">欢迎使用 Agent 对话</p>
              <p className="text-sm text-gray-400 mb-4">请先在左侧选择一个 Agent 开始对话</p>
              <div className="flex items-center justify-center gap-2 text-sm text-gray-400">
                <ArrowLeft className="w-4 h-4" />
                <span>在左侧下拉菜单中选择 Agent</span>
              </div>
            </div>
          ) : messages.length === 0 ? (
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
                      <>
                        {/* 如果是最后一条消息且正在加载且没有内容，显示加载动画 */}
                        {loading && messages[messages.length - 1]?.id === message.id && !message.content ? (
                          <Loader2 className="w-5 h-5 animate-spin text-blue-500" />
                        ) : (
                          <MessageContent
                            content={message.content}
                            reasoningContent={message.reasoning_content}
                            isStreaming={loading && messages[messages.length - 1]?.id === message.id}
                          />
                        )}
                      </>
                    )}

                    {message.references && message.references.length > 0 && (
                      <ReferencesList references={message.references} />
                    )}

                    {/* 多轮迭代显示 */}
                    {message.role === 'assistant' && message.iteration && (
                      <MultiTurnDisplay iteration={message.iteration} />
                    )}

                    {/* 思考过程显示 */}
                    {message.role === 'assistant' && message.thinking && !message.content && (
                      <ThinkingProcess content={message.thinking} typewriter={false} />
                    )}

                    {/* 工具调用状态显示 */}
                    {message.role === 'assistant' && message.toolCalls && message.toolCalls.length > 0 && (
                      <ToolCallStatus
                        toolCalls={message.toolCalls}
                      />
                    )}

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
