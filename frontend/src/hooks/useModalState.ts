/**
 * Modal 状态管理 Hook
 * 统一管理 Modal 的打开/关闭和编辑项状态
 */

import { useState, useCallback } from 'react';

export interface ModalState<T = any> {
  /** Modal 是否打开 */
  isOpen: boolean;
  /** 当前编辑的项（如果有） */
  editingItem: T | null;
  /** 打开 Modal */
  open: (item?: T) => void;
  /** 关闭 Modal */
  close: () => void;
  /** 是否为编辑模式 */
  isEditing: boolean;
}

/**
 * 使用 Modal 状态管理
 *
 * @template T - 编辑项的类型
 * @returns Modal 状态和操作方法
 *
 * @example
 * ```typescript
 * const modal = useModalState<KnowledgeBase>();
 *
 * // 打开创建 Modal
 * <button onClick={() => modal.open()}>创建</button>
 *
 * // 打开编辑 Modal
 * <button onClick={() => modal.open(item)}>编辑</button>
 *
 * // 渲染 Modal
 * {modal.isOpen && (
 *   <CreateModal
 *     item={modal.editingItem}
 *     onClose={modal.close}
 *   />
 * )}
 * ```
 */
export function useModalState<T = any>(): ModalState<T> {
  const [isOpen, setIsOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<T | null>(null);

  const open = useCallback((item?: T) => {
    setEditingItem(item || null);
    setIsOpen(true);
  }, []);

  const close = useCallback(() => {
    setIsOpen(false);
    // 延迟清除编辑项，等待 Modal 关闭动画完成
    setTimeout(() => {
      setEditingItem(null);
    }, 300);
  }, []);

  const isEditing = !!editingItem;

  return {
    isOpen,
    editingItem,
    open,
    close,
    isEditing,
  };
}