'use client';

/**
 * dbdlp 控制台 · 概览切片（04 文档 §2.5，P01）
 *
 * 职责：概览聚合与刷新时点。持久化：否。
 */

import { create } from 'zustand';
import { getDashboardSummary } from '@/services/system';
import { listDataSources } from '@/services/datasources';
import { listTasks } from '@/services/tasks';
import { listFindings } from '@/services/findings';
import { ApiError } from '@/services/http';
import type { ColumnFinding, DataSource, DashboardSummary, ScanTask } from '@/types/api';

interface OverviewState {
  summary: DashboardSummary | null;
  /** 数据源总数 */
  instanceTotal: number;
  /** 连接异常数 */
  abnormalCount: number;
  /** 运行中任务数 */
  runningTasks: number;
  /** 高风险字段数 */
  highRiskFields: number;
  /** 数据源列表（首屏左 8 列） */
  datasources: DataSource[];
  /** 任务列表（首屏右 4 列进度卡） */
  tasks: ScanTask[];
  /** 最近台账 8 条 */
  recentFindings: ColumnFinding[];
  loading: boolean;
  error: string | null;
  /** 最近刷新时间戳 */
  refreshedAt: number | null;
  fetchOverview: () => Promise<void>;
}

export const useOverviewStore = create<OverviewState>((set) => ({
  summary: null,
  instanceTotal: 0,
  abnormalCount: 0,
  runningTasks: 0,
  highRiskFields: 0,
  datasources: [],
  tasks: [],
  recentFindings: [],
  loading: false,
  error: null,
  refreshedAt: null,

  fetchOverview: async () => {
    set({ loading: true, error: null });
    try {
      const [summary, datasources, tasks, findings] = await Promise.all([
        getDashboardSummary(),
        listDataSources({ page: 1, limit: 100 }),
        listTasks({ page: 1, limit: 100 }),
        listFindings({ page: 1, limit: 8, sort: 'updatedAt', order: 'desc' }),
      ]);

      const abnormal = (datasources.items ?? []).filter(
        (d) => d.connectionState === 'failed',
      ).length;
      const running = (tasks.items ?? []).filter((t) => t.status === 'running').length;

      set({
        summary,
        instanceTotal: datasources.total ?? (datasources.items ?? []).length,
        abnormalCount: abnormal,
        runningTasks: running,
        highRiskFields: summary?.riskCounts?.high ?? 0,
        datasources: datasources.items ?? [],
        tasks: tasks.items ?? [],
        recentFindings: findings.items ?? [],
        loading: false,
        refreshedAt: Date.now(),
      });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '概览数据加载失败，请稍后重试';
      set({ loading: false, error: message });
    }
  },
}));
