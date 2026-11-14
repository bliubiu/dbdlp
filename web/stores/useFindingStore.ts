'use client';

/**
 * dbdlp 控制台 · 结果台账切片（04 文档 §2.5、P07）
 *
 * 职责：筛选、选中、列显隐与顺序、排序、分页、密度。持久化：会话存储。
 *
 * 掩码约束：仅消费 sampleMasked，不承载任何明文（§4.6）。
 */

import { create } from 'zustand';
import { listFindings, type FindingQuery } from '@/services/findings';
import { ApiError } from '@/services/http';
import type { ColumnFinding } from '@/types/api';

/** 排序方向 */
export type SortOrder = 'asc' | 'desc';

const VIEW_KEY = 'dbdlp.findings.view';

interface FindingView {
  keyword: string;
  riskLevel: string;
  reviewStatus: string;
  taskId: string;
  sort: string;
  order: SortOrder;
  page: number;
  limit: number;
  density: 'compact' | 'default' | 'comfortable';
  /** 隐藏的列 key 集合 */
  hiddenColumns: string[];
  /** 列顺序 */
  columnOrder: string[];
}

/** 台账列定义（§4.3，18 列；accessorKey 与后端字段一一对应） */
export const FINDING_COLUMNS = [
  { id: 'select', label: '选择框', canSort: false, canFilter: false },
  { id: 'instanceId', label: '实例', canSort: true, canFilter: true },
  { id: 'schemaName', label: '模式', canSort: true, canFilter: true },
  { id: 'tableName', label: '表名', canSort: true, canFilter: true },
  { id: 'columnName', label: '字段名', canSort: true, canFilter: true },
  { id: 'columnComment', label: '字段注释', canSort: false, canFilter: false },
  { id: 'dataType', label: '字段类型', canSort: true, canFilter: true },
  { id: 'sensitiveTypes', label: '敏感类型', canSort: false, canFilter: true },
  { id: 'hitRatio', label: '命中率', canSort: true, canFilter: true },
  { id: 'sampleHit', label: '命中/样本', canSort: true, canFilter: false },
  { id: 'verifyPassed', label: 'Verify', canSort: false, canFilter: true },
  { id: 'confidence', label: '置信度', canSort: true, canFilter: true },
  { id: 'storageForm', label: '存储形态', canSort: false, canFilter: true },
  { id: 'riskLevel', label: '风险等级', canSort: true, canFilter: true },
  { id: 'reviewStatus', label: '复核状态', canSort: true, canFilter: true },
  { id: 'sampleMasked', label: '掩码样例', canSort: false, canFilter: false },
  { id: 'updatedAt', label: '最后扫描', canSort: true, canFilter: true },
  { id: 'actions', label: '操作', canSort: false, canFilter: false },
] as const;

/** 默认显示的列（§4.3：默认 1~5、8~9、13~14、15~17 列，其余由列显隐开启） */
export const DEFAULT_VISIBLE_COLUMNS = [
  'select',
  'instanceId',
  'schemaName',
  'tableName',
  'columnName',
  'sensitiveTypes',
  'hitRatio',
  'confidence',
  'storageForm',
  'riskLevel',
  'reviewStatus',
  'sampleMasked',
  'updatedAt',
];

/** 读取会话存储中的台账视图配置 */
function readView(): Partial<FindingView> {
  if (typeof window === 'undefined') return {};
  try {
    const raw = window.sessionStorage.getItem(VIEW_KEY);
    if (!raw) return {};
    return JSON.parse(raw) as Partial<FindingView>;
  } catch {
    return {};
  }
}

interface FindingState {
  items: ColumnFinding[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  keyword: string;
  riskLevel: string;
  reviewStatus: string;
  taskId: string;
  sort: string;
  order: SortOrder;
  density: 'compact' | 'default' | 'comfortable';
  hiddenColumns: string[];
  columnOrder: string[];
  selected: string[];
  fetchList: (query?: Partial<FindingQuery>) => Promise<void>;
  setKeyword: (keyword: string) => void;
  setRiskLevel: (riskLevel: string) => void;
  setReviewStatus: (reviewStatus: string) => void;
  setTaskId: (taskId: string) => void;
  setSort: (sort: string) => void;
  toggleSort: (column: string) => void;
  setPage: (page: number) => void;
  setLimit: (limit: number) => void;
  setDensity: (density: 'compact' | 'default' | 'comfortable') => void;
  toggleColumn: (columnId: string) => void;
  resetColumns: () => void;
  setColumnOrder: (order: string[]) => void;
  toggleSelected: (id: string) => void;
  setSelected: (ids: string[]) => void;
  clearSelected: () => void;
}

const initialView = readView();

export const useFindingStore = create<FindingState>((set, get) => ({
  items: [],
  total: 0,
  page: 1,
  limit: 50,
  hasMore: false,
  loading: false,
  error: null,
  keyword: initialView.keyword ?? '',
  riskLevel: initialView.riskLevel ?? '',
  reviewStatus: initialView.reviewStatus ?? '',
  taskId: initialView.taskId ?? '',
  sort: initialView.sort ?? 'updatedAt',
  order: initialView.order ?? 'desc',
  density: initialView.density ?? 'compact',
  hiddenColumns: initialView.hiddenColumns ?? FINDING_COLUMNS.map((c) => c.id).filter(
    (id) => !DEFAULT_VISIBLE_COLUMNS.includes(id),
  ),
  columnOrder: initialView.columnOrder ?? FINDING_COLUMNS.map((c) => c.id),
  selected: [],

  fetchList: async (query) => {
    const state = get();
    const next: FindingQuery = {
      taskId: query?.taskId ?? state.taskId,
      riskLevel: query?.riskLevel ?? state.riskLevel,
      reviewStatus: query?.reviewStatus ?? state.reviewStatus,
      keyword: query?.keyword ?? state.keyword,
      page: query?.page ?? state.page,
      limit: query?.limit ?? state.limit,
      sort: query?.sort ?? state.sort,
      order: query?.order ?? state.order,
    };
    set({ loading: true, error: null });
    try {
      const result = await listFindings(next);
      set({
        items: result.items ?? [],
        total: result.total ?? 0,
        page: result.page ?? next.page ?? 1,
        limit: result.limit ?? next.limit ?? 50,
        hasMore: result.hasMore ?? false,
        loading: false,
      });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '台账加载失败，请稍后重试';
      set({ loading: false, error: message, items: [] });
    }
  },

  setKeyword: (keyword) => set({ keyword, page: 1 }),
  setRiskLevel: (riskLevel) => set({ riskLevel, page: 1 }),
  setReviewStatus: (reviewStatus) => set({ reviewStatus, page: 1 }),
  setTaskId: (taskId) => set({ taskId, page: 1 }),
  setPage: (page) => set({ page }),
  setLimit: (limit) => set({ limit, page: 1 }),
  setDensity: (density) => {
    set({ density });
    persist(get());
    // 同步全局密度档（写入 <html data-density>），使台账行高与页面其它表格一致
    void import('@/stores/useUIStore').then(({ useUIStore }) => {
      useUIStore.getState().setDensity(density);
    });
  },

  setSort: (sort) => set({ sort, page: 1 }),

  toggleSort: (column) => {
    const state = get();
    const order: SortOrder = state.sort === column && state.order === 'asc' ? 'desc' : 'asc';
    set({ sort: column, order, page: 1 });
  },

  toggleColumn: (columnId) => {
    const hidden = get().hiddenColumns.includes(columnId)
      ? get().hiddenColumns.filter((id) => id !== columnId)
      : [...get().hiddenColumns, columnId];
    set({ hiddenColumns: hidden });
    persist(get());
  },

  resetColumns: () => {
    set({
      hiddenColumns: FINDING_COLUMNS.map((c) => c.id).filter(
        (id) => !DEFAULT_VISIBLE_COLUMNS.includes(id),
      ),
      columnOrder: FINDING_COLUMNS.map((c) => c.id),
    });
    persist(get());
  },

  setColumnOrder: (order) => {
    set({ columnOrder: order });
    persist(get());
  },

  toggleSelected: (id) => {
    const selected = get().selected;
    set({
      selected: selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id],
    });
  },

  setSelected: (ids) => set({ selected: ids }),
  clearSelected: () => set({ selected: [] }),
}));

/** 持久化台账视图配置（会话存储；不含任何明文数据） */
function persist(state: FindingState): void {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.setItem(
      VIEW_KEY,
      JSON.stringify({
        keyword: state.keyword,
        riskLevel: state.riskLevel,
        reviewStatus: state.reviewStatus,
        taskId: state.taskId,
        sort: state.sort,
        order: state.order,
        density: state.density,
        hiddenColumns: state.hiddenColumns,
        columnOrder: state.columnOrder,
      }),
    );
  } catch {
    /* 忽略存储失败 */
  }
}
