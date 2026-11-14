'use client';

/**
 * dbdlp 控制台 · P02 数据源管理（04 文档 §2.3）
 *
 * 顶部筛选栏 + 批量操作栏（选中后浮动）+ 全宽表格 + 分页。
 */

import * as React from 'react';
import Link from 'next/link';
import { FileUp, RefreshCw } from 'lucide-react';
import { useDataSourceStore } from '@/stores/useDataSourceStore';
import { useUIStore } from '@/stores/useUIStore';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/primitives';
import {
  BulkActionBar,
  ConfirmDeleteDialog,
  CreateDataSourceButton,
  DataSourceFilterBar,
  DataSourceTable,
} from '@/components/business/datasources';
import type { DataSource } from '@/types/api';
import { PageDensity } from '@/components/shell/PageDensity';

export default function DataSourcesPage() {
  const store = useDataSourceStore();
  const pushToast = useUIStore((s) => s.pushToast);

  const [deleteTarget, setDeleteTarget] = React.useState<DataSource | null>(null);
  const [deletePending, setDeletePending] = React.useState(false);
  const [deleteError, setDeleteError] = React.useState<string | null>(null);

  React.useEffect(() => {
    void store.fetchList({ page: 1 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleToggleAll = (checked: boolean) => {
    store.setSelected(checked ? store.items.map((item) => item.id) : []);
  };

  const handleBatchTest = async () => {
    for (const id of store.selected) {
      await store.testOne(id);
    }
    pushToast('批量连接测试已完成', 'success');
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeletePending(true);
    setDeleteError(null);
    const error = await store.removeOne(deleteTarget.id);
    setDeletePending(false);
    if (error) {
      setDeleteError(error);
      return;
    }
    pushToast(`数据源「${deleteTarget.name}」已删除`, 'success');
    setDeleteTarget(null);
  };

  const totalPages = Math.max(1, Math.ceil(store.total / store.limit));

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <PageDensity density="compact" />
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h1>数据源管理</h1>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">
            配置资产数据源连接；连接测试通过才保存，凭据不出后端
          </p>
        </div>
        <div className="flex items-center gap-[var(--space-2)]">
          <Button asChild variant="secondary" size="sm">
            <Link href="/datasources/import">
              <FileUp size={16} aria-hidden="true" />
              批量导入
            </Link>
          </Button>
          <CreateDataSourceButton />
        </div>
      </div>

      {store.error ? (
        <Alert tone="danger" title="列表加载失败">
          {store.error}
        </Alert>
      ) : null}

      <DataSourceFilterBar
        keyword={store.keyword}
        dialect={store.dialect}
        onKeywordChange={(value) => {
          store.setKeyword(value);
          void store.fetchList({ page: 1 });
        }}
        onDialectChange={(value) => {
          store.setDialect(value);
          void store.fetchList({ page: 1 });
        }}
        onReset={() => {
          store.setKeyword('');
          store.setDialect('');
          void store.fetchList({ page: 1 });
        }}
      />

      <BulkActionBar
        selectedCount={store.selected.length}
        onTest={() => void handleBatchTest()}
        onClear={() => store.clearSelected()}
      />

      <DataSourceTable
        items={store.items}
        selected={store.selected}
        testResults={store.testResults}
        loading={store.loading}
        onToggle={store.toggleSelected}
        onToggleAll={handleToggleAll}
        onTest={(id) => void store.testOne(id)}
        onDelete={(item) => {
          setDeleteTarget(item);
          setDeleteError(null);
        }}
      />

      {/* 分页（服务端分页，参数 page/limit） */}
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
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void store.fetchList()}
            loading={store.loading}
          >
            <RefreshCw size={16} aria-hidden="true" />
            刷新
          </Button>
        </div>
      </div>

      <ConfirmDeleteDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
            setDeleteError(null);
          }
        }}
        target={deleteTarget}
        onConfirm={() => void handleDelete()}
        pending={deletePending}
        error={deleteError}
      />
    </div>
  );
}
