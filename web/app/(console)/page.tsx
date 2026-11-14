'use client';

/**
 * dbdlp 控制台 · P01 总览工作台（04 文档 §2.3、§1.3）
 *
 * 12 列布局：摘要行通栏 → 左 8 列数据源列表 / 右 4 列（进度、风险分布）→ 通栏最近台账。
 * 首屏直接进入真实数据内容，无营销 Hero。
 */

import * as React from 'react';
import { RefreshCw } from 'lucide-react';
import { useOverviewStore } from '@/stores/useOverviewStore';
import { useTaskStore } from '@/stores/useTaskStore';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/primitives';
import {
  DataSourceStatusTable,
  OverviewKpiStrip,
  RecentFindingsTable,
  RiskDistributionCard,
  ScanProgressCard,
} from '@/components/business/overview';

export default function OverviewPage() {
  const overview = useOverviewStore();
  const fetchOverview = useOverviewStore((s) => s.fetchOverview);
  const fetchTasks = useTaskStore((s) => s.fetchList);

  React.useEffect(() => {
    void fetchOverview();
    void fetchTasks({ page: 1, limit: 50 });
  }, [fetchOverview, fetchTasks]);

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      {/* 页头 */}
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h1>总览工作台</h1>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">
            数据源、扫描任务与字段风险的当前视图快照
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            void fetchOverview();
            void fetchTasks({ page: 1, limit: 50 });
          }}
          loading={overview.loading}
        >
          <RefreshCw size={16} aria-hidden="true" />
          刷新
        </Button>
      </div>

      {overview.error ? (
        <Alert tone="danger" title="数据加载失败">
          {overview.error}
        </Alert>
      ) : null}

      {/* 摘要行 */}
      <OverviewKpiStrip
        instanceTotal={overview.instanceTotal}
        abnormalCount={overview.abnormalCount}
        runningTasks={overview.runningTasks}
        highRiskFields={overview.highRiskFields}
        loading={overview.loading}
      />

      {/* 左 8 列 + 右 4 列 */}
      <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] lg:grid-cols-12">
        <div className="lg:col-span-8">
          <DataSourceStatusTable items={overview.datasources} loading={overview.loading} />
        </div>
        <div className="flex flex-col gap-[var(--gutter-desktop)] lg:col-span-4">
          <ScanProgressCard tasks={overview.tasks} loading={overview.loading} />
          <RiskDistributionCard summary={overview.summary} loading={overview.loading} />
        </div>
      </div>

      {/* 通栏最近台账 */}
      <RecentFindingsTable items={overview.recentFindings} loading={overview.loading} />
    </div>
  );
}
