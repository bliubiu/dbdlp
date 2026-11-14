'use client';

/**
 * dbdlp 控制台 · 扫描任务列表切片（04 文档 §2.5，P05）
 *
 * 职责：任务列表、筛选、轮询句柄、新建草稿。持久化：否。
 */

import { create } from 'zustand';
import { createTask, listTasks, type TaskQuery } from '@/services/tasks';
import { ApiError } from '@/services/http';
import { isTerminalStatus } from '@/lib/format';
import type { ScanTask } from '@/types/api';

interface TaskState {
  items: ScanTask[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  /** 筛选 */
  status: string;
  dataSourceId: string;
  /** 新建任务草稿 */
  draft: {
    dataSourceId: string;
    sampleRows: number;
    batchSize: number;
    concurrency: number;
    lowPeakWindow: string;
  };
  /** 轮询句柄（组件卸载时清理） */
  pollTimer: ReturnType<typeof setTimeout> | null;
  fetchList: (query?: Partial<TaskQuery>) => Promise<void>;
  setStatus: (status: string) => void;
  setDataSourceId: (dataSourceId: string) => void;
  setPage: (page: number) => void;
  setDraft: (draft: Partial<TaskState['draft']>) => void;
  submitDraft: () => Promise<string | null>;
  startPolling: (taskId: string) => void;
  stopPolling: () => void;
}

export const useTaskStore = create<TaskState>((set, get) => ({
  items: [],
  total: 0,
  page: 1,
  limit: 20,
  hasMore: false,
  loading: false,
  error: null,
  status: '',
  dataSourceId: '',
  draft: {
    dataSourceId: '',
    sampleRows: 20,
    batchSize: 200,
    concurrency: 2,
    lowPeakWindow: '',
  },
  pollTimer: null,

  fetchList: async (query) => {
    const state = get();
    const next = {
      page: query?.page ?? state.page,
      limit: query?.limit ?? state.limit,
      status: query?.status ?? state.status,
      dataSourceId: query?.dataSourceId ?? state.dataSourceId,
    };
    set({ loading: true, error: null });
    try {
      const result = await listTasks(next);
      set({
        items: result.items ?? [],
        total: result.total ?? 0,
        page: result.page ?? next.page,
        limit: result.limit ?? next.limit,
        hasMore: result.hasMore ?? false,
        loading: false,
      });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '任务列表加载失败，请稍后重试';
      set({ loading: false, error: message, items: [] });
    }
  },

  setStatus: (status) => set({ status, page: 1 }),
  setDataSourceId: (dataSourceId) => set({ dataSourceId, page: 1 }),
  setPage: (page) => set({ page }),

  setDraft: (draft) => set({ draft: { ...get().draft, ...draft } }),

  submitDraft: async () => {
    const draft = get().draft;
    if (!draft.dataSourceId) {
      return null;
    }
    try {
      const result = await createTask({
        dataSourceId: draft.dataSourceId,
        sampleRows: draft.sampleRows,
        batchSize: draft.batchSize,
        concurrency: draft.concurrency,
        lowPeakWindow: draft.lowPeakWindow || undefined,
      });
      await get().fetchList();
      return result.taskId;
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '任务发起失败，请稍后重试';
      set({ error: message });
      return null;
    }
  },

  startPolling: (taskId) => {
    get().stopPolling();
    const tick = async () => {
      try {
        await get().fetchList();
        const current = get().items.find((t) => t.id === taskId);
        // 任务进入终态则停止轮询
        if (current && isTerminalStatus(current.status)) {
          get().stopPolling();
          return;
        }
      } catch {
        /* 轮询失败静默重试，不打断页面 */
      }
      const timer = setTimeout(tick, 5000);
      set({ pollTimer: timer });
    };
    const timer = setTimeout(tick, 5000);
    set({ pollTimer: timer });
  },

  stopPolling: () => {
    const timer = get().pollTimer;
    if (timer) clearTimeout(timer);
    set({ pollTimer: null });
  },
}));
