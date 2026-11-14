'use client';

/**
 * dbdlp 控制台 · 系统配置切片（04 文档 §2.5，P12）
 *
 * 职责：配置回显、审计筛选、纯前端外观偏好。持久化：会话存储（仅偏好）。
 * 边界：不提供深色主题切换（§9），外观仅含密度档与侧栏折叠。
 */

import { create } from 'zustand';
import { listAuditLogs } from '@/services/system';
import { ApiError } from '@/services/http';
import type { AppConfig, AuditLog } from '@/types/api';

interface ConfigState {
  config: AppConfig | null;
  loading: boolean;
  error: string | null;
  auditLogs: AuditLog[];
  totalAuditLogs: number;
  auditActor: string;
  auditAction: string;
  auditPage: number;
  fetchConfig: () => Promise<void>;
  fetchAuditLogs: () => Promise<void>;
  setAuditActor: (actor: string) => void;
  setAuditAction: (action: string) => void;
  setAuditPage: (page: number) => void;
}

export const useConfigStore = create<ConfigState>((set, get) => ({
  config: null,
  loading: false,
  error: null,
  auditLogs: [],
  totalAuditLogs: 0,
  auditActor: '',
  auditAction: '',
  auditPage: 1,

  fetchConfig: async () => {
    set({ loading: true, error: null });
    try {
      const config = await getConfigSafe();
      set({ config, loading: false });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '配置加载失败，请稍后重试';
      set({ loading: false, error: message });
    }
  },

  fetchAuditLogs: async () => {
    const { auditActor, auditAction, auditPage } = get();
    try {
      const result = await listAuditLogs({
        page: auditPage,
        limit: 50,
        actor: auditActor || undefined,
        action: auditAction || undefined,
      });
      set({ auditLogs: result.items ?? [], totalAuditLogs: result.total ?? 0 });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '审计日志加载失败，请稍后重试';
      set({ error: message });
    }
  },

  setAuditActor: (auditActor) => set({ auditActor, auditPage: 1 }),
  setAuditAction: (auditAction) => set({ auditAction, auditPage: 1 }),
  setAuditPage: (auditPage) => set({ auditPage }),
}));

/** 读取生效配置（不含密钥与 DSN） */
async function getConfigSafe(): Promise<AppConfig> {
  const { getConfig } = await import('@/services/system');
  return getConfig();
}
