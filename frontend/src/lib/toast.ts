/**
 * Toast 通知助手
 * 封装 react-hot-toast，提供统一的通知接口
 */

import toast from 'react-hot-toast';

/**
 * 显示成功通知
 */
export function showSuccess(message: string) {
  return toast.success(message, {
    duration: 3000,
    position: 'top-right',
  });
}

/**
 * 显示错误通知
 */
export function showError(message: string) {
  return toast.error(message, {
    duration: 4000,
    position: 'top-right',
  });
}

/**
 * 显示信息通知
 */
export function showInfo(message: string) {
  return toast(message, {
    duration: 3000,
    position: 'top-right',
    icon: 'ℹ️',
  });
}

/**
 * 显示警告通知
 */
export function showWarning(message: string) {
  return toast(message, {
    duration: 3500,
    position: 'top-right',
    icon: '⚠️',
  });
}

/**
 * 显示加载通知
 */
export function showLoading(message: string) {
  return toast.loading(message, {
    position: 'top-right',
  });
}

/**
 * 关闭指定的通知
 */
export function dismissToast(toastId: string) {
  toast.dismiss(toastId);
}

/**
 * 关闭所有通知
 */
export function dismissAllToasts() {
  toast.dismiss();
}

/**
 * Promise 通知 - 自动处理加载/成功/失败状态
 */
export function toastPromise<T>(
  promise: Promise<T>,
  messages: {
    loading: string;
    success: string | ((data: T) => string);
    error: string | ((err: any) => string);
  }
) {
  return toast.promise(
    promise,
    {
      loading: messages.loading,
      success: messages.success,
      error: messages.error,
    },
    {
      position: 'top-right',
    }
  );
}