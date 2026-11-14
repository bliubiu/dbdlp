'use client';

/**
 * dbdlp 控制台 · 扫描任务详情切片（04 文档 §2.5，P06）
 *
 * 职责：进度快照、Tab、重试结果。持久化：否。
 */

import { create } from 'zustand';
import { getTask, getTaskProgress, pauseTask, resumeTask, retryTask } from '@/services/tasks';
import { ApiError } from '@/services/http';
import { isTerminalStatus } from '@/lib/format';
import type { ScanTask, TaskProgress } from '@/types/api';

export type TaskDetailTab = 'progress' | 'failed' | 'audit' | 'params';

interface TaskDetailState {
  task: ScanTask | null;
  progress: TaskProgress | null;
  loading: boolean;
  error: string | null;
  /** 操作级中文提示（成功/失败） */
  notice: string | null;
  tab: TaskDetailTab;
  /** 轮询句柄 */
  pollTimer: ReturnType<typeof setTimeout> | null;
  load: (id: string) => Promise<void>;
  refreshProgress: (id: string) => Promise<void>;
  setTab: (tab: TaskDetailTab) => void;
  pause: (id: string) => Promise<void>;
  resume: (id: string) => Promise<void>;
  retry: (id: string, targetIds: string[]) => Promise<void>;
  startPolling: (id: string) => void;
  stopPolling: () => void;
}

export const useTaskDetailStore = create<TaskDetailState>((set, get) => ({
  task: null,
  progress: null,
  loading: false,
  error: null,
  notice: null,
  tab: 'progress',
  pollTimer: null,

  load: async (id) => {
    set({ loading: true, error: null });
    try {
      const [task, progress] = await Promise.all([
        getTask(id),
        getTaskProgress(id),
      ]);
      set({ task, progress, loading: false });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '任务详情加载失败，请稍后重试';
      set({ loading: false, error: message, task: null, progress: null });
    }
  },

  refreshProgress: async (id) => {
    try {
      const progress = await getTaskProgress(id);
      set({ progress });
    } catch {
      /* 静默：轮询失败不打断页面 */
    }
  },

  setTab: (tab) => set({ tab }),

  pause: async (id) => {
    try {
      const result = await pauseTask(id);
      set({ notice: `任务已${result.status === 'paused' ? '暂停' : '更新状态'}` });
      await get().load(id);
    } catch (error) {
      set({ notice: error instanceof ApiError ? error.message : '暂停失败，请稍后重试' });
    }
  },

  resume: async (id) => {
    try {
      const result = await resumeTask(id);
      set({ notice: `任务已${result.status === 'running' ? '继续执行' : '更新状态'}` });
      await get().load(id);
    } catch (error) {
      set({ notice: error instanceof ApiError ? error.message : '继续执行失败，请稍后重试' });
    }
  },

  retry: async (id, targetIds) => {
    try {
      const result = await retryTask(id, targetIds);
      set({
        notice: `已重扫 ${result.retried} 张失败表，新增 ${result.newFindings} 条命中，剩余 ${result.remaining} 张失败表`,
      });
      await get().load(id);
    } catch (error) {
      set({ notice: error instanceof ApiError ? error.message : '重试失败，请稍后重试' });
    }
  },

  startPolling: (id) => {
    get().stopPolling();
    const tick = async () => {
      await get().refreshProgress(id);
      const status = get().progress?.status;
      if (status && isTerminalStatus(status)) {
        get().stopPolling();
        return;
      }
      const timer = setTimeout(tick, 3000);
      set({ pollTimer: timer });
    };
    const timer = setTimeout(tick, 3000);
    set({ pollTimer: timer });
  },

  stopPolling: () => {
    const timer = get().pollTimer;
    if (timer) clearTimeout(timer);
    set({ pollTimer: null });
  },
}));
