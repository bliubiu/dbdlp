'use client';

/**
 * dbdlp 控制台 · 认证切片（04 文档 §2.5）
 *
 * 职责：token / expiresAt / displayName / 登录态。
 * 持久化：否（仅内存会话 + sessionStorage 令牌）。
 * 边界：单管理员 + 会话，不做 SSO / LDAP（04 文档 §2.2、U-08）。
 */

import { create } from 'zustand';
import { fetchCurrentUser, login as loginRequest, logout as logoutRequest } from '@/services/auth';
import { ApiError, clearToken, setToken, setUnauthorizedHandler } from '@/services/http';

interface AuthState {
  token: string | null;
  expiresAt: string | null;
  displayName: string | null;
  /** 登录中 */
  pending: boolean;
  /** 中文错误信息（登录失败 / 会话失效） */
  error: string | null;
  /** 已确认服务端会话有效 */
  checked: boolean;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => Promise<void>;
  /** 校验当前登录态（进入受保护页时调用） */
  checkSession: () => Promise<boolean>;
  /** 供 http 层在 1002 时调用 */
  invalidate: (message?: string) => void;
  clearError: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  expiresAt: null,
  displayName: null,
  pending: false,
  error: null,
  checked: false,

  login: async (username, password) => {
    set({ pending: true, error: null });
    try {
      const result = await loginRequest({ username, password });
      setToken(result.token);
      set({
        token: result.token,
        expiresAt: result.expiresAt,
        displayName: result.displayName,
        pending: false,
        error: null,
        checked: true,
      });
      return true;
    } catch (error) {
      // 只回显后端中文 message，不改写为英文、不暴露堆栈
      const message =
        error instanceof ApiError ? error.message : '登录失败，请稍后重试';
      set({ pending: false, error: message, token: null, expiresAt: null });
      return false;
    }
  },

  logout: async () => {
    try {
      await logoutRequest();
    } catch {
      /* 登出请求失败也清理本地会话，避免残留态 */
    }
    clearToken();
    set({ token: null, expiresAt: null, displayName: null, error: null, checked: true });
  },

  checkSession: async () => {
    if (get().token === null && typeof window !== 'undefined') {
      // 令牌在 sessionStorage，刷新后从存储恢复
      const stored = (await import('@/services/http')).getToken();
      if (stored) {
        set({ token: stored });
      } else {
        set({ checked: true });
        return false;
      }
    }
    try {
      const user = await fetchCurrentUser();
      set({ displayName: user.username, checked: true, error: null });
      return true;
    } catch (error) {
      if (error instanceof ApiError && error.isUnauthorized) {
        clearToken();
        set({ token: null, expiresAt: null, checked: true, error: error.message });
      } else {
        set({ checked: true });
      }
      return false;
    }
  },

  invalidate: (message) => {
    clearToken();
    set({
      token: null,
      expiresAt: null,
      displayName: null,
      checked: true,
      error: message ?? '登录状态已失效，请重新登录',
    });
  },

  clearError: () => set({ error: null }),
}));

/** 注册 1002 全局回调：会话失效时统一清理并跳转 /login */
if (typeof window !== 'undefined') {
  setUnauthorizedHandler(() => {
    useAuthStore.getState().invalidate('登录状态已失效，请重新登录');
  });
}
