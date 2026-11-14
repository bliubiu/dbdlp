'use client';

/**
 * dbdlp 控制台 · P05 扫描任务列表页（04 文档 §2.3）
 *
 * 顶部「发起扫描」+ 筛选栏 + 任务表格 + 分页；运行中任务按进度轮询刷新。
 */

import * as React from 'react';
import { useRouter } from 'next/navigation';
import { Plus, ScanSearch } from 'lucide-react';
import { useTaskStore } from '@/stores/useTaskStore';
import { useDataSourceStore } from '@/stores/useDataSourceStore';
import { useUIStore } from '@/stores/useUIStore';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/primitives';
import {
  NewTaskDialog,
  TaskCountCard,
  TaskFilterBar,
  TaskLowPeakHint,
  TaskRefreshButton,
  TaskTable,
} from '@/components/business/tasks';
import { PageDensity } from '@/components/shell/PageDensity';

export default function TasksPage() {
  const router = useRouter();
  const store = useTaskStore();
  const dataSourceStore = useDataSourceStore();
  const pushToast = useUIStore((s) => s.pushToast);
  const [dialogOpen, setDialogOpen] = React.useState(false);

  React.useEffect(() => {
    void store.fetchList({ page: 1 });
    void dataSourceStore.fetchList({ page: 1, limit: 100 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 有运行中任务时开启轮询（终态自动停止）
  const hasRunning = store.items.some((item) => item.status === 'running');
  React.useEffect(() => {
    if (!hasRunning) return;
    store.startPolling(store.items.find((item) => item.status === 'running')?.id ?? '');
    return () => store.stopPolling();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasRunning]);

  const handleSubmit = async () => {
    const taskId = await store.submitDraft();
    if (taskId) {
      setDialogOpen(false);
      pushToast('扫描任务已发起，正在执行', 'success');
      router.push(`/tasks/${taskId}`);
    }
  };

  const totalPages = Math.max(1, Math.ceil(store.total / store.limit));

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <PageDensity density="compact" />
      {/* 页头 */}
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h1>扫描任务</h1>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">
            按数据源发起敏感字段识别任务，失败对象支持单任务重试
          </p>
          <TaskLowPeakHint />
        </div>
        <div className="flex items-center gap-[var(--space-2)]">
          <TaskRefreshButton
            loading={store.loading}
            onClick={() => void store.fetchList()}
          />
          <Button size="sm" onClick={() => setDialogOpen(true)}>
            <Plus size={16} aria-hidden="true" />
            发起扫描
          </Button>
        </div>
      </div>

      {store.error ? (
        <Alert tone="danger" title="任务列表加载失败">
          {store.error}
        </Alert>
      ) : null}

      {/* 概览 */}
      <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] sm:grid-cols-3">
        <TaskCountCard label="任务总数" value={store.total} Icon={ScanSearch} />
        <TaskCountCard
          label="运行中"
          value={store.items.filter((t) => t.status === 'running').length}
          Icon={ScanSearch}
        />
        <TaskCountCard
          label="失败 / 部分失败"
          value={store.items.filter((t) => t.status === 'failed' || t.status === 'partial').length}
          Icon={ScanSearch}
        />
      </div>

      <TaskFilterBar
        status={store.status}
        dataSourceId={store.dataSourceId}
        datasources={dataSourceStore.items}
        onStatusChange={(value) => {
          store.setStatus(value);
          void store.fetchList({ page: 1, status: value });
        }}
        onDataSourceChange={(value) => {
          store.setDataSourceId(value);
          void store.fetchList({ page: 1, dataSourceId: value });
        }}
        onReset={() => {
          store.setStatus('');
          store.setDataSourceId('');
          void store.fetchList({ page: 1, status: '', dataSourceId: '' });
        }}
      />

      <TaskTable
        items={store.items}
        loading={store.loading}
        onRefresh={() => void store.fetchList()}
      />

      {/* 分页 */}
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <span className="text-[var(--text-xs)] text-[var(--muted)]">
          共 {store.total} 条，当前第 {store.page} / {totalPages} 页
        </span>
        <div className="flex items-center gap-[var(--space-2)]">
          <Button
            variant="secondary"
            size="sm"
            disabled={store.page <= 1}
            onClick={() => {
              const next = store.page - 1;
              store.setPage(next);
              void store.fetchList({ page: next });
            }}
          >
            上一页
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={!store.hasMore}
            onClick={() => {
              const next = store.page + 1;
              store.setPage(next);
              void store.fetchList({ page: next });
            }}
          >
            下一页
          </Button>
        </div>
      </div>

      <NewTaskDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        datasources={dataSourceStore.items}
        draft={store.draft}
        pending={false}
        error={store.error}
        onDraftChange={(draft) => store.setDraft(draft)}
        onSubmit={() => void handleSubmit()}
      />
    </div>
  );
}
