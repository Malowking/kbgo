/**
 * Server-Sent Events (SSE) 流处理客户端
 * 统一处理所有 SSE 流式响应，避免代码重复
 */

import { API_CONFIG } from '@/config/constants';
import { logger, getErrorMessage } from './logger';

/** 工具调用信息 */
export interface ToolCallInfo {
  /** 工具调用ID */
  tool_id: string;
  /** 工具名称 */
  tool_name: string;
  /** 工具状态 */
  status: 'pending' | 'running' | 'success' | 'error';
  /** 工具参数 */
  arguments?: any;
  /** 工具结果 */
  result?: string;
  /** 错误信息 */
  error?: string;
  /** 开始时间 */
  start_time?: number;
  /** 结束时间 */
  end_time?: number;
  /** 执行时长（毫秒） */
  duration?: number;
}

/** LLM迭代信息 */
export interface LLMIterationInfo {
  /** 当前迭代轮次 */
  iteration: number;
  /** 最大迭代轮次 */
  max_iterations: number;
  /** 迭代消息 */
  message: string;
}

export interface SSEStreamOptions {
  /** 接收到内容块时的回调 */
  onChunk?: (content: string) => void;
  /** 接收到推理内容时的回调 */
  onReasoning?: (reasoning: string) => void;
  /** 接收到引用时的回调 */
  onReferences?: (references: any[]) => void;
  /** 工具调用开始时的回调 */
  onToolCallStart?: (toolCall: ToolCallInfo) => void;
  /** 工具调用结束时的回调 */
  onToolCallEnd?: (toolCall: ToolCallInfo) => void;
  /** LLM迭代时的回调 */
  onLLMIteration?: (iteration: LLMIterationInfo) => void;
  /** 思考过程时的回调 */
  onThinking?: (thinking: string) => void;
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
    onToolCallStart,
    onToolCallEnd,
    onLLMIteration,
    onThinking,
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

            // 根据事件类型分发处理
            const eventType = parsed.type || 'content';

            switch (eventType) {
              case 'tool_call_start':
                // 工具调用开始事件
                if (onToolCallStart && parsed.metadata) {
                  const toolCall: ToolCallInfo = {
                    tool_id: parsed.metadata.tool_id,
                    tool_name: parsed.metadata.tool_name,
                    status: 'running',
                    arguments: parsed.metadata.arguments,
                    start_time: parsed.created * 1000, // 转换为毫秒
                  };
                  onToolCallStart(toolCall);
                  logger.debug('Tool call started:', toolCall);
                }
                break;

              case 'tool_call_end':
                // 工具调用结束事件
                if (onToolCallEnd && parsed.metadata) {
                  const toolCall: ToolCallInfo = {
                    tool_id: parsed.metadata.tool_id,
                    tool_name: parsed.metadata.tool_name,
                    status: parsed.metadata.error ? 'error' : 'success',
                    result: parsed.metadata.result,
                    error: parsed.metadata.error,
                    end_time: parsed.created * 1000,
                    duration: parsed.metadata.duration_ms,
                  };
                  onToolCallEnd(toolCall);
                  logger.debug('Tool call ended:', toolCall);
                }
                break;

              case 'llm_iteration':
                // LLM迭代事件
                if (onLLMIteration && parsed.metadata) {
                  const iteration: LLMIterationInfo = {
                    iteration: parsed.metadata.iteration,
                    max_iterations: parsed.metadata.max_iterations,
                    message: parsed.metadata.message,
                  };
                  onLLMIteration(iteration);
                  logger.debug('LLM iteration:', iteration);
                }
                break;

              case 'thinking':
                // 思考过程事件
                if (onThinking && parsed.content) {
                  onThinking(parsed.content);
                  logger.debug('Thinking:', parsed.content);
                }
                break;

              case 'content':
              default:
                // 普通内容块
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
                break;
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
    onToolCallStart,
    onToolCallEnd,
    onLLMIteration,
    onThinking,
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

            // 根据事件类型分发处理（与 handleSSEStream 相同的逻辑）
            const eventType = parsed.type || 'content';

            switch (eventType) {
              case 'tool_call_start':
                if (onToolCallStart && parsed.metadata) {
                  const toolCall: ToolCallInfo = {
                    tool_id: parsed.metadata.tool_id,
                    tool_name: parsed.metadata.tool_name,
                    status: 'running',
                    arguments: parsed.metadata.arguments,
                    start_time: parsed.created * 1000,
                  };
                  onToolCallStart(toolCall);
                }
                break;

              case 'tool_call_end':
                if (onToolCallEnd && parsed.metadata) {
                  const toolCall: ToolCallInfo = {
                    tool_id: parsed.metadata.tool_id,
                    tool_name: parsed.metadata.tool_name,
                    status: parsed.metadata.error ? 'error' : 'success',
                    result: parsed.metadata.result,
                    error: parsed.metadata.error,
                    end_time: parsed.created * 1000,
                    duration: parsed.metadata.duration_ms,
                  };
                  onToolCallEnd(toolCall);
                }
                break;

              case 'llm_iteration':
                if (onLLMIteration && parsed.metadata) {
                  const iteration: LLMIterationInfo = {
                    iteration: parsed.metadata.iteration,
                    max_iterations: parsed.metadata.max_iterations,
                    message: parsed.metadata.message,
                  };
                  onLLMIteration(iteration);
                }
                break;

              case 'thinking':
                if (onThinking && parsed.content) {
                  onThinking(parsed.content);
                }
                break;

              case 'content':
              default:
                if (parsed.content && onChunk) onChunk(parsed.content);
                if (parsed.reasoning_content && onReasoning) onReasoning(parsed.reasoning_content);
                if (parsed.references && onReferences) onReferences(parsed.references);
                break;
            }
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