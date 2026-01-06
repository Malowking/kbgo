/**
 * 引用列表组件
 * 显示聊天消息中检索到的文档片段
 * 解决原 ChatMessage 组件中的 hooks 违规问题
 */

import { useState, useMemo } from 'react';
import { ChevronDown, FileText, Database, Search } from 'lucide-react';
import { cn } from '@/lib/utils';

interface ReferencesListProps {
  references: any[];
}

interface DocumentGroupProps {
  docName: string;
  chunks: any[];
  toolType?: string;
}

/**
 * 获取工具类型的显示信息
 */
function getToolTypeInfo(toolType?: string, type?: string) {
  // 优先使用 tool_type，其次使用 type
  const actualType = toolType || type;

  switch (actualType) {
    case 'knowledge_retrieval':
      return {
        icon: Search,
        label: '知识库检索',
        color: 'blue'
      };
    case 'nl2sql_query':
    case 'nl2sql_result':
    case 'nl2sql':
      return {
        icon: Database,
        label: 'NL2SQL查询',
        color: 'purple'
      };
    default:
      return {
        icon: FileText,
        label: '文档',
        color: 'gray'
      };
  }
}

/**
 * 单个文档分组组件
 */
function DocumentGroup({ docName, chunks, toolType }: DocumentGroupProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  // 从第一个 chunk 获取工具类型信息
  const firstChunk = chunks[0];
  const typeInfo = getToolTypeInfo(
    firstChunk?.metadata?.tool_type,
    firstChunk?.metadata?.type
  );
  const Icon = typeInfo.icon;

  return (
    <div className="mb-2 last:mb-0">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className={cn(
          "flex items-center justify-between w-full px-3 py-2.5 text-sm rounded-lg transition-colors",
          typeInfo.color === 'blue' && "bg-blue-50 hover:bg-blue-100",
          typeInfo.color === 'purple' && "bg-purple-50 hover:bg-purple-100",
          typeInfo.color === 'gray' && "bg-gray-50 hover:bg-gray-100"
        )}
      >
        <div className="flex items-center gap-2">
          <Icon className={cn(
            "w-4 h-4 flex-shrink-0",
            typeInfo.color === 'blue' && "text-blue-600",
            typeInfo.color === 'purple' && "text-purple-600",
            typeInfo.color === 'gray' && "text-gray-600"
          )} />
          <span className={cn(
            "font-medium truncate",
            typeInfo.color === 'blue' && "text-blue-900",
            typeInfo.color === 'purple' && "text-purple-900",
            typeInfo.color === 'gray' && "text-gray-900"
          )}>
            {docName}
          </span>
          <span className={cn(
            "text-xs px-2 py-0.5 rounded-full",
            typeInfo.color === 'blue' && "bg-blue-200 text-blue-700",
            typeInfo.color === 'purple' && "bg-purple-200 text-purple-700",
            typeInfo.color === 'gray' && "bg-gray-200 text-gray-700"
          )}>
            {typeInfo.label}
          </span>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          <span className={cn(
            "text-xs px-2 py-0.5 rounded-full",
            typeInfo.color === 'blue' && "text-blue-600 bg-blue-200",
            typeInfo.color === 'purple' && "text-purple-600 bg-purple-200",
            typeInfo.color === 'gray' && "text-gray-600 bg-gray-200"
          )}>
            {chunks.length} 个片段
          </span>
          <ChevronDown
            className={cn(
              'w-4 h-4 transition-transform duration-200',
              typeInfo.color === 'blue' && "text-blue-600",
              typeInfo.color === 'purple' && "text-purple-600",
              typeInfo.color === 'gray' && "text-gray-600",
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
  // 按文档名和工具类型分组
  const docGroups = useMemo(() => {
    return references.reduce((groups: Record<string, any[]>, ref: any) => {
      // 获取工具类型
      const toolType = ref.metadata?.tool_type || ref.metadata?.type;

      // 根据工具类型决定文档名
      let docName: string;
      if (toolType === 'nl2sql_query' || toolType === 'nl2sql_result' || toolType === 'nl2sql') {
        docName = 'NL2SQL 查询结果';
      } else if (toolType === 'knowledge_retrieval') {
        docName = ref.metadata?.document_name || '知识库文档';
      } else {
        docName = ref.metadata?.document_name || '未知文档';
      }

      // 创建唯一的分组键（文档名 + 工具类型）
      const groupKey = `${docName}__${toolType || 'unknown'}`;

      if (!groups[groupKey]) {
        groups[groupKey] = [];
      }
      groups[groupKey].push(ref);
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
          工具调用结果
        </h4>
        <span className="text-xs text-gray-500">
          {totalDocs} 个来源, {totalChunks} 个片段
        </span>
      </div>

      <div className="space-y-2">
        {Object.entries(docGroups).map(([groupKey, chunks]) => {
          // 从 groupKey 中提取文档名（去掉工具类型后缀）
          const docName = groupKey.split('__')[0];
          return (
            <DocumentGroup key={groupKey} docName={docName} chunks={chunks} />
          );
        })}
      </div>
    </div>
  );
}