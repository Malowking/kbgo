/**
 * 统一的日志工具
 * 开发环境输出到控制台，生产环境可以扩展为上报到日志服务
 */

const isDev = import.meta.env.DEV;

export const logger = {
  /**
   * 信息日志 - 仅在开发环境输出
   */
  info: (...args: any[]) => {
    if (isDev) {
      console.log('[INFO]', ...args);
    }
  },

  /**
   * 警告日志 - 仅在开发环境输出
   */
  warn: (...args: any[]) => {
    if (isDev) {
      console.warn('[WARN]', ...args);
    }
  },

  /**
   * 错误日志 - 开发环境输出，生产环境可以上报
   */
  error: (...args: any[]) => {
    if (isDev) {
      console.error('[ERROR]', ...args);
    }

    // 生产环境错误上报
    // 在生产环境中应该配置错误上报服务，例如：
    // 1. Sentry: Sentry.captureException(args[0]);
    // 2. 其他监控服务: trackError(args[0]);
    //
    // 示例配置:
    // if (!isDev && typeof window !== 'undefined') {
    //   try {
    //     if (args[0] instanceof Error) {
    //       // Sentry.captureException(args[0]);
    //     } else {
    //       // Sentry.captureMessage(String(args[0]), 'error');
    //     }
    //   } catch (reportError) {
    //     // 错误上报失败不应影响主流程
    //     console.warn('Failed to report error:', reportError);
    //   }
    // }
  },

  /**
   * 调试日志 - 仅在开发环境输出
   */
  debug: (...args: any[]) => {
    if (isDev) {
      console.debug('[DEBUG]', ...args);
    }
  },
};

/**
 * 从未知类型的错误对象中提取错误消息
 * 类型安全的错误处理
 */
export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }

  if (typeof error === 'string') {
    return error;
  }

  if (error && typeof error === 'object' && 'message' in error) {
    return String(error.message);
  }

  return '未知错误';
}

/**
 * 格式化错误对象为可读字符串
 */
export function formatError(error: unknown): string {
  const message = getErrorMessage(error);

  if (error instanceof Error && error.stack && isDev) {
    return `${message}\n${error.stack}`;
  }

  return message;
}