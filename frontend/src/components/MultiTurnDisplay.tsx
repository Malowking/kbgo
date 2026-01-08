/**
 * 多轮对话展示组件
 * 显示LLM迭代进度和历史
 */

import React from 'react';
import { LLMIterationInfo } from '@/lib/sse-client';
import { Loader2, CheckCircle2, Circle } from 'lucide-react';

interface MultiTurnDisplayProps {
  /** 当前迭代信息 */
  iteration: LLMIterationInfo;
  /** 是否显示进度条 */
  showProgress?: boolean;
}

/**
 * 多轮对话展示组件
 */
export const MultiTurnDisplay: React.FC<MultiTurnDisplayProps> = ({
  iteration,
  showProgress = true,
}) => {
  const { iteration: current, max_iterations: max, message } = iteration;
  const progress = (current / max) * 100;

  return (
    <div className="my-3 animate-fadeIn">
      <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
        {/* 标题和进度 */}
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Loader2 className="w-5 h-5 animate-spin text-blue-600 dark:text-blue-400" />
            <span className="text-sm font-medium text-blue-900 dark:text-blue-100">
              多轮推理进行中
            </span>
          </div>
          <div className="text-sm font-semibold text-blue-700 dark:text-blue-300">
            {current} / {max}
          </div>
        </div>

        {/* 进度条 */}
        {showProgress && (
          <div className="mb-3">
            <div className="h-2 bg-blue-100 dark:bg-blue-900/40 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-blue-500 to-blue-600 dark:from-blue-400 dark:to-blue-500 transition-all duration-500 ease-out"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        )}

        {/* 当前消息 */}
        {message && (
          <div className="text-sm text-blue-800 dark:text-blue-200">
            {message}
          </div>
        )}

        {/* 迭代历史时间线 */}
        <div className="mt-4 flex items-center gap-2">
          {Array.from({ length: max }, (_, i) => i + 1).map((step) => (
            <div key={step} className="flex items-center">
              {/* 步骤圆点 */}
              <div
                className={`flex items-center justify-center w-8 h-8 rounded-full border-2 transition-all duration-300 ${
                  step < current
                    ? 'bg-green-500 border-green-500 dark:bg-green-600 dark:border-green-600'
                    : step === current
                    ? 'bg-blue-500 border-blue-500 dark:bg-blue-600 dark:border-blue-600 animate-pulse'
                    : 'bg-gray-200 border-gray-300 dark:bg-gray-700 dark:border-gray-600'
                }`}
              >
                {step < current ? (
                  <CheckCircle2 className="w-4 h-4 text-white" />
                ) : step === current ? (
                  <Loader2 className="w-4 h-4 text-white animate-spin" />
                ) : (
                  <Circle className="w-3 h-3 text-gray-400 dark:text-gray-500" />
                )}
              </div>

              {/* 连接线 */}
              {step < max && (
                <div
                  className={`h-0.5 w-8 transition-all duration-300 ${
                    step < current
                      ? 'bg-green-500 dark:bg-green-600'
                      : 'bg-gray-300 dark:bg-gray-600'
                  }`}
                />
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default MultiTurnDisplay;