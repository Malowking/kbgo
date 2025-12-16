import { create } from 'zustand';
import type { KnowledgeBase, Conversation, Model } from '@/types';

interface AppStore {
  // 当前选中的知识库
  currentKB: KnowledgeBase | null;
  setCurrentKB: (kb: KnowledgeBase | null) => void;

  // 当前会话
  currentConversation: Conversation | null;
  setCurrentConversation: (conv: Conversation | null) => void;

  // 模型列表
  models: Model[];
  setModels: (models: Model[]) => void;

  // 侧边栏状态
  sidebarOpen: boolean;
  toggleSidebar: () => void;
}

export const useAppStore = create<AppStore>((set) => ({
  currentKB: null,
  setCurrentKB: (kb) => set({ currentKB: kb }),

  currentConversation: null,
  setCurrentConversation: (conv) => set({ currentConversation: conv }),

  models: [],
  setModels: (models) => set({ models }),

  sidebarOpen: true,
  toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),
}));