import { CheckCircle2, Loader2, XCircle, MinusCircle, Brain, Wrench, MessageSquare } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ToolExecutionStep } from '@/types';

interface ToolExecutionProgressProps {
  reasoning?: string;
  steps: ToolExecutionStep[];
  stepsCount?: number;
  needTools?: boolean;
}

const statusLabels: Record<ToolExecutionStep['status'], string> = {
  pending: '等待中',
  running: '执行中',
  completed: '已完成',
  failed: '失败',
};

const statusColors: Record<ToolExecutionStep['status'], string> = {
  pending: 'border-gray-200 bg-gray-50',
  running: 'border-blue-200 bg-blue-50',
  completed: 'border-green-200 bg-green-50',
  failed: 'border-red-200 bg-red-50',
};

function StatusIcon({ status }: { status: ToolExecutionStep['status'] }) {
  switch (status) {
    case 'running':
      return <Loader2 className="w-4 h-4 text-blue-500 animate-spin" />;
    case 'completed':
      return <CheckCircle2 className="w-4 h-4 text-green-500" />;
    case 'failed':
      return <XCircle className="w-4 h-4 text-red-500" />;
    default:
      return <MinusCircle className="w-4 h-4 text-gray-400" />;
  }
}

export default function ToolExecutionProgress({
  reasoning,
  steps,
  stepsCount,
  needTools,
}: ToolExecutionProgressProps) {
  const hasContent = steps.length > 0 || !!reasoning || needTools === false || stepsCount !== undefined;

  if (!hasContent) {
    return null;
  }

  const headerText = needTools === false ? '无需使用工具' : '工具执行计划';
  const stepsSummary = needTools === false ? '' : stepsCount !== undefined ? `（${stepsCount} 步）` : '';

  // 计算执行进度
  const completedSteps = steps.filter(s => s.status === 'completed').length;
  const failedSteps = steps.filter(s => s.status === 'failed').length;
  const runningSteps = steps.filter(s => s.status === 'running').length;
  const totalSteps = steps.length;

  return (
    <div className="mt-3 rounded-lg border border-blue-200 bg-gradient-to-br from-blue-50/80 to-indigo-50/60 shadow-sm">
      {/* 头部 */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b border-blue-200 bg-gradient-to-r from-blue-100/70 to-indigo-100/50">
        <div className="flex items-center gap-2">
          <Brain className="w-4 h-4 text-blue-600" />
          <span className="text-sm font-semibold text-blue-900">
            {headerText}{stepsSummary}
          </span>
        </div>
        {totalSteps > 0 && (
          <div className="flex items-center gap-2 text-xs">
            {runningSteps > 0 && (
              <span className="flex items-center gap-1 text-blue-600">
                <Loader2 className="w-3 h-3 animate-spin" />
                执行中 {runningSteps}
              </span>
            )}
            {completedSteps > 0 && (
              <span className="flex items-center gap-1 text-green-600">
                <CheckCircle2 className="w-3 h-3" />
                完成 {completedSteps}
              </span>
            )}
            {failedSteps > 0 && (
              <span className="flex items-center gap-1 text-red-600">
                <XCircle className="w-3 h-3" />
                失败 {failedSteps}
              </span>
            )}
          </div>
        )}
      </div>

      {/* 思考过程 */}
      {reasoning && (
        <div className="px-4 py-3 border-b border-blue-100">
          <div className="flex items-start gap-2">
            <MessageSquare className="w-4 h-4 text-indigo-500 mt-0.5 flex-shrink-0" />
            <div className="flex-1">
              <div className="text-xs font-medium text-indigo-700 mb-1">思考过程</div>
              <div className="text-xs text-gray-700 leading-relaxed prose prose-sm max-w-none">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  components={{
                    p({ children }) {
                      return <p className="my-1 leading-relaxed">{children}</p>;
                    },
                    ul({ children }) {
                      return <ul className="my-1 ml-4 list-disc">{children}</ul>;
                    },
                    ol({ children }) {
                      return <ol className="my-1 ml-4 list-decimal">{children}</ol>;
                    },
                    li({ children }) {
                      return <li className="my-0.5">{children}</li>;
                    },
                    code({ children }) {
                      return <code className="px-1 py-0.5 bg-gray-200 rounded text-xs font-mono">{children}</code>;
                    },
                    strong({ children }) {
                      return <strong className="font-semibold text-gray-900">{children}</strong>;
                    },
                    em({ children }) {
                      return <em className="italic">{children}</em>;
                    },
                  }}
                >
                  {reasoning}
                </ReactMarkdown>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 执行步骤 */}
      {steps.length > 0 && (
        <div className="px-4 py-3 space-y-2.5">
          {steps.map((step, index) => (
            <div
              key={step.step_id}
              className={`rounded-lg border ${statusColors[step.status]} px-3 py-2.5 transition-all duration-200`}
            >
              {/* 步骤头部 */}
              <div className="flex items-center gap-2">
                <div className="flex items-center justify-center w-5 h-5 rounded-full bg-white border border-gray-300 text-xs font-medium text-gray-600">
                  {index + 1}
                </div>
                <StatusIcon status={step.status} />
                <Wrench className="w-3.5 h-3.5 text-gray-500" />
                <span className="text-sm font-semibold text-gray-900">{step.tool_name}</span>
                <span className={`ml-auto text-xs font-medium px-2 py-0.5 rounded-full ${
                  step.status === 'running' ? 'bg-blue-100 text-blue-700' :
                  step.status === 'completed' ? 'bg-green-100 text-green-700' :
                  step.status === 'failed' ? 'bg-red-100 text-red-700' :
                  'bg-gray-100 text-gray-600'
                }`}>
                  {statusLabels[step.status]}
                </span>
              </div>

              {/* 执行原因 */}
              {step.reason && (
                <div className="mt-2 pl-7 text-xs text-gray-700 bg-white/60 rounded px-2 py-1.5 border border-gray-200">
                  <span className="font-medium text-gray-600">原因：</span>
                  {step.reason}
                </div>
              )}

              {/* 执行进度 */}
              {step.progress && (
                <div className="mt-2 pl-7 text-xs text-blue-700 bg-blue-100/60 rounded px-2 py-1.5 border border-blue-200 flex items-start gap-1.5">
                  <Loader2 className="w-3 h-3 animate-spin mt-0.5 flex-shrink-0" />
                  <span className="flex-1">{step.progress}</span>
                </div>
              )}

              {/* 执行结果 */}
              {step.result_summary && (
                <div className="mt-2 pl-7 text-xs text-green-800 bg-green-100/60 rounded px-2 py-1.5 border border-green-200">
                  <span className="font-medium text-green-700">结果：</span>
                  {step.result_summary}
                </div>
              )}

              {/* 错误信息 */}
              {step.error && (
                <div className="mt-2 pl-7 text-xs text-red-800 bg-red-100/60 rounded px-2 py-1.5 border border-red-200 flex items-start gap-1.5">
                  <XCircle className="w-3 h-3 mt-0.5 flex-shrink-0" />
                  <div className="flex-1">
                    <span className="font-medium text-red-700">错误：</span>
                    {step.error}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
