'use client';

/**
 * dbdlp 控制台 · P05 扫描任务业务组件
 *
 * 包含：*TaskStatusBadge（六态）、*TaskFilterBar、*TaskTable、*NewTaskDialog
 *
 * 规范（04 文档 §4.5）：任务六态各有令牌组与徽章图标，附加中文编码；
 * 发起前提示业务低峰期；运行中按 tasks/{id}/progress 轮询。
 */

import * as React from 'react';
import Link from 'next/link';
import {
  CalendarClock,
  CircleCheck,
  CirclePause,
  CircleX,
  Clock,
  Database,
  Ellipsis,
  GitBranch,
  ListFilter,
  LoaderCircle,
  Plus,
  RefreshCw,
  ScanSearch,
  TriangleExclamationPoint,
} from 'lucide-react';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Skeleton } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Input, Field } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
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
import { formatDateTime, TASK_STATUS_LABEL } from '@/lib/format';
import type { DataSource, ScanTask, TaskStatus } from '@/types/api';

/* ---------------------------------------------------------- *TaskStatusBadge */

/** 任务六态徽章：图标 + 中文文字（颜色非唯一线索） */
export function TaskStatusBadge({ status }: { status: TaskStatus | string }) {
  // 键为后端 port.TaskStatus；tone 名沿用 04 文档 §4.5 的 `--state-*` 令牌组语义。
  const map: Record<string, { tone: 'neutral' | 'info' | 'warning' | 'success' | 'danger'; Icon: typeof Clock }> = {
    pending: { tone: 'neutral', Icon: Clock },
    running: { tone: 'info', Icon: LoaderCircle },
    paused: { tone: 'warning', Icon: CirclePause },
    done: { tone: 'success', Icon: CircleCheck },
    partial: { tone: 'warning', Icon: TriangleExclamationPoint },
    failed: { tone: 'danger', Icon: CircleX },
  };
  const entry = map[status] ?? map.pending;
  const Icon = entry.Icon;
  return (
    <Badge tone={entry.tone}>
      <Icon
        size={12}
        aria-hidden="true"
        className={status === 'running' ? 'animate-spin' : undefined}
      />
      {TASK_STATUS_LABEL[status as TaskStatus] ?? '待执行'}
    </Badge>
  );
}

/* -------------------------------------------------------------- *TaskFilterBar */

/** 任务筛选栏：状态 + 数据源 */
export function TaskFilterBar({
  status,
  dataSourceId,
  datasources,
  onStatusChange,
  onDataSourceChange,
  onReset,
}: {
  status: string;
  dataSourceId: string;
  datasources: DataSource[];
  onStatusChange: (value: string) => void;
  onDataSourceChange: (value: string) => void;
  onReset: () => void;
}) {
  return (
    <Card>
      <CardContent className="flex flex-wrap items-end gap-[var(--space-3)] p-[var(--space-4)]">
        <div className="flex min-w-[180px] flex-col gap-[var(--space-1)]">
          <label
            htmlFor="task-status"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            任务状态
          </label>
          <Select value={status} onValueChange={onStatusChange}>
            <SelectTrigger id="task-status">
              <SelectValue placeholder="全部状态" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部状态</SelectItem>
              <SelectItem value="pending">待执行</SelectItem>
              <SelectItem value="running">运行中</SelectItem>
              <SelectItem value="paused">已暂停</SelectItem>
              <SelectItem value="done">成功</SelectItem>
              <SelectItem value="partial">部分失败</SelectItem>
              <SelectItem value="failed">失败</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex min-w-[200px] flex-col gap-[var(--space-1)]">
          <label
            htmlFor="task-datasource"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            数据源
          </label>
          <Select value={dataSourceId} onValueChange={onDataSourceChange}>
            <SelectTrigger id="task-datasource">
              <SelectValue placeholder="全部数据源" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部数据源</SelectItem>
              {datasources.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {item.name}
                </SelectItem>
              ))}
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

/* ---------------------------------------------------------------- *TaskTable */

const columnHelper = createColumnHelper<ScanTask>();

export function TaskTable({
  items,
  loading,
  onRefresh,
}: {
  items: ScanTask[];
  loading?: boolean;
  onRefresh: () => void;
}) {
  const columns = React.useMemo(
    () => [
      columnHelper.display({
        id: 'id',
        header: '任务编号',
        cell: ({ row }) => (
          <Link
            href={`/tasks/${row.original.id}`}
            className="font-[var(--font-mono)] text-[var(--fg)] underline-offset-2 hover:underline"
          >
            {row.original.id}
          </Link>
        ),
      }),
      columnHelper.display({
        id: 'dataSource',
        header: '数据源',
        cell: ({ row }) => (
          <span className="font-[var(--font-mono)] text-[var(--fg-2)]">
            {row.original.dataSourceName ?? row.original.dataSourceId}
          </span>
        ),
      }),
      columnHelper.accessor('status', {
        header: '状态',
        cell: (info) => <TaskStatusBadge status={info.getValue()} />,
      }),
      columnHelper.display({
        id: 'ruleVersion',
        header: '规则版本',
        cell: ({ row }) =>
          row.original.ruleVersion ? (
            <span className="inline-flex items-center gap-[var(--space-1)] font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--fg-2)]">
              <GitBranch size={12} aria-hidden="true" />
              {row.original.ruleVersion}
            </span>
          ) : (
            <span className="text-[var(--meta)]">—</span>
          ),
      }),
      columnHelper.display({
        id: 'summary',
        header: '命中 / 总列',
        cell: ({ row }) => (
          <span className="font-[var(--font-mono)] tabular-nums text-[var(--fg-2)]">
            {row.original.summary?.hitColumns ?? 0} / {row.original.summary?.totalColumns ?? 0}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'createdAt',
        header: '创建时间',
        cell: ({ row }) => (
          <span className="text-[var(--muted)]">{formatDateTime(row.original.createdAt)}</span>
        ),
      }),
      columnHelper.display({
        id: 'actions',
        header: '操作',
        cell: ({ row }) => (
          <div className="flex items-center justify-end gap-[var(--space-1)]">
            <Button variant="ghost" size="icon" asChild aria-label={`查看任务 ${row.original.id} 详情`}>
              <Link href={`/tasks/${row.original.id}`}>
                <ScanSearch size={16} aria-hidden="true" />
              </Link>
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`任务 ${row.original.id} 的更多操作`}
                >
                  <Ellipsis size={16} aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem asChild>
                  <Link href={`/tasks/${row.original.id}`}>查看任务详情</Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link href={`/findings?taskId=${encodeURIComponent(row.original.id)}`}>
                    查看该任务台账
                  </Link>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        ),
      }),
    ],
    [],
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
                    <ScanSearch size={24} aria-hidden="true" className="text-[var(--meta)]" />
                    <p className="text-[var(--text-sm)] text-[var(--muted)]">还没有扫描任务</p>
                    <p className="text-[var(--text-xs)] text-[var(--meta)]">
                      发起任务前请先完成数据源配置与连接测试
                    </p>
                  </div>
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className="bg-[var(--table-row-bg)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--table-row-bg-hover)]"
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

/* -------------------------------------------------------------- *NewTaskDialog */

/** 发起扫描任务弹窗：参数缺省取后端配置 */
export function NewTaskDialog({
  open,
  onOpenChange,
  datasources,
  draft,
  pending,
  error,
  onDraftChange,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  datasources: DataSource[];
  draft: {
    dataSourceId: string;
    sampleRows: number;
    batchSize: number;
    concurrency: number;
    lowPeakWindow: string;
  };
  pending: boolean;
  error: string | null;
  onDraftChange: (draft: Partial<{
    dataSourceId: string;
    sampleRows: number;
    batchSize: number;
    concurrency: number;
    lowPeakWindow: string;
  }>) => void;
  onSubmit: () => void;
}) {
  const [formError, setFormError] = React.useState<string | null>(null);

  const handleSubmit = () => {
    if (!draft.dataSourceId) {
      setFormError('请选择要扫描的数据源');
      return;
    }
    if (draft.sampleRows < 1 || draft.sampleRows > 1000) {
      setFormError('采样行数必须为 1 到 1000 之间的整数');
      return;
    }
    setFormError(null);
    onSubmit();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>发起扫描任务</DialogTitle>
          <DialogDescription>
            扫描按只读账号采样识别敏感字段；参数缺省取服务端配置，可按需覆盖。
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-[var(--space-4)]">
          <Field
            label="数据源"
            htmlFor="new-task-datasource"
            required
            error={formError && !draft.dataSourceId ? formError : undefined}
            hint="仅列出连接测试通过的数据源"
          >
            <Select
              value={draft.dataSourceId}
              onValueChange={(value) => onDraftChange({ dataSourceId: value })}
            >
              <SelectTrigger id="new-task-datasource">
                <SelectValue placeholder="选择数据源" />
              </SelectTrigger>
              <SelectContent>
                {datasources.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          <div className="grid grid-cols-1 gap-[var(--space-3)] sm:grid-cols-2">
            <Field
              label="采样行数"
              htmlFor="new-task-sample"
              required
              error={formError && draft.sampleRows < 1 ? formError : undefined}
              hint="每列采样行数，1~1000"
            >
              <Input
                id="new-task-sample"
                type="number"
                min={1}
                max={1000}
                value={draft.sampleRows}
                onChange={(e) => onDraftChange({ sampleRows: Number(e.target.value) })}
              />
            </Field>
            <Field label="批大小" htmlFor="new-task-batch" hint="单批处理行数">
              <Input
                id="new-task-batch"
                type="number"
                min={1}
                value={draft.batchSize}
                onChange={(e) => onDraftChange({ batchSize: Number(e.target.value) })}
              />
            </Field>
          </div>

          <div className="grid grid-cols-1 gap-[var(--space-3)] sm:grid-cols-2">
            <Field label="并发度" htmlFor="new-task-concurrency" hint="并发扫描的表数">
              <Input
                id="new-task-concurrency"
                type="number"
                min={1}
                value={draft.concurrency}
                onChange={(e) => onDraftChange({ concurrency: Number(e.target.value) })}
              />
            </Field>
            <Field label="低峰窗口" htmlFor="new-task-window" hint="留空表示不限时段">
              <Input
                id="new-task-window"
                value={draft.lowPeakWindow}
                placeholder="例如 01:00-06:00"
                onChange={(e) => onDraftChange({ lowPeakWindow: e.target.value })}
              />
            </Field>
          </div>

          <p className="flex items-start gap-[var(--space-1)] rounded-[var(--radius-sm)] bg-[var(--surface-warm)] px-[var(--space-2)] py-[var(--space-2)] text-[var(--text-xs)] text-[var(--muted)]">
            <CalendarClock size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
            建议在业务低峰期执行扫描任务，避免影响生产库读写。
          </p>

          {error ? (
            <p role="alert" className="text-[var(--text-sm)] text-[var(--danger)]">
              {error}
            </p>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={pending}>
            取消
          </Button>
          <Button onClick={handleSubmit} loading={pending}>
            <Plus size={16} aria-hidden="true" />
            发起任务
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ------------------------------------------------------------ 低峰提示条 */

/** 低峰提示（页面顶部） */
export function TaskLowPeakHint() {
  return (
    <p className="flex items-start gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
      <CalendarClock size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
      建议业务低峰期执行；采样行数默认 20，可在发起时覆盖。
    </p>
  );
}

/** 刷新按钮 */
export function TaskRefreshButton({ loading, onClick }: { loading: boolean; onClick: () => void }) {
  return (
    <Button variant="secondary" size="sm" loading={loading} onClick={onClick}>
      <RefreshCw size={16} aria-hidden="true" />
      刷新
    </Button>
  );
}

/** 统计小卡（任务数量） */
export function TaskCountCard({
  label,
  value,
  Icon,
}: {
  label: string;
  value: number;
  Icon: typeof Database;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <div className="flex flex-col gap-[var(--space-1)]">
          <CardTitle>{label}</CardTitle>
          <CardDescription>当前视图快照</CardDescription>
        </div>
        <Icon size={20} aria-hidden="true" className="text-[var(--muted)]" />
      </CardHeader>
      <CardContent>
        <span className="font-[var(--font-mono)] text-[var(--text-2xl)] font-[var(--weight-announce)] tabular-nums text-[var(--fg)]">
          {value}
        </span>
      </CardContent>
    </Card>
  );
}
