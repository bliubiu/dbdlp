'use client';

/**
 * dbdlp 控制台 · 报告切片（04 文档 §2.5，P09 / P10）
 *
 * 职责：报告列表、生成轮询、下载状态。持久化：否。
 */

import { create } from 'zustand';
import { downloadReport, generateReport, listReports } from '@/services/system';
import { ApiError } from '@/services/http';
import type { ReportArtifact, ReportFormat } from '@/types/api';

interface ReportState {
  items: ReportArtifact[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  /** 生成中的报告（可离开页面，回 P09 后轮询恢复） */
  generating: boolean;
  generatingTaskId: string | null;
  generatingFormats: ReportFormat[];
  notice: string | null;
  downloadingId: string | null;
  fetchList: (query?: { taskId?: string; page?: number; limit?: number }) => Promise<void>;
  generate: (taskId: string, formats: ReportFormat[]) => Promise<boolean>;
  download: (id: string, fileName: string) => Promise<void>;
  setPage: (page: number) => void;
  clearNotice: () => void;
}

export const useReportStore = create<ReportState>((set, get) => ({
  items: [],
  total: 0,
  page: 1,
  limit: 20,
  hasMore: false,
  loading: false,
  error: null,
  generating: false,
  generatingTaskId: null,
  generatingFormats: [],
  notice: null,
  downloadingId: null,

  fetchList: async (query) => {
    const state = get();
    const next = {
      taskId: query?.taskId,
      page: query?.page ?? state.page,
      limit: query?.limit ?? state.limit,
    };
    set({ loading: true, error: null });
    try {
      const result = await listReports(next);
      set({
        items: result.items ?? [],
        total: result.total ?? 0,
        page: result.page ?? next.page ?? 1,
        limit: result.limit ?? next.limit ?? 20,
        hasMore: result.hasMore ?? false,
        loading: false,
      });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '报告列表加载失败，请稍后重试';
      set({ loading: false, error: message, items: [] });
    }
  },

  generate: async (taskId, formats) => {
    if (!taskId) {
      set({ notice: '请先选择要生成报告的扫描任务' });
      return false;
    }
    if (formats.length === 0) {
      set({ notice: '请至少选择一种报告格式' });
      return false;
    }
    set({ generating: true, generatingTaskId: taskId, generatingFormats: formats, notice: null });
    try {
      await generateReport({ taskId, formats });
      set({ generating: false, generatingTaskId: null, generatingFormats: [] });
      await get().fetchList();
      set({ notice: '报告已生成，可在本页下载；生成中可离开本页' });
      return true;
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '报告生成失败，请稍后重试';
      set({ generating: false, generatingTaskId: null, generatingFormats: [], notice: message });
      return false;
    }
  },

  download: async (id, fileName) => {
    set({ downloadingId: id, notice: null });
    try {
      const blob = await downloadReport(id);
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      set({ downloadingId: null, notice: '报告已开始下载，样例均为掩码值' });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '报告下载失败，请稍后重试';
      set({ downloadingId: null, notice: message });
    }
  },

  setPage: (page) => set({ page }),
  clearNotice: () => set({ notice: null }),
}));
