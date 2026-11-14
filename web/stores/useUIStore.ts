'use client';

/**
 * dbdlp 控制台 · UI 切片（04 文档 §2.5）
 *
 * 职责：侧栏折叠、Command 面板、密度档、Toast 队列。
 * 持久化：会话存储（仅 UI 偏好）。密度档对应 §5.6 的三档令牌覆盖。
 */

import { create } from 'zustand';

export type Density = 'compact' | 'default' | 'comfortable';

export interface Toast {
  id: string;
  /** 中文提示文案 */
  message: string;
  tone: 'success' | 'error' | 'info' | 'warning';
}

const DENSITY_KEY = 'dbdlp.ui.density';
const SIDEBAR_KEY = 'dbdlp.ui.sidebar';

interface UIState {
  sidebarCollapsed: boolean;
  commandOpen: boolean;
  density: Density;
  /** 用户是否显式设置过密度（显式设置后不再被页面级默认覆盖） */
  densityExplicit: boolean;
  toasts: Toast[];
  toggleSidebar: () => void;
  setCommandOpen: (open: boolean) => void;
  setDensity: (density: Density) => void;
  pushToast: (message: string, tone?: Toast['tone']) => void;
  dismissToast: (id: string) => void;
}

/** 读取会话存储中的 UI 偏好（浏览器环境才可读） */
function readDensity(): Density {
  if (typeof window === 'undefined') return 'default';
  const stored = window.sessionStorage.getItem(DENSITY_KEY);
  return stored === 'compact' || stored === 'comfortable' ? stored : 'default';
}

function readSidebarCollapsed(): boolean {
  if (typeof window === 'undefined') return false;
  return window.sessionStorage.getItem(SIDEBAR_KEY) === '1';
}

export const useUIStore = create<UIState>((set, get) => ({
  sidebarCollapsed: readSidebarCollapsed(),
  commandOpen: false,
  density: readDensity(),
  densityExplicit: false,
  toasts: [],

  toggleSidebar: () => {
    const next = !get().sidebarCollapsed;
    if (typeof window !== 'undefined') {
      window.sessionStorage.setItem(SIDEBAR_KEY, next ? '1' : '0');
    }
    set({ sidebarCollapsed: next });
  },

  setCommandOpen: (open) => set({ commandOpen: open }),

  setDensity: (density) => {
    if (typeof window !== 'undefined') {
      window.sessionStorage.setItem(DENSITY_KEY, density);
    }
    // 同步到 <html data-density>，驱动 CSS 令牌覆盖（§5.6）
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-density', density);
    }
    set({ density, densityExplicit: true });
  },

  pushToast: (message, tone = 'info') => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    set({ toasts: [...get().toasts, { id, message, tone }] });
    // Toast 自动消失（≤ --motion-slow 后收起）
    if (typeof window !== 'undefined') {
      window.setTimeout(() => {
        useUIStore.getState().dismissToast(id);
      }, 4000);
    }
  },

  dismissToast: (id) => set({ toasts: get().toasts.filter((t) => t.id !== id) }),
}));
