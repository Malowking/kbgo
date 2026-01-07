/**
 * 工具调用状态显示组件
 * 实时显示工具调用的进度和结果
 */

import React, { useState } from 'react';
import { ToolCallInfo, LLMIterationInfo } from '@/lib/sse-client';
import { Loader2, CheckCircle2, XCircle, ChevronDown, ChevronRight, Clock } from 'lucide-react';

interface ToolCallStatusProps {
  /** 工具调用列表 */
  toolCalls: ToolCallInfo[];
  /** LLM迭代信息 */
  iteration?: LLMIterationInfo;
  /** 思考内容 */
  thinking?: string;
}

/**
 * 工具调用状态组件
 */
export const ToolCallStatus: React.FC<ToolCallStatusProps> = ({
  toolCalls,
  iteration,
  thinking,
}) => {
  const [expandedTools, setExpandedTools] = useState<Set<string>>(new Set());

  // 切换工具详情展开/折叠
  const toggleToolExpand = (toolId: string) => {
    setExpandedTools((prev) => {
      const next = new Set(prev);
      if (next.has(toolId)) {
        next.delete(toolId);
      } else {
        next.add(toolId);
      }
      return next;
    });
  };

  // 获取工具名称的显示文本
  const getToolDisplayName = (toolName: string): string => {
    const nameMap: Record<string, string> = {
      knowledge_retrieval: '知识检索',
      nl2sql: '数据查询',
      file_export: '文件导出',
    };
    return nameMap[toolName] || toolName;
  };

  // 格式化执行时间
  const formatDuration = (ms?: number): string => {
    if (!ms) return '';
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  if (toolCalls.length === 0 && !iteration && !thinking) {
    return null;
  }

  return (
    <div className="space-y-2 my-3">
      {/* LLM迭代进度 */}
      {iteration && (
        <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400 bg-blue-50 dark:bg-blue-900/20 px-3 py-2 rounded-lg">
          <Loader2 className="w-4 h-4 animate-spin text-blue-500" />
          <span>
            第 {iteration.iteration}/{iteration.max_iterations} 轮 - {iteration.message}
          </span>
        </div>
      )}

      {/* 思考过程 */}
      {thinking && (
        <div className="text-sm text-gray-600 dark:text-gray-400 bg-purple-50 dark:bg-purple-900/20 px-3 py-2 rounded-lg italic">
          💭 {thinking}
        </div>
      )}

      {/* 工具调用列表 */}
      {toolCalls.length > 0 && (
        <div className="space-y-2">
          {toolCalls.map((tool) => {
            const isExpanded = expandedTools.has(tool.tool_id);
            const hasDetails = tool.arguments || tool.result || tool.error;

            return (
              <div
                key={tool.tool_id}
                className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden"
              >
                {/* 工具头部 */}
                <div
                  className={`flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors ${
                    tool.status === 'running'
                      ? 'bg-blue-50 dark:bg-blue-900/20'
                      : tool.status === 'success'
                      ? 'bg-green-50 dark:bg-green-900/20'
                      : tool.status === 'error'
                      ? 'bg-red-50 dark:bg-red-900/20'
                      : 'bg-gray-50 dark:bg-gray-800'
                  }`}
                  onClick={() => hasDetails && toggleToolExpand(tool.tool_id)}
                >
                  {/* 状态图标 */}
                  <div className="flex-shrink-0">
                    {tool.status === 'running' && (
                      <Loader2 className="w-4 h-4 animate-spin text-blue-500" />
                    )}
                    {tool.status === 'success' && (
                      <CheckCircle2 className="w-4 h-4 text-green-500" />
                    )}
                    {tool.status === 'error' && (
                      <XCircle className="w-4 h-4 text-red-500" />
                    )}
                  </div>

                  {/* 工具名称 */}
                  <span className="flex-1 text-sm font-medium text-gray-900 dark:text-gray-100">
                    {getToolDisplayName(tool.tool_name)}
                  </span>

                  {/* 执行时间 */}
                  {tool.duration && (
                    <div className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
                      <Clock className="w-3 h-3" />
                      <span>{formatDuration(tool.duration)}</span>
                    </div>
                  )}

                  {/* 展开/折叠图标 */}
                  {hasDetails && (
                    <div className="flex-shrink-0">
                      {isExpanded ? (
                        <ChevronDown className="w-4 h-4 text-gray-400" />
                      ) : (
                        <ChevronRight className="w-4 h-4 text-gray-400" />
                      )}
                    </div>
                  )}
                </div>

                {/* 工具详情（展开时显示） */}
                {isExpanded && hasDetails && (
                  <div className="px-3 py-2 bg-white dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700 space-y-2">
                    {/* 参数 */}
                    {tool.arguments && (
                      <div>
                        <div className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                          参数:
                        </div>
                        <pre className="text-xs bg-gray-50 dark:bg-gray-800 p-2 rounded overflow-x-auto">
                          {JSON.stringify(tool.arguments, null, 2)}
                        </pre>
                      </div>
                    )}

                    {/* 结果 */}
                    {tool.result && (
                      <div>
                        <div className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                          结果:
                        </div>
                        <div className="text-xs text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-800 p-2 rounded">
                          {tool.result}
                        </div>
                      </div>
                    )}

                    {/* 错误 */}
                    {tool.error && (
                      <div>
                        <div className="text-xs font-medium text-red-500 mb-1">
                          错误:
                        </div>
                        <div className="text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-2 rounded">
                          {tool.error}
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};

export default ToolCallStatus;