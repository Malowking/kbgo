import { User, Bot, Wrench } from 'lucide-react';
import type { Message } from '@/types';
import { formatDate } from '@/lib/utils';
import MessageContent from '@/components/MessageContent';
import ReferencesList from '@/components/ReferencesList';

interface ChatMessageProps {
  message: Message;
  isStreaming?: boolean;
}

export default function ChatMessage({ message, isStreaming = false }: ChatMessageProps) {
  const isUser = message.role === 'user';
  const toolResults = message.extra?.tool_results || [];

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

          {/* Tool Results Section - 显示工具调用结果 */}
          {!isUser && toolResults.length > 0 && (
            <div className="mt-3 space-y-2">
              {toolResults.map((toolResult, index) => (
                <div
                  key={index}
                  className="rounded-md border border-blue-200 bg-gradient-to-r from-blue-50 to-indigo-50 shadow-sm"
                >
                  {/* 工具调用头部 */}
                  <div className="flex items-center gap-2 px-3 py-2 border-b border-blue-200 bg-blue-100/50">
                    <div className="flex items-center justify-center w-6 h-6 rounded-full bg-blue-500">
                      <Wrench className="w-3.5 h-3.5 text-white" />
                    </div>
                    <span className="text-sm font-semibold text-blue-900">
                      {toolResult.tool_name || '工具调用'}
                    </span>
                    <span className="ml-auto text-xs text-blue-600 font-medium">
                      执行结果
                    </span>
                  </div>

                  {/* 工具调用内容 */}
                  <div className="px-3 py-3">
                    <div className="text-sm text-gray-800 whitespace-pre-wrap leading-relaxed">
                      {toolResult.content}
                    </div>

                    {/* 工具参数（可折叠） */}
                    {toolResult.tool_args && (
                      <details className="mt-3 group">
                        <summary className="text-xs text-blue-600 cursor-pointer hover:text-blue-800 font-medium flex items-center gap-1">
                          <span>查看调用参数</span>
                          <svg
                            className="w-3 h-3 transition-transform group-open:rotate-180"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                          </svg>
                        </summary>
                        <pre className="mt-2 text-xs text-gray-700 bg-white/80 p-3 rounded border border-blue-100 overflow-x-auto font-mono">
                          {JSON.stringify(toolResult.tool_args, null, 2)}
                        </pre>
                      </details>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* References Section - 只在助手消息且有references时显示 */}
          {!isUser && message.references && message.references.length > 0 && (
            <ReferencesList references={message.references} />
          )}

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