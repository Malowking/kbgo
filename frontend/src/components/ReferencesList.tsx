/**
 * 引用列表组件
 * 显示聊天消息中检索到的文档片段
 * 解决原 ChatMessage 组件中的 hooks 违规问题
 */

import { useState, useMemo } from 'react';
import { ChevronDown, FileText } from 'lucide-react';
import { cn } from '@/lib/utils';

interface ReferencesListProps {
  references: any[];
}

interface DocumentGroupProps {
  docName: string;
  chunks: any[];
}

/**
 * 单个文档分组组件
 */
function DocumentGroup({ docName, chunks }: DocumentGroupProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <div className="mb-2 last:mb-0">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="flex items-center justify-between w-full px-3 py-2.5 text-sm bg-blue-50 hover:bg-blue-100 rounded-lg transition-colors"
      >
        <div className="flex items-center gap-2">
          <FileText className="w-4 h-4 text-blue-600 flex-shrink-0" />
          <span className="font-medium text-blue-900 truncate">{docName}</span>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          <span className="text-xs text-blue-600 bg-blue-200 px-2 py-0.5 rounded-full">
            {chunks.length} 个片段
          </span>
          <ChevronDown
            className={cn(
              'w-4 h-4 text-blue-600 transition-transform duration-200',
              isExpanded && 'rotate-180'
            )}
          />
        </div>
      </button>

      {isExpanded && (
        <div className="mt-2 space-y-2 pl-4 animate-in slide-in-from-top-2 duration-200">
          {chunks.map((ref, idx) => (
            <div
              key={idx}
              className="p-3 bg-gray-50 border border-gray-200 rounded-lg text-xs hover:bg-gray-100 transition-colors"
            >
              <p className="text-gray-700 line-clamp-3 leading-relaxed mb-2">
                {ref.content || '无内容'}
              </p>
              <div className="flex items-center justify-between text-gray-500">
                {ref.metadata?.chunk_index !== undefined && (
                  <span>片段 #{ref.metadata.chunk_index + 1}</span>
                )}
                {ref.score !== undefined && (
                  <span className="bg-green-100 text-green-700 px-2 py-0.5 rounded">
                    相似度: {(ref.score * 100).toFixed(1)}%
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * 引用列表主组件
 */
export default function ReferencesList({ references }: ReferencesListProps) {
  // 按文档名分组
  const docGroups = useMemo(() => {
    return references.reduce((groups: Record<string, any[]>, ref: any) => {
      const docName = ref.metadata?.document_name || '未知文档';
      if (!groups[docName]) {
        groups[docName] = [];
      }
      groups[docName].push(ref);
      return groups;
    }, {});
  }, [references]);

  const totalChunks = references.length;
  const totalDocs = Object.keys(docGroups).length;

  return (
    <div className="mt-4 pt-4 border-t border-gray-200">
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-sm font-medium text-gray-700 flex items-center gap-2">
          <FileText className="w-4 h-4 text-gray-500" />
          检索到的文档片段
        </h4>
        <span className="text-xs text-gray-500">
          {totalDocs} 个文档, {totalChunks} 个片段
        </span>
      </div>

      <div className="space-y-2">
        {Object.entries(docGroups).map(([docName, chunks]) => (
          <DocumentGroup key={docName} docName={docName} chunks={chunks} />
        ))}
      </div>
    </div>
  );
}