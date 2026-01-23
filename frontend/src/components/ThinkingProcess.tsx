/**
 * 思考过程显示组件
 * 显示LLM的思考过程，带有打字机效果
 */

import React, { useState, useEffect } from 'react';
import { Brain, Sparkles } from 'lucide-react';

interface ThinkingProcessProps {
  /** 思考内容 */
  content: string;
  /** 是否显示打字机效果 */
  typewriter?: boolean;
  /** 打字机速度（毫秒/字符） */
  typewriterSpeed?: number;
}

/**
 * 思考过程组件
 */
export const ThinkingProcess: React.FC<ThinkingProcessProps> = ({
  content,
  typewriter = false,
  typewriterSpeed = 30,
}) => {
  const [displayedContent, setDisplayedContent] = useState('');
  const [isTyping, setIsTyping] = useState(false);

  useEffect(() => {
    if (!typewriter) {
      setDisplayedContent(content);
      return;
    }

    setIsTyping(true);
    let currentIndex = 0;
    const interval = setInterval(() => {
      if (currentIndex < content.length) {
        setDisplayedContent(content.slice(0, currentIndex + 1));
        currentIndex++;
      } else {
        setIsTyping(false);
        clearInterval(interval);
      }
    }, typewriterSpeed);

    return () => clearInterval(interval);
  }, [content, typewriter, typewriterSpeed]);

  if (!content) {
    return null;
  }

  return (
    <div className="my-3 animate-fadeIn">
      <div className="relative overflow-hidden rounded-lg bg-gradient-to-r from-purple-50 via-pink-50 to-purple-50 dark:from-purple-900/20 dark:via-pink-900/20 dark:to-purple-900/20 border border-purple-200 dark:border-purple-800">
        {/* 背景装饰 */}
        <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/50 to-transparent dark:via-white/5 animate-shimmer" />

        {/* 内容 */}
        <div className="relative px-4 py-3">
          <div className="flex items-start gap-3">
            {/* 图标 */}
            <div className="flex-shrink-0 mt-0.5">
              <div className="relative">
                <Brain className="w-5 h-5 text-purple-600 dark:text-purple-400" />
                <Sparkles className="w-3 h-3 text-pink-500 absolute -top-1 -right-1 animate-pulse" />
              </div>
            </div>

            {/* 思考内容 */}
            <div className="flex-1 min-w-0">
              <div className="text-xs font-medium text-purple-700 dark:text-purple-300 mb-1">
                思考中...
              </div>
              <div className="text-sm text-gray-700 dark:text-gray-300 italic">
                {displayedContent}
                {isTyping && (
                  <span className="inline-block w-0.5 h-4 ml-1 bg-purple-600 dark:bg-purple-400 animate-blink" />
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ThinkingProcess;