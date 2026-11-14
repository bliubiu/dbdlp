'use client';

/**
 * dbdlp 控制台 · P07 结果台账业务组件
 *
 * 包含：*FindingFacetPanel、*ColumnVisibilityMenu、*BatchActionBar、
 *       *HitRatioCell、*RiskBadge、*MaskedSampleCell、*FindingTable
 *
 * 规范（04 文档 §4.3）：
 * - 18 列定义，accessorKey 与后端字段一一对应
 * - 默认显示 1~5、8~9、13~14、15~17 列，其余由列显隐开启（会话存储持久化）
 * - 表头 sticky、行分隔 1px、无斑马纹、无彩色左边框
 * - 排序：数值列按数值，等级列按权重（禁字符串序）
 * - 掩码样例只渲染 sampleMasked
 */

import * as React from 'react';
import Link from 'next/link';
import {
  ArrowUpDown,
  ChevronRight,
  CircleCheck,
  CircleDot,
  CircleExclamationPoint,
  CircleQuestionMark,
  CircleX,
  Columns3,
  Copy,
  Download,
  EyeOff,
  Hash,
  Inbox,
  ListChecks,
  ListFilter,
  Rows3,
  Type,
} from 'lucide-react';
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Skeleton } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Checkbox,
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/overlays';
import {
  CONFIDENCE_LABEL,
  formatDateTime,
  formatPercent,
  formatRelative,
  REVIEW_STATUS_LABEL,
  STORAGE_FORM_LABEL,
  toUiRisk,
  UI_RISK_LABEL,
} from '@/lib/format';
import { cn } from '@/lib/utils';
import type { ColumnFinding, Confidence, ReviewStatus, StorageForm } from '@/types/api';

/* -------------------------------------------------------------- *RiskBadge */

/** 风险徽章：色块 + 图标 + 中文文字三重编码（§4.4 规则 ④） */
export function RiskBadge({ level }: { level: string }) {
  const ui = toUiRisk(level);
  const Icon =
    ui === 'high'
      ? CircleExclamationPoint
      : ui === 'medium'
        ? CircleDot
        : ui === 'low'
          ? CircleCheck
          : CircleQuestionMark;
  const cls =
    ui === 'high'
      ? 'border-[var(--risk-high-border)] bg-[var(--risk-high-bg)] text-[var(--risk-high-fg)]'
      : ui === 'medium'
        ? 'border-[var(--risk-medium-border)] bg-[var(--risk-medium-bg)] text-[var(--risk-medium-fg)]'
        : ui === 'low'
          ? 'border-[var(--risk-low-border)] bg-[var(--risk-low-bg)] text-[var(--risk-low-fg)]'
          : 'border-[var(--risk-pending-border)] bg-[var(--risk-pending-bg)] text-[var(--risk-pending-fg)]';
  return (
    <span
      className={cn(
        'inline-flex items-center gap-[var(--space-1)] whitespace-nowrap rounded-[var(--badge-radius)] border px-[var(--badge-padding-x)] py-[var(--badge-padding-y)] text-[var(--text-xs)] font-[var(--weight-emphasize)]',
        cls,
      )}
    >
      <Icon size={12} aria-hidden="true" />
      {UI_RISK_LABEL[ui]}
    </span>
  );
}

/* ----------------------------------------------------------- *HitRatioCell */

/** 命中率单元格：等宽 + 微型进度条 */
export function HitRatioCell({ ratio }: { ratio: number }) {
  const pct = Math.max(0, Math.min(100, Math.round((ratio > 1 ? ratio : ratio * 100) * 10) / 10));
  return (
    <span className="flex items-center justify-end gap-[var(--space-2)]">
      <span className="font-[var(--font-mono)] tabular-nums text-[var(--fg-2)]">
        {formatPercent(ratio)}
      </span>
      <span
        aria-hidden="true"
        className="h-[4px] w-[40px] overflow-hidden rounded-[var(--progress-radius)] bg-[var(--progress-track)]"
      >
        <span
          className="block h-full rounded-[var(--progress-radius)] bg-[var(--progress-fill)]"
          style={{ width: `${pct}%` }}
        />
      </span>
    </span>
  );
}

/* -------------------------------------------------------- *MaskedSampleCell */

/** 掩码样例单元格：仅渲染 sampleMasked（§4.6） */
export function MaskedSampleCell({ value }: { value: string }) {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <span className="inline-flex items-center gap-[var(--space-1)]">
      <span
        title="样例已脱敏，平台不展示完整明文"
        className="rounded-[var(--radius-sm)] border border-[var(--sample-border)] bg-[var(--sample-bg)] px-[var(--space-1)] font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--sample-fg)]"
      >
        {value || '—'}
      </span>
      <Button
        variant="ghost"
        size="icon"
        onClick={() => void handleCopy()}
        aria-label={`复制掩码样例 ${value}`}
        className="h-[24px] w-[24px]"
      >
        {copied ? (
          <CircleCheck size={12} aria-hidden="true" />
        ) : (
          <Copy size={12} aria-hidden="true" />
        )}
      </Button>
    </span>
  );
}

/* -------------------------------------------------------- *FindingFacetPanel */

/** 左侧分面筛选面板（可折叠） */
export function FindingFacetPanel({
  keyword,
  riskLevel,
  reviewStatus,
  taskId,
  tasks,
  onKeywordChange,
  onRiskLevelChange,
  onReviewStatusChange,
  onTaskIdChange,
  onReset,
}: {
  keyword: string;
  riskLevel: string;
  reviewStatus: string;
  taskId: string;
  tasks: { id: string; dataSourceName?: string; dataSourceId: string }[];
  onKeywordChange: (value: string) => void;
  onRiskLevelChange: (value: string) => void;
  onReviewStatusChange: (value: string) => void;
  onTaskIdChange: (value: string) => void;
  onReset: () => void;
}) {
  return (
    <Card className="sticky top-[var(--space-3)]">
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ListFilter size={20} aria-hidden="true" />
          筛选
        </CardTitle>
        <Button variant="ghost" size="sm" onClick={onReset}>
          重置
        </Button>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <label
            htmlFor="finding-keyword"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            表名 / 字段名关键字
          </label>
          <Input
            id="finding-keyword"
            value={keyword}
            placeholder="输入关键字筛选"
            onChange={(e) => onKeywordChange(e.target.value)}
          />
        </div>

        <div className="flex flex-col gap-[var(--space-1)]">
          <label
            htmlFor="finding-risk"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            风险等级
          </label>
          <Select value={riskLevel} onValueChange={onRiskLevelChange}>
            <SelectTrigger id="finding-risk">
              <SelectValue placeholder="全部等级" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部等级</SelectItem>
              <SelectItem value="high">高</SelectItem>
              <SelectItem value="medium_high">中高</SelectItem>
              <SelectItem value="medium">中</SelectItem>
              <SelectItem value="medium_low">中低</SelectItem>
              <SelectItem value="low">低</SelectItem>
              <SelectItem value="pending_review">待复核</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-[var(--space-1)]">
          <label
            htmlFor="finding-review"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            复核状态
          </label>
          <Select value={reviewStatus} onValueChange={onReviewStatusChange}>
            <SelectTrigger id="finding-review">
              <SelectValue placeholder="全部状态" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部状态</SelectItem>
              <SelectItem value="pending">待复核</SelectItem>
              <SelectItem value="confirmed">确认敏感</SelectItem>
              <SelectItem value="false_positive">误报</SelectItem>
              <SelectItem value="ignored">已忽略</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-[var(--space-1)]">
          <label
            htmlFor="finding-task"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            所属任务
          </label>
          <Select value={taskId} onValueChange={onTaskIdChange}>
            <SelectTrigger id="finding-task">
              <SelectValue placeholder="全部任务" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部任务</SelectItem>
              {tasks.map((task) => (
                <SelectItem key={task.id} value={task.id}>
                  {task.dataSourceName ?? task.dataSourceId}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------ *ColumnVisibilityMenu */

/** 列显隐菜单（持久化在会话存储） */
export function ColumnVisibilityMenu({
  columns,
  hiddenColumns,
  onToggle,
  onReset,
}: {
  columns: readonly { id: string; label: string }[];
  hiddenColumns: string[];
  onToggle: (id: string) => void;
  onReset: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" size="sm">
          <Columns3 size={16} aria-hidden="true" />
          列显隐
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[220px]">
        <DropdownMenuLabel>显示的列</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {columns
          .filter((col) => col.id !== 'select' && col.id !== 'actions')
          .map((col) => (
            <DropdownMenuCheckboxItem
              key={col.id}
              checked={!hiddenColumns.includes(col.id)}
              onCheckedChange={() => onToggle(col.id)}
            >
              {col.label}
            </DropdownMenuCheckboxItem>
          ))}
        <DropdownMenuSeparator />
        <Button variant="ghost" size="sm" className="w-full" onClick={onReset}>
          恢复默认列
        </Button>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/* ------------------------------------------------------------ *BatchActionBar */

/** 批量操作栏（选中后出现） */
export function BatchActionBar({
  selectedCount,
  onExport,
  onClear,
  exportDisabled,
}: {
  selectedCount: number;
  onExport: () => void;
  onClear: () => void;
  exportDisabled?: boolean;
}) {
  if (selectedCount === 0) return null;
  return (
    <div className="sticky top-[var(--space-2)] z-[var(--z-sticky)] flex flex-wrap items-center gap-[var(--space-3)] rounded-[var(--radius-md)] border border-[var(--accent-border)] bg-[var(--accent-soft)] px-[var(--space-3)] py-[var(--space-2)]">
      <span className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
        已选中 {selectedCount} 个字段
      </span>
      <Button variant="secondary" size="sm" disabled={exportDisabled} onClick={onExport}>
        <Download size={16} aria-hidden="true" />
        导出所选
      </Button>
      <Button variant="ghost" size="sm" onClick={onClear}>
        取消选择
      </Button>
      <span className="text-[var(--text-xs)] text-[var(--muted)]">
        批量复核需逐条提交；导出会生成后端报告产物并记录审计
      </span>
    </div>
  );
}

/* ------------------------------------------------------------- *FindingTable */

const columnHelper = createColumnHelper<ColumnFinding>();

export function FindingTable({
  items,
  selected,
  hiddenColumns,
  sort,
  order,
  loading,
  onToggle,
  onToggleAll,
  onSort,
  onOpenDetail,
}: {
  items: ColumnFinding[];
  selected: string[];
  hiddenColumns: string[];
  sort: string;
  order: 'asc' | 'desc';
  loading?: boolean;
  onToggle: (id: string) => void;
  onToggleAll: (checked: boolean) => void;
  onSort: (column: string) => void;
  onOpenDetail: (id: string) => void;
}) {
  const visible = React.useCallback(
    (id: string) => !hiddenColumns.includes(id),
    [hiddenColumns],
  );

  const SortableHeader = React.useCallback(
    (label: string, columnId: string) => (
      <button
        type="button"
        onClick={() => onSort(columnId)}
        className="inline-flex items-center gap-[var(--space-1)] rounded-[var(--radius-sm)] text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)] transition-colors duration-[var(--motion-fast)] hover:text-[var(--fg)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
      >
        {label}
        <ArrowUpDown
          size={12}
          aria-hidden="true"
          className={sort === columnId ? 'text-[var(--fg)]' : 'text-[var(--meta)]'}
        />
      </button>
    ),
    [onSort, sort],
  );

  const columns = React.useMemo(
    () => [
      columnHelper.display({
        id: 'select',
        header: () => (
          <Checkbox
            checked={items.length > 0 && selected.length === items.length}
            onCheckedChange={(checked) => onToggleAll(Boolean(checked))}
            aria-label="全选当前页字段"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={selected.includes(row.original.id)}
            onCheckedChange={() => onToggle(row.original.id)}
            aria-label={`选择字段 ${row.original.columnName}`}
          />
        ),
      }),
      columnHelper.accessor('instanceId', {
        header: () => SortableHeader('实例', 'instanceId'),
        cell: (info) => (
          <span className="font-[var(--font-mono)] text-[var(--text-xs)]">{info.getValue()}</span>
        ),
      }),
      columnHelper.accessor('schemaName', {
        header: () => SortableHeader('模式', 'schemaName'),
        cell: (info) => (
          <span className="font-[var(--font-mono)] text-[var(--text-xs)]">{info.getValue()}</span>
        ),
      }),
      columnHelper.accessor('tableName', {
        header: () => SortableHeader('表名', 'tableName'),
        cell: (info) => (
          <span title={info.getValue()} className="block max-w-[180px] truncate font-[var(--font-mono)] text-[var(--text-xs)]">
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor('columnName', {
        header: () => SortableHeader('字段名', 'columnName'),
        cell: (info) => (
          <span className="rounded-[var(--radius-sm)] bg-[var(--sample-bg)] px-[var(--space-1)] font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--sample-fg)]">
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor('columnComment', {
        header: '字段注释',
        cell: (info) => (
          <span title={info.getValue()} className="block max-w-[200px] truncate text-[var(--text-xs)] text-[var(--muted)]">
            {info.getValue() || '—'}
          </span>
        ),
      }),
      columnHelper.accessor('dataType', {
        header: () => SortableHeader('字段类型', 'dataType'),
        cell: (info) => (
          <span className="font-[var(--font-mono)] text-[var(--text-xs)]">{info.getValue()}</span>
        ),
      }),
      columnHelper.accessor('sensitiveTypes', {
        header: '敏感类型',
        cell: (info) => {
          const types = info.getValue() ?? [];
          return (
            <span className="flex flex-wrap gap-[var(--space-1)]">
              {types.slice(0, 2).map((type) => (
                <Badge key={type} tone="neutral">
                  {type}
                </Badge>
              ))}
              {types.length > 2 ? <Badge tone="neutral">+{types.length - 2}</Badge> : null}
            </span>
          );
        },
      }),
      columnHelper.accessor('hitRatio', {
        header: () => SortableHeader('命中率', 'hitRatio'),
        cell: (info) => <HitRatioCell ratio={info.getValue()} />,
      }),
      columnHelper.display({
        id: 'sampleHit',
        header: () => SortableHeader('命中/样本', 'sampleHit'),
        cell: ({ row }) => (
          <span className="font-[var(--font-mono)] tabular-nums text-[var(--text-xs)] text-[var(--fg-2)]">
            {row.original.sampleHit} / {row.original.sampleTotal}
          </span>
        ),
      }),
      columnHelper.accessor('verifyPassed', {
        header: 'Verify',
        cell: (info) =>
          info.getValue() ? (
            <span className="inline-flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--success)]">
              <CircleCheck size={12} aria-hidden="true" />
              通过
            </span>
          ) : (
            <span className="inline-flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--danger)]">
              <CircleX size={12} aria-hidden="true" />
              未通过
            </span>
          ),
      }),
      columnHelper.accessor('confidence', {
        header: () => SortableHeader('置信度', 'confidence'),
        cell: (info) => (
          <Badge tone={info.getValue() === 'high' ? 'success' : info.getValue() === 'medium' ? 'info' : 'neutral'}>
            {CONFIDENCE_LABEL[info.getValue() as Confidence] ?? '低'}
          </Badge>
        ),
      }),
      columnHelper.accessor('storageForm', {
        header: '存储形态',
        cell: (info) => (
          <span className="text-[var(--text-xs)] text-[var(--fg-2)]">
            {STORAGE_FORM_LABEL[info.getValue() as StorageForm] ?? '—'}
          </span>
        ),
      }),
      columnHelper.accessor('riskLevel', {
        header: () => SortableHeader('风险等级', 'riskLevel'),
        cell: (info) => <RiskBadge level={info.getValue()} />,
      }),
      columnHelper.accessor('reviewStatus', {
        header: () => SortableHeader('复核状态', 'reviewStatus'),
        cell: (info) => (
          <Badge
            tone={
              info.getValue() === 'confirmed'
                ? 'success'
                : info.getValue() === 'false_positive'
                  ? 'neutral'
                  : info.getValue() === 'ignored'
                    ? 'neutral'
                    : 'warning'
            }
          >
            {REVIEW_STATUS_LABEL[info.getValue() as ReviewStatus] ?? '待复核'}
          </Badge>
        ),
      }),
      columnHelper.accessor('sampleMasked', {
        header: '掩码样例',
        cell: (info) => <MaskedSampleCell value={info.getValue()} />,
      }),
      columnHelper.accessor('updatedAt', {
        header: () => SortableHeader('最后扫描', 'updatedAt'),
        cell: (info) => (
          <span title={formatDateTime(info.getValue())} className="text-[var(--text-xs)] text-[var(--muted)]">
            {formatRelative(info.getValue())}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'actions',
        header: '操作',
        cell: ({ row }) => (
          <div className="flex items-center justify-end">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => onOpenDetail(row.original.id)}
              aria-label={`查看字段 ${row.original.columnName} 明细`}
            >
              <ChevronRight size={16} aria-hidden="true" />
            </Button>
          </div>
        ),
      }),
    ],
    [items, selected, onToggle, onToggleAll, SortableHeader, onOpenDetail],
  );

  const visibleColumns = React.useMemo(
    () => columns.filter((col) => (col.id ? visible(col.id) : true)),
    [columns, visible],
  );

  const table = useReactTable({
    data: items,
    columns: visibleColumns,
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
                <td colSpan={visibleColumns.length} className="h-[180px] text-center">
                  <div className="flex flex-col items-center gap-[var(--space-2)]">
                    <Inbox size={24} aria-hidden="true" className="text-[var(--meta)]" />
                    <p className="text-[var(--text-sm)] text-[var(--muted)]">当前筛选条件下没有命中字段</p>
                    <p className="text-[var(--text-xs)] text-[var(--meta)]">
                      调整筛选条件，或先在扫描任务中完成一次识别
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

/* ------------------------------------------------------------------ 密度档 */

/** 密度切换按钮组 */
export function FindingDensitySwitch({
  density,
  onChange,
}: {
  density: 'compact' | 'default' | 'comfortable';
  onChange: (value: 'compact' | 'default' | 'comfortable') => void;
}) {
  const options: { value: 'compact' | 'default' | 'comfortable'; label: string }[] = [
    { value: 'compact', label: '紧凑' },
    { value: 'default', label: '标准' },
    { value: 'comfortable', label: '宽松' },
  ];
  return (
    <div
      role="group"
      aria-label="行密度"
      className="inline-flex items-center gap-[var(--space-1)] rounded-[var(--radius-md)] border border-[var(--border-soft)] bg-[var(--surface-warm)] p-[var(--space-1)]"
    >
      <Rows3 size={16} aria-hidden="true" className="mx-[var(--space-1)] text-[var(--muted)]" />
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => onChange(option.value)}
          aria-pressed={density === option.value}
          className={cn(
            'min-h-[28px] rounded-[var(--radius-sm)] px-[var(--space-2)] text-[var(--text-xs)] font-[var(--weight-emphasize)] transition-colors duration-[var(--motion-fast)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]',
            density === option.value
              ? 'bg-[var(--surface)] text-[var(--fg)]'
              : 'text-[var(--muted)] hover:text-[var(--fg)]',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

/* ------------------------------------------------------------ 面板提示信息 */

/** 台账口径说明 */
export function FindingScopeHint() {
  return (
    <p className="flex items-start gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
      <EyeOff size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
      掩码样例只展示脱敏值，平台不展示完整明文；导出走服务端流式生成并记录审计。
    </p>
  );
}

/** 字段类型图标（供 P08 复用） */
export function DataTypeIcon({ type }: { type: string }) {
  return type === 'Type' ? (
    <Type size={16} aria-hidden="true" />
  ) : (
    <Hash size={16} aria-hidden="true" />
  );
}

/** 批量操作条里的列表检查图标（保持图标语义一致性） */
export const FindingListIcon = ListChecks;
