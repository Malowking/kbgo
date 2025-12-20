import React from 'react';
import { User, Bot, FileText } from 'lucide-react';
import type { Message } from '@/types';
import { formatDate } from '@/lib/utils';
import MessageContent from '@/components/MessageContent';

interface ChatMessageProps {
  message: Message;
  isStreaming?: boolean;
}

export default function ChatMessage({ message, isStreaming = false }: ChatMessageProps) {
  const isUser = message.role === 'user';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className={`flex max-w-[80%] ${isUser ? 'flex-row-reverse' : 'flex-row'}`}>
        {/* Avatar */}
        <div className={`flex-shrink-0 ${isUser ? 'ml-3' : 'mr-3'}`}>
          <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
            isUser ? 'bg-primary-600' : 'bg-gray-600'
          }`}>
            {isUser ? (
              <User className="w-5 h-5 text-white" />
            ) : (
              <Bot className="w-5 h-5 text-white" />
            )}
          </div>
        </div>

        {/* Message Content */}
        <div className="flex-1">
          <div className={`rounded-lg px-4 py-3 ${
            isUser
              ? 'bg-primary-600 text-white'
              : 'bg-gray-100 text-gray-900'
          }`}>
            {isUser ? (
              <p className="whitespace-pre-wrap">{message.content}</p>
            ) : (
              <MessageContent
                content={message.content}
                reasoningContent={message.reasoning_content}
                isStreaming={isStreaming}
              />
            )}
          </div>

          {/* References Section - 只在助手消息且有references时显示 */}
          {!isUser && message.references && message.references.length > 0 && (() => {
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
                <div className="flex items-center gap-1 mb-1.5">
                  <FileText className="w-3.5 h-3.5 text-blue-600" />
                  <span className="text-xs font-medium text-blue-900">
                    知识检索结果 ({message.references.length} 个片段，来自 {Object.keys(docGroups).length} 个文档)
                  </span>
                </div>
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
                                  <p className="text-gray-600 text-xs line-clamp-2 leading-relaxed">
                                    {ref.content}
                                  </p>
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

          <div className={`mt-1 text-xs text-gray-500 ${isUser ? 'text-right' : 'text-left'}`}>
            {formatDate(message.create_time)}
            {message.tokens_used && (
              <span className="ml-2">
                · {message.tokens_used} tokens
              </span>
            )}
            {message.latency_ms && (
              <span className="ml-2">
                · {message.latency_ms}ms
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}