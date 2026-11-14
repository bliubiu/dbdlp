'use client';

/**
 * dbdlp 控制台 · P02 数据源管理业务组件
 *
 * 包含：*DataSourceTable(TanStack)、*DataSourceFilterBar、*BulkActionBar、
 *       *ConnectionStateBadge、*ConfirmDeleteDialog
 *
 * 规范：服务端分页、连接测试通过才保存、删除二次确认、中文错误近行回显。
 */

import * as React from 'react';
import Link from 'next/link';
import {
  CircleCheck,
  CircleX,
  Database,
  Ellipsis,
  ListChecks,
  ListFilter,
  Pencil,
  PlugZap,
  Plus,
  Trash,
  Upload,
} from 'lucide-react';
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table';
import { Button } from '@/components/ui/button';
import { Badge, Card, CardContent, Skeleton } from '@/components/ui/primitives';
import { Input } from '@/components/ui/input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/overlays';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/overlays';
import { Checkbox } from '@/components/ui/overlays';
import { CONNECTION_STATE_LABEL, formatDateTime } from '@/lib/format';
import type { DataSource } from '@/types/api';

/* ---------------------------------------------------- *DataSourceFilterBar */

export function DataSourceFilterBar({
  keyword,
  dialect,
  onKeywordChange,
  onDialectChange,
  onReset,
}: {
  keyword: string;
  dialect: string;
  onKeywordChange: (value: string) => void;
  onDialectChange: (value: string) => void;
  onReset: () => void;
}) {
  return (
    <Card>
      <CardContent className="flex flex-wrap items-end gap-[var(--space-3)] p-[var(--space-4)]">
        <div className="flex min-w-[240px] flex-1 flex-col gap-[var(--space-1)]">
          <label
            htmlFor="ds-keyword"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            实例标识 / 主机关键字
          </label>
          <Input
            id="ds-keyword"
            value={keyword}
            placeholder="输入关键字筛选"
            onChange={(e) => onKeywordChange(e.target.value)}
          />
        </div>
        <div className="flex min-w-[160px] flex-col gap-[var(--space-1)]">
          <label
            htmlFor="ds-dialect"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            数据库方言
          </label>
          <Select value={dialect} onValueChange={onDialectChange}>
            <SelectTrigger id="ds-dialect">
              <SelectValue placeholder="全部方言" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部方言</SelectItem>
              <SelectItem value="dm">达梦 dm</SelectItem>
              <SelectItem value="mysql">MySQL</SelectItem>
              <SelectItem value="oracle">Oracle</SelectItem>
              <SelectItem value="postgresql">PostgreSQL</SelectItem>
              <SelectItem value="kingbase">Kingbase</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button variant="secondary" size="sm" onClick={onReset}>
          <ListFilter size={16} aria-hidden="true" />
          重置筛选
        </Button>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------- *BulkActionBar */

/** 批量操作浮动栏（选中后出现） */
export function BulkActionBar({
  selectedCount,
  onTest,
  onClear,
}: {
  selectedCount: number;
  onTest: () => void;
  onClear: () => void;
}) {
  if (selectedCount === 0) return null;
  return (
    <div className="sticky top-[var(--space-2)] z-[var(--z-sticky)] flex items-center gap-[var(--space-3)] rounded-[var(--radius-md)] border border-[var(--accent-border)] bg-[var(--accent-soft)] px-[var(--space-3)] py-[var(--space-2)]">
      <span className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
        已选中 {selectedCount} 个数据源
      </span>
      <Button variant="secondary" size="sm" onClick={onTest}>
        <ListChecks size={16} aria-hidden="true" />
        批量连接测试
      </Button>
      <Button variant="ghost" size="sm" onClick={onClear}>
        取消选择
      </Button>
    </div>
  );
}

/* -------------------------------------------------- *ConnectionStateBadge */

export function ConnectionStateBadge({
  state,
  testResult,
}: {
  state: string;
  testResult?: { ok: boolean; message: string };
}) {
  if (testResult) {
    return testResult.ok ? (
      <Badge tone="success">
        <CircleCheck size={12} aria-hidden="true" />
        连接正常
      </Badge>
    ) : (
      <Badge tone="danger">
        <CircleX size={12} aria-hidden="true" />
        连接异常
      </Badge>
    );
  }
  if (state === 'ok') {
    return (
      <Badge tone="success">
        <CircleCheck size={12} aria-hidden="true" />
        连接正常
      </Badge>
    );
  }
  if (state === 'failed') {
    return (
      <Badge tone="danger">
        <CircleX size={12} aria-hidden="true" />
        连接异常
      </Badge>
    );
  }
  return <Badge tone="neutral">{CONNECTION_STATE_LABEL[state] ?? '未验证'}</Badge>;
}

/* ---------------------------------------------------- *ConfirmDeleteDialog */

/** 删除二次确认（危险操作，焦点默认落「取消」，§8.3） */
export function ConfirmDeleteDialog({
  open,
  onOpenChange,
  target,
  onConfirm,
  pending,
  error,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  target: DataSource | null;
  onConfirm: () => void;
  pending: boolean;
  error: string | null;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>确认删除数据源</DialogTitle>
          <DialogDescription>
            将软删除数据源「{target?.name ?? ''}」。该操作不删除后端存储库中的历史台账记录，
            但存在运行中任务时服务端会拒绝并给出中文提示。
          </DialogDescription>
        </DialogHeader>
        {error ? (
          <p role="alert" className="mb-[var(--space-3)] text-[var(--text-sm)] text-[var(--danger)]">
            {error}
          </p>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={pending}>
            取消
          </Button>
          <Button variant="destructive" onClick={onConfirm} loading={pending}>
            <Trash size={16} aria-hidden="true" />
            确认删除
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* -------------------------------------------------------- *DataSourceTable */

const columnHelper = createColumnHelper<DataSource>();

export function DataSourceTable({
  items,
  selected,
  onToggle,
  onToggleAll,
  onTest,
  onDelete,
  testResults,
  loading,
}: {
  items: DataSource[];
  selected: string[];
  onToggle: (id: string) => void;
  onToggleAll: (checked: boolean) => void;
  onTest: (id: string) => void;
  onDelete: (item: DataSource) => void;
  testResults: Record<string, { ok: boolean; message: string }>;
  loading?: boolean;
}) {
  const columns = React.useMemo(
    () => [
      columnHelper.display({
        id: 'select',
        header: () => (
          <Checkbox
            checked={items.length > 0 && selected.length === items.length}
            onCheckedChange={(checked) => onToggleAll(Boolean(checked))}
            aria-label="全选当前页数据源"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={selected.includes(row.original.id)}
            onCheckedChange={() => onToggle(row.original.id)}
            aria-label={`选择数据源 ${row.original.name}`}
          />
        ),
      }),
      columnHelper.accessor('name', {
        header: '实例标识',
        cell: ({ row }) => (
          <Link
            href={`/datasources/${row.original.id}/edit`}
            className="font-[var(--font-mono)] text-[var(--fg)] underline-offset-2 hover:underline"
          >
            {row.original.name}
          </Link>
        ),
      }),
      columnHelper.accessor('dialect', {
        header: '方言',
        cell: (info) => <span className="font-[var(--font-mono)]">{info.getValue()}</span>,
      }),
      columnHelper.display({
        id: 'host',
        header: '主机:端口',
        cell: ({ row }) => (
          <span className="font-[var(--font-mono)]">
            {row.original.host}:{row.original.port}
          </span>
        ),
      }),
      columnHelper.accessor('serviceName', {
        header: '服务名',
        cell: (info) => <span className="font-[var(--font-mono)]">{info.getValue()}</span>,
      }),
      columnHelper.accessor('username', {
        header: '只读账号',
        cell: (info) => <span className="font-[var(--font-mono)]">{info.getValue()}</span>,
      }),
      columnHelper.display({
        id: 'connectionState',
        header: '连接状态',
        cell: ({ row }) => (
          <ConnectionStateBadge
            state={row.original.connectionState}
            testResult={testResults[row.original.id]}
          />
        ),
      }),
      columnHelper.accessor('lastScanAt', {
        header: '最近扫描',
        cell: (info) => (
          <span className="text-[var(--muted)]">{formatDateTime(info.getValue())}</span>
        ),
      }),
      columnHelper.display({
        id: 'actions',
        header: '操作',
        cell: ({ row }) => (
          <div className="flex items-center justify-end gap-[var(--space-1)]">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => onTest(row.original.id)}
              aria-label={`测试 ${row.original.name} 的连接`}
            >
              <PlugZap size={16} aria-hidden="true" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              asChild
              aria-label={`编辑 ${row.original.name}`}
            >
              <Link href={`/datasources/${row.original.id}/edit`}>
                <Pencil size={16} aria-hidden="true" />
              </Link>
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label={`${row.original.name} 的更多操作`}>
                  <Ellipsis size={16} aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => onTest(row.original.id)}>
                  <PlugZap size={16} aria-hidden="true" />
                  只读连接测试
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => onDelete(row.original)}>
                  <Trash size={16} aria-hidden="true" />
                  删除数据源
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        ),
      }),
    ],
    [items, selected, onToggle, onToggleAll, onTest, onDelete, testResults],
  );

  const table = useReactTable({
    data: items,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (row) => row.id,
  });

  if (loading) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-[var(--space-2)] p-[var(--card-padding)]">
          <Skeleton className="h-[36px] w-full" />
          <Skeleton className="h-[36px] w-full" />
          <Skeleton className="h-[36px] w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardContent className="p-0">
        <table className="w-full border-collapse text-[var(--text-sm)]">
          <thead className="sticky top-0 z-[var(--z-sticky)] bg-[var(--table-header-bg)]">
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th
                    key={header.id}
                    className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]"
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="h-[160px] text-center">
                  <div className="flex flex-col items-center gap-[var(--space-2)]">
                    <Database size={24} aria-hidden="true" className="text-[var(--meta)]" />
                    <p className="text-[var(--text-sm)] text-[var(--muted)]">配置第一个数据源</p>
                    <div className="flex items-center gap-[var(--space-2)]">
                      <Link
                        href="/datasources/new"
                        className="text-[var(--text-sm)] text-[var(--accent)] underline underline-offset-2"
                      >
                        新建数据源
                      </Link>
                      <span className="text-[var(--text-sm)] text-[var(--meta)]">或</span>
                      <Link
                        href="/datasources/import"
                        className="inline-flex items-center gap-[var(--space-1)] text-[var(--text-sm)] text-[var(--accent)] underline underline-offset-2"
                      >
                        <Upload size={14} aria-hidden="true" />
                        批量导入
                      </Link>
                    </div>
                  </div>
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  data-state={selected.includes(row.original.id) ? 'selected' : undefined}
                  className={
                    selected.includes(row.original.id)
                      ? 'bg-[var(--table-row-bg-selected)]'
                      : 'bg-[var(--table-row-bg)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--table-row-bg-hover)]'
                  }
                >
                  {row.getVisibleCells().map((cell) => (
                    <td
                      key={cell.id}
                      className="px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle text-[var(--fg-2)]"
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

/** 新建入口按钮 */
export function CreateDataSourceButton() {
  return (
    <Button asChild size="sm">
      <Link href="/datasources/new">
        <Plus size={16} aria-hidden="true" />
        新建数据源
      </Link>
    </Button>
  );
}
