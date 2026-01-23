/**
 * 统一的加载组件
 * 提供一致的加载状态展示
 */

import { cn } from '@/lib/utils';

export interface LoadingProps {
  /** 尺寸 */
  size?: 'sm' | 'md' | 'lg';
  /** 加载文本提示 */
  text?: string;
  /** 是否全屏居中 */
  fullScreen?: boolean;
  /** 自定义类名 */
  className?: string;
}

export default function Loading({
  size = 'md',
  text,
  fullScreen = false,
  className,
}: LoadingProps) {
  const spinnerSizes = {
    sm: 'w-4 h-4 border-2',
    md: 'w-8 h-8 border-4',
    lg: 'w-12 h-12 border-4',
  };

  const textSizes = {
    sm: 'text-xs',
    md: 'text-sm',
    lg: 'text-base',
  };

  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center',
        fullScreen && 'h-screen',
        !fullScreen && 'py-12',
        className
      )}
    >
      <div
        className={cn(
          'border-primary-600 border-t-transparent rounded-full animate-spin',
          spinnerSizes[size]
        )}
      />
      {text && (
        <p className={cn('mt-3 text-gray-600', textSizes[size])}>
          {text}
        </p>
      )}
    </div>
  );
}

/**
 * 内联加载指示器（用于按钮等）
 */
export function InlineLoading({ size = 'sm' }: { size?: 'sm' | 'md' }) {
  const spinnerSizes = {
    sm: 'w-4 h-4 border-2',
    md: 'w-5 h-5 border-2',
  };

  return (
    <div
      className={cn(
        'border-white border-t-transparent rounded-full animate-spin',
        spinnerSizes[size]
      )}
    />
  );
}