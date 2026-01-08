import { useState, useEffect } from 'react';
import { MessageSquare, Trash2, Plus } from 'lucide-react';
import { conversationApi } from '@/services';
import type { Conversation } from '@/types';
import { formatDate, truncate } from '@/lib/utils';

interface ConversationSidebarProps {
  currentConvId: string;
  onLoadConversation: (convId: string) => void;
  onDeleteConversation: (convId: string) => void;
  onNewConversation: () => void;
}

export default function ConversationSidebar({
  currentConvId,
  onLoadConversation,
  onDeleteConversation,
  onNewConversation,
}: ConversationSidebarProps) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchConversations();
  }, []);

  const fetchConversations = async () => {
    try {
      setLoading(true);
      const response = await conversationApi.list({
        page_size: 50,
        conversation_type: 'text' // 只查询普通对话
      });
      setConversations(response.conversations || []);
    } catch (error) {
      console.error('Failed to fetch conversations:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (e: React.MouseEvent, convId: string) => {
    e.stopPropagation();
    if (!confirm('确定要删除这个对话吗?')) return;

    try {
      await onDeleteConversation(convId);
      fetchConversations();
    } catch (error) {
      console.error('Failed to delete conversation:', error);
    }
  };

  return (
    <div className="w-80 bg-white rounded-lg shadow-sm border border-gray-200 flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-200">
        <button
          onClick={onNewConversation}
          className="w-full btn btn-primary flex items-center justify-center"
        >
          <Plus className="w-4 h-4 mr-2" />
          新对话
        </button>
      </div>

      {/* Conversations List */}
      <div className="flex-1 overflow-y-auto p-2 space-y-1">
        {loading ? (
          <div className="text-center py-8 text-gray-500">
            加载中...
          </div>
        ) : conversations.length === 0 ? (
          <div className="text-center py-8 text-gray-500 text-sm">
            还没有对话记录
          </div>
        ) : (
          conversations.map((conv) => (
            <div
              key={conv.conv_id}
              onClick={() => onLoadConversation(conv.conv_id)}
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
                      {conv.title || '未命名对话'}
                    </h3>
                  </div>
                  <p className="text-xs text-gray-500 line-clamp-2">
                    {truncate(conv.last_message, 60)}
                  </p>
                  <div className="flex items-center justify-between mt-2">
                    <span className="text-xs text-gray-400">
                      {formatDate(conv.last_message_time)}
                    </span>
                    <span className="text-xs text-gray-400">
                      {conv.message_count} 条消息
                    </span>
                  </div>
                </div>

                <button
                  onClick={(e) => handleDelete(e, conv.conv_id)}
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
  );
}