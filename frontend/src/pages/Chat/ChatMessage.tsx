import { User, Bot } from 'lucide-react';
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