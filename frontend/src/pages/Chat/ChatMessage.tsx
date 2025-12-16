import { User, Bot } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import type { Message } from '@/types';
import { formatDate } from '@/lib/utils';

interface ChatMessageProps {
  message: Message;
}

export default function ChatMessage({ message }: ChatMessageProps) {
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
              <div className="prose prose-sm max-w-none">
                <ReactMarkdown>{message.content}</ReactMarkdown>
              </div>
            )}

            {message.reasoning_content && (
              <details className="mt-3 pt-3 border-t border-gray-300">
                <summary className="cursor-pointer text-sm font-medium">
                  思考过程
                </summary>
                <div className="mt-2 text-sm opacity-80">
                  <ReactMarkdown>{message.reasoning_content}</ReactMarkdown>
                </div>
              </details>
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