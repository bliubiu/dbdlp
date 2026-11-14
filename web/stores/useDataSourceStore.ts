'use client';

/**
 * dbdlp 控制台 · 数据源列表切片（04 文档 §2.5，P02）
 *
 * 职责：列表、选中、筛选、分页、重测结果。持久化：否。
 */

import { create } from 'zustand';
import {
  deleteDataSource,
  listDataSources,
  testDataSource,
  type DataSourceQuery,
} from '@/services/datasources';
import { ApiError } from '@/services/http';
import type { DataSource } from '@/types/api';

interface DataSourceState {
  items: DataSource[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  /** 筛选条件 */
  keyword: string;
  dialect: string;
  sort: string;
  /** 选中的数据源 id 集合（批量操作栏） */
  selected: string[];
  /** 最近一次重测结果（id → 结果） */
  testResults: Record<string, { ok: boolean; message: string; latencyMs: number }>;
  fetchList: (query?: Partial<DataSourceQuery>) => Promise<void>;
  setKeyword: (keyword: string) => void;
  setDialect: (dialect: string) => void;
  setPage: (page: number) => void;
  toggleSelected: (id: string) => void;
  setSelected: (ids: string[]) => void;
  clearSelected: () => void;
  testOne: (id: string) => Promise<void>;
  removeOne: (id: string) => Promise<string | null>;
}

export const useDataSourceStore = create<DataSourceState>((set, get) => ({
  items: [],
  total: 0,
  page: 1,
  limit: 20,
  hasMore: false,
  loading: false,
  error: null,
  keyword: '',
  dialect: '',
  sort: '',
  selected: [],
  testResults: {},

  fetchList: async (query) => {
    const state = get();
    const next = {
      page: query?.page ?? state.page,
      limit: query?.limit ?? state.limit,
      dialect: query?.dialect ?? state.dialect,
      keyword: query?.keyword ?? state.keyword,
      sort: query?.sort ?? state.sort,
    };
    set({ loading: true, error: null });
    try {
      const result = await listDataSources(next);
      set({
        items: result.items ?? [],
        total: result.total ?? 0,
        page: result.page ?? next.page,
        limit: result.limit ?? next.limit,
        hasMore: result.hasMore ?? false,
        loading: false,
      });
    } catch (error) {
      const message =
        error instanceof ApiError ? error.message : '数据源列表加载失败，请稍后重试';
      set({ loading: false, error: message, items: [] });
    }
  },

  setKeyword: (keyword) => set({ keyword, page: 1 }),
  setDialect: (dialect) => set({ dialect, page: 1 }),
  setPage: (page) => set({ page }),

  toggleSelected: (id) => {
    const selected = get().selected;
    set({
      selected: selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id],
    });
  },

  setSelected: (ids) => set({ selected: ids }),
  clearSelected: () => set({ selected: [] }),

  testOne: async (id) => {
    try {
      const result = await testDataSource(id);
      set({
        testResults: {
          ...get().testResults,
          [id]: { ok: result.ok, message: result.message, latencyMs: result.latencyMs },
        },
      });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '连接测试失败，请稍后重试';
      set({
        testResults: {
          ...get().testResults,
          [id]: { ok: false, message, latencyMs: 0 },
        },
      });
    }
  },

  removeOne: async (id) => {
    try {
      await deleteDataSource(id);
      set({
        items: get().items.filter((item) => item.id !== id),
        total: Math.max(0, get().total - 1),
        selected: get().selected.filter((x) => x !== id),
      });
      return null;
    } catch (error) {
      // 原样回显后端中文 message（如「存在运行中任务，无法删除」）
      return error instanceof ApiError ? error.message : '删除失败，请稍后重试';
    }
  },
}));
