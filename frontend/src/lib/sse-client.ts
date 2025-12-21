/**
 * Server-Sent Events (SSE) 流处理客户端
 * 统一处理所有 SSE 流式响应，避免代码重复
 */

import { API_CONFIG } from '@/config/constants';
import { logger, getErrorMessage } from './logger';

export interface SSEStreamOptions {
  /** 接收到内容块时的回调 */
  onChunk?: (content: string) => void;
  /** 接收到推理内容时的回调 */
  onReasoning?: (reasoning: string) => void;
  /** 接收到引用时的回调 */
  onReferences?: (references: any[]) => void;
  /** 发生错误时的回调 */
  onError?: (error: Error) => void;
  /** 流结束时的回调 */
  onEnd?: () => void;
  /** 超时时间（毫秒），默认 5 分钟 */
  timeout?: number;
}

/**
 * 处理 SSE 流式响应
 *
 * @param url - 请求 URL
 * @param data - 请求数据
 * @param options - 回调选项
 */
export async function handleSSEStream(
  url: string,
  data: any,
  options: SSEStreamOptions
): Promise<void> {
  const {
    onChunk,
    onReasoning,
    onReferences,
    onError,
    onEnd,
    timeout = API_CONFIG.STREAM_TIMEOUT,
  } = options;

  const controller = new AbortController();
  const timeoutId = setTimeout(() => {
    controller.abort();
    onError?.(new Error('请求超时'));
  }, timeout);

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
      signal: controller.signal,
    });

    if (!response.ok) {
      let errorMessage = `HTTP ${response.status}`;
      try {
        const errorData = await response.json();
        errorMessage = errorData.error || errorData.message || errorMessage;
      } catch {
        errorMessage = await response.text() || errorMessage;
      }
      throw new Error(errorMessage);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error('无法获取响应流');
    }

    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();

      if (done) {
        logger.debug('SSE stream completed');
        break;
      }

      // 解码数据块并添加到缓冲区
      buffer += decoder.decode(value, { stream: true });

      // 按行分割处理
      const lines = buffer.split('\n');

      // 保留最后一个不完整的行
      buffer = lines.pop() || '';

      for (const line of lines) {
        const trimmedLine = line.trim();

        if (!trimmedLine) {
          continue;
        }

        // 处理 "data:" 行
        if (trimmedLine.startsWith('data:')) {
          const jsonStr = trimmedLine.slice(5).trim(); // 移除 "data:" 前缀

          // 流结束标记
          if (jsonStr === '[DONE]') {
            logger.debug('SSE stream received [DONE]');
            continue;
          }

          try {
            const parsed = JSON.parse(jsonStr);

            // 处理内容块
            if (parsed.content && onChunk) {
              onChunk(parsed.content);
            }

            // 处理推理内容
            if (parsed.reasoning_content && onReasoning) {
              onReasoning(parsed.reasoning_content);
            }

            // 处理引用
            if (parsed.references && onReferences) {
              onReferences(parsed.references);
            }
          } catch (parseError) {
            logger.warn('Failed to parse SSE data:', jsonStr, parseError);
          }
        }
        // 处理 "documents:" 行（兼容旧格式）
        else if (trimmedLine.startsWith('documents:')) {
          const jsonStr = trimmedLine.slice(10).trim(); // 移除 "documents:" 前缀

          try {
            const parsed = JSON.parse(jsonStr);

            // 处理文档作为引用
            if (parsed.document && onReferences) {
              onReferences(parsed.document);
            }
          } catch (parseError) {
            logger.warn('Failed to parse SSE documents:', jsonStr, parseError);
          }
        }
      }
    }

    onEnd?.();
  } catch (error) {
    const errorMessage = getErrorMessage(error);
    logger.error('SSE stream error:', errorMessage);
    onError?.(error instanceof Error ? error : new Error(errorMessage));
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * 使用 FormData 的 SSE 流处理（用于文件上传）
 */
export async function handleSSEStreamWithFormData(
  url: string,
  formData: FormData,
  options: SSEStreamOptions
): Promise<void> {
  const {
    onChunk,
    onReasoning,
    onReferences,
    onError,
    onEnd,
    timeout = API_CONFIG.STREAM_TIMEOUT,
  } = options;

  const controller = new AbortController();
  const timeoutId = setTimeout(() => {
    controller.abort();
    onError?.(new Error('请求超时'));
  }, timeout);

  try {
    const response = await fetch(url, {
      method: 'POST',
      body: formData,
      signal: controller.signal,
    });

    if (!response.ok) {
      let errorMessage = `HTTP ${response.status}`;
      try {
        const errorData = await response.json();
        errorMessage = errorData.error || errorData.message || errorMessage;
      } catch {
        errorMessage = await response.text() || errorMessage;
      }
      throw new Error(errorMessage);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error('无法获取响应流');
    }

    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();

      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        const trimmedLine = line.trim();

        if (!trimmedLine) {
          continue;
        }

        // 处理 "data:" 行
        if (trimmedLine.startsWith('data:')) {
          const jsonStr = trimmedLine.slice(5).trim(); // 移除 "data:" 前缀

          // 流结束标记
          if (jsonStr === '[DONE]') {
            continue;
          }

          try {
            const parsed = JSON.parse(jsonStr);
            if (parsed.content && onChunk) onChunk(parsed.content);
            if (parsed.reasoning_content && onReasoning) onReasoning(parsed.reasoning_content);
            if (parsed.references && onReferences) onReferences(parsed.references);
          } catch (parseError) {
            logger.warn('Failed to parse SSE data:', jsonStr);
          }
        }
        // 处理 "documents:" 行（兼容旧格式）
        else if (trimmedLine.startsWith('documents:')) {
          const jsonStr = trimmedLine.slice(10).trim(); // 移除 "documents:" 前缀

          try {
            const parsed = JSON.parse(jsonStr);
            if (parsed.document && onReferences) {
              onReferences(parsed.document);
            }
          } catch (parseError) {
            logger.warn('Failed to parse SSE documents:', jsonStr);
          }
        }
      }
    }

    onEnd?.();
  } catch (error) {
    const errorMessage = getErrorMessage(error);
    logger.error('SSE stream error:', errorMessage);
    onError?.(error instanceof Error ? error : new Error(errorMessage));
  } finally {
    clearTimeout(timeoutId);
  }
}