'use client';

/**
 * dbdlp 控制台 · P07 结果台账页（04 文档 §2.3、§4.3）
 *
 * 左侧分面筛选（可折叠）+ 右侧全宽表格；顶部批量栏与列显隐。
 * 排序按数值与等级权重，等级列禁字符串序。
 */

import * as React from 'react';
import { useRouter } from 'next/navigation';
import { RefreshCw } from 'lucide-react';
import { useFindingStore, FINDING_COLUMNS } from '@/stores/useFindingStore';
import { useTaskStore } from '@/stores/useTaskStore';
import { useUIStore } from '@/stores/useUIStore';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/primitives';
import {
  BatchActionBar,
  ColumnVisibilityMenu,
  FindingDensitySwitch,
  FindingFacetPanel,
  FindingScopeHint,
  FindingTable,
} from '@/components/business/findings';
import { PageDensity } from '@/components/shell/PageDensity';

export default function FindingsPage() {
  const router = useRouter();
  const store = useFindingStore();
  const taskStore = useTaskStore();
  const pushToast = useUIStore((s) => s.pushToast);
  const [notice, setNotice] = React.useState<string | null>(null);

  // 深链参数：taskId（从任务详情/概览跳转进入）
  // 静态导出下不能用 useSearchParams（需要 Suspense 边界），改为挂载后读 location.search
  const appliedTaskId = React.useRef<string | null>(null);

  React.useEffect(() => {
    if (appliedTaskId.current !== null) return;
    appliedTaskId.current = 'done';
    const initialTaskId = new URLSearchParams(window.location.search).get('taskId') ?? '';
    if (initialTaskId) {
      store.setTaskId(initialTaskId);
      void store.fetchList({ page: 1, taskId: initialTaskId });
    } else {
      void store.fetchList({ page: 1 });
    }
    void taskStore.fetchList({ page: 1, limit: 100 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleToggleAll = (checked: boolean) => {
    store.setSelected(checked ? store.items.map((item) => item.id) : []);
  };

  const handleExport = () => {
    // 导出走服务端报告生成（POST /reports），前端不一次性载入全量
    pushToast(
      '导出需在报告中心选择任务生成，样例均为掩码值',
      'info',
    );
    router.push('/reports');
  };

  const totalPages = Math.max(1, Math.ceil(store.total / store.limit));

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <PageDensity density="compact" />
      {/* 页头 */}
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h1>结果台账</h1>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">
            字段级敏感数据盘点结果，支持多维筛选、列显隐与排序
          </p>
          <FindingScopeHint />
        </div>
        <div className="flex flex-wrap items-center gap-[var(--space-2)]">
          <FindingDensitySwitch density={store.density} onChange={(value) => store.setDensity(value)} />
          <ColumnVisibilityMenu
            columns={FINDING_COLUMNS}
            hiddenColumns={store.hiddenColumns}
            onToggle={(id) => store.toggleColumn(id)}
            onReset={() => store.resetColumns()}
          />
          <Button
            variant="secondary"
            size="sm"
            loading={store.loading}
            onClick={() => void store.fetchList()}
          >
            <RefreshCw size={16} aria-hidden="true" />
            刷新
          </Button>
        </div>
      </div>

      {store.error ? (
        <Alert tone="danger" title="台账加载失败">
          {store.error}
        </Alert>
      ) : null}
      {notice ? <Alert tone="info">{notice}</Alert> : null}

      <BatchActionBar
        selectedCount={store.selected.length}
        onExport={handleExport}
        onClear={() => store.clearSelected()}
      />

      {/* 左侧分面 + 右侧表格 */}
      <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] lg:grid-cols-12">
        <div className="lg:col-span-3">
          <FindingFacetPanel
            keyword={store.keyword}
            riskLevel={store.riskLevel}
            reviewStatus={store.reviewStatus}
            taskId={store.taskId}
            tasks={taskStore.items}
            onKeywordChange={(value) => {
              store.setKeyword(value);
              void store.fetchList({ page: 1, keyword: value });
            }}
            onRiskLevelChange={(value) => {
              store.setRiskLevel(value);
              void store.fetchList({ page: 1, riskLevel: value });
            }}
            onReviewStatusChange={(value) => {
              store.setReviewStatus(value);
              void store.fetchList({ page: 1, reviewStatus: value });
            }}
            onTaskIdChange={(value) => {
              store.setTaskId(value);
              void store.fetchList({ page: 1, taskId: value });
            }}
            onReset={() => {
              store.setKeyword('');
              store.setRiskLevel('');
              store.setReviewStatus('');
              store.setTaskId('');
              void store.fetchList({
                page: 1,
                keyword: '',
                riskLevel: '',
                reviewStatus: '',
                taskId: '',
              });
              setNotice(null);
            }}
          />
        </div>

        <div className="flex flex-col gap-[var(--space-3)] lg:col-span-9">
          <FindingTable
            items={store.items}
            selected={store.selected}
            hiddenColumns={store.hiddenColumns}
            sort={store.sort}
            order={store.order}
            loading={store.loading}
            onToggle={store.toggleSelected}
            onToggleAll={handleToggleAll}
            onSort={(column) => store.toggleSort(column)}
            onOpenDetail={(id) => router.push(`/findings/${id}`)}
          />

          {/* 分页 */}
          <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
            <span className="text-[var(--text-xs)] text-[var(--muted)]">
              共 {store.total} 条，当前第 {store.page} / {totalPages} 页，每页 {store.limit} 条
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
        </div>
      </div>
    </div>
  );
}
