'use client';

/**
 * dbdlp 控制台 · 基础组件：Table
 *
 * 规范（04 文档 §4.3）：表头 sticky、行分隔 1px、不用斑马纹与彩色左边框；
 * 数字列 tabular-nums + 等宽字体。
 */

import * as React from 'react';
import { cn } from '@/lib/utils';

export const Table = React.forwardRef<
  HTMLTableElement,
  React.HTMLAttributes<HTMLTableElement>
>(({ className, ...props }, ref) => (
  <div className="w-full overflow-x-auto">
    <table
      ref={ref}
      className={cn('w-full caption-bottom border-collapse text-[var(--text-sm)]', className)}
      {...props}
    />
  </div>
));
Table.displayName = 'Table';

export const TableHeader = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(({ className, ...props }, ref) => (
  <thead
    ref={ref}
    className={cn('sticky top-0 z-[var(--z-sticky)] bg-[var(--table-header-bg)]', className)}
    {...props}
  />
));
TableHeader.displayName = 'TableHeader';

export const TableBody = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(({ className, ...props }, ref) => (
  <tbody ref={ref} className={cn('[&_tr:last-child]:border-0', className)} {...props}
  />
));
TableBody.displayName = 'TableBody';

export const TableRow = React.forwardRef<
  HTMLTableRowElement,
  React.HTMLAttributes<HTMLTableRowElement> & { selected?: boolean }
>(({ className, selected = false, ...props }, ref) => (
  <tr
    ref={ref}
    data-state={selected ? 'selected' : undefined}
    className={cn(
      'border-b border-[var(--table-divider)] transition-colors duration-[var(--motion-fast)] ease-[var(--ease-standard)]',
      selected
        ? 'bg-[var(--table-row-bg-selected)]'
        : 'bg-[var(--table-row-bg)] hover:bg-[var(--table-row-bg-hover)]',
      className,
    )}
    {...props}
  />
));
TableRow.displayName = 'TableRow';

export const TableHead = React.forwardRef<
  HTMLTableCellElement,
  React.ThHTMLAttributes<HTMLTableCellElement>
>(({ className, ...props }, ref) => (
  <th
    ref={ref}
    className={cn(
      'h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]',
      className,
    )}
    {...props}
  />
));
TableHead.displayName = 'TableHead';

export const TableCell = React.forwardRef<
  HTMLTableCellElement,
  React.TdHTMLAttributes<HTMLTableCellElement>
>(({ className, ...props }, ref) => (
  <td
    ref={ref}
    className={cn(
      'px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle text-[var(--fg-2)]',
      className,
    )}
    {...props}
  />
));
TableCell.displayName = 'TableCell';

/** 等宽数字单元格 */
export function NumericCell({
  className,
  ...props
}: React.TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <TableCell
      className={cn('text-right font-[var(--font-mono)] tabular-nums', className)}
      {...props}
    />
  );
}

/** 表头排序按钮（含 aria-sort 由 TableHead 承载） */
export function SortableHead({
  label,
  sorted,
  direction,
  onSort,
  className,
}: {
  label: string;
  sorted: boolean;
  direction: 'asc' | 'desc';
  onSort: () => void;
  className?: string;
}) {
  return (
    <TableHead
      aria-sort={sorted ? (direction === 'asc' ? 'ascending' : 'descending') : 'none'}
      className={className}
    >
      <button
        type="button"
        onClick={onSort}
        className="inline-flex items-center gap-[var(--space-1)] rounded-[var(--radius-sm)] text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)] transition-colors duration-[var(--motion-fast)] hover:text-[var(--fg)]"
      >
        {label}
        <span aria-hidden="true" className="text-[var(--meta)]">
          {sorted ? (direction === 'asc' ? '↑' : '↓') : '↕'}
        </span>
      </button>
    </TableHead>
  );
}

/** 表体空态行 */
export function TableEmptyRow({
  colSpan,
  message,
  action,
}: {
  colSpan: number;
  message: string;
  action?: React.ReactNode;
}) {
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className="h-[120px] text-center">
        <div className="flex flex-col items-center gap-[var(--space-2)]">
          <p className="text-[var(--text-sm)] text-[var(--muted)]">{message}</p>
          {action}
        </div>
      </TableCell>
    </TableRow>
  );
}
