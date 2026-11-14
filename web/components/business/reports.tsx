'use client';

/**
 * dbdlp 控制台 · P09 报告中心业务组件
 *
 * 包含：*ReportTable、*GenerateReportDialog、*ReportFormatBadge、*ReportStatusBadge
 *
 * 规范（04 文档 §2.3、§4.7）：
 * - 生成中提示「可离开本页」，回本页后由 useReportStore 轮询恢复
 * - 下载前明示「样例均为掩码」
 * - 报告生成状态复用任务状态徽章
 */

import * as React from 'react';
import {
  CircleCheck,
  CircleX,
  ClipboardList,
  Download,
  FileBraces,
  FileSpreadsheet,
  FileText,
  LoaderCircle,
  Lock,
  Plus,
  TriangleExclamationPoint,
} from 'lucide-react';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Skeleton } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Checkbox,
} from '@/components/ui/overlays';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, NumericCell } from '@/components/ui/table';
import { formatBytes, formatDateTime, TASK_STATUS_LABEL } from '@/lib/format';
import type { ReportArtifact, ReportFormat, ScanTask, TaskStatus } from '@/types/api';

/* ------------------------------------------------------- *ReportFormatBadge */

/** 报告格式徽章 */
export function ReportFormatBadge({ format }: { format: ReportFormat | string }) {
  const Icon =
    format === 'excel' ? FileSpreadsheet : format === 'json' ? FileBraces : FileText;
  const label = format === 'excel' ? 'Excel 台账' : format === 'json' ? 'JSON 明细' : 'HTML 审计报告';
  return (
    <Badge tone="neutral">
      <Icon size={12} aria-hidden="true" />
      {label}
    </Badge>
  );
}

/* ------------------------------------------------------- *ReportStatusBadge */

/** 报告生成状态徽章（复用任务六态语义） */
export function ReportStatusBadge({ status }: { status: TaskStatus | string }) {
  const map: Record<
    string,
    { tone: 'neutral' | 'info' | 'warning' | 'success' | 'danger'; Icon: typeof CircleCheck }
  > = {
    pending: { tone: 'neutral', Icon: ClipboardList },
    running: { tone: 'info', Icon: LoaderCircle },
    paused: { tone: 'warning', Icon: ClipboardList },
    done: { tone: 'success', Icon: CircleCheck },
    partial: { tone: 'warning', Icon: TriangleExclamationPoint },
    failed: { tone: 'danger', Icon: CircleX },
  };
  const entry = map[status] ?? map.pending;
  const Icon = entry.Icon;
  return (
    <Badge tone={entry.tone}>
      <Icon size={12} aria-hidden="true" className={status === 'running' ? 'animate-spin' : undefined} />
      {TASK_STATUS_LABEL[status as TaskStatus] ?? '待执行'}
    </Badge>
  );
}

/* ------------------------------------------------------------- *ReportTable */

export function ReportTable({
  items,
  loading,
  downloadingId,
  onDownload,
  onPreview,
}: {
  items: ReportArtifact[];
  loading?: boolean;
  downloadingId: string | null;
  onDownload: (item: ReportArtifact) => void;
  onPreview: (item: ReportArtifact) => void;
}) {
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
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>格式</TableHead>
              <TableHead>所属任务</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="w-[110px]">大小</TableHead>
              <TableHead className="w-[150px]">生成时间</TableHead>
              <TableHead className="w-[180px] text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.length === 0 ? (
              <TableEmptyRow colSpan={6} message="还没有生成过报告" />
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell>
                    <ReportFormatBadge format={item.format} />
                  </TableCell>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">
                    {item.taskName ?? item.taskId}
                  </TableCell>
                  <TableCell>
                    <ReportStatusBadge status={item.status} />
                  </TableCell>
                  <NumericCell className="text-[var(--text-xs)]">{formatBytes(item.fileSize)}</NumericCell>
                  <TableCell className="text-[var(--muted)]">{formatDateTime(item.createdAt)}</TableCell>
                  <TableCell>
                    <div className="flex items-center justify-end gap-[var(--space-1)]">
                      <Button variant="ghost" size="sm" onClick={() => onPreview(item)}>
                        预览
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        loading={downloadingId === item.id}
                        onClick={() => onDownload(item)}
                      >
                        <Download size={16} aria-hidden="true" />
                        下载
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------ *GenerateReportDialog */

/** 生成报告对话框：选任务 + 选格式 */
export function GenerateReportDialog({
  open,
  onOpenChange,
  tasks,
  taskId,
  formats,
  generating,
  error,
  onTaskIdChange,
  onFormatsChange,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tasks: ScanTask[];
  taskId: string;
  formats: ReportFormat[];
  generating: boolean;
  error: string | null;
  onTaskIdChange: (value: string) => void;
  onFormatsChange: (formats: ReportFormat[]) => void;
  onSubmit: () => void;
}) {
  const allFormats: { value: ReportFormat; label: string; desc: string }[] = [
    {
      value: 'excel',
      label: 'Excel 台账',
      desc: '六个 Sheet：概览 / 字段台账 / 命中明细 / 复核记录 / 规则清单 / 任务记录',
    },
    {
      value: 'html',
      label: 'HTML 审计报告',
      desc: '八章节在线可读，可在报告预览页查阅与打印',
    },
    {
      value: 'json',
      label: 'JSON 明细',
      desc: '供下游治理平台接入，仅含掩码样例',
    },
  ];

  const toggle = (value: ReportFormat) => {
    onFormatsChange(
      formats.includes(value) ? formats.filter((f) => f !== value) : [...formats, value],
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>生成报告</DialogTitle>
          <DialogDescription>
            报告由服务端流式生成并写入产物表，生成中可离开本页，回来后可继续查看。
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-[var(--space-4)]">
          <div className="flex flex-col gap-[var(--space-1)]">
            <label
              htmlFor="report-task"
              className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]"
            >
              扫描任务 <span className="text-[var(--danger)]">*</span>
            </label>
            <select
              id="report-task"
              value={taskId}
              onChange={(e) => onTaskIdChange(e.target.value)}
              className="h-[var(--input-height)] w-full rounded-[var(--input-radius)] border border-[var(--input-border)] bg-[var(--input-bg)] px-[var(--space-3)] text-[var(--text-sm)] text-[var(--input-fg)] focus-visible:border-[var(--input-border-focus)] focus-visible:outline-none focus-visible:shadow-[var(--input-ring-focus)]"
            >
              <option value="">选择要生成报告的任务</option>
              {tasks.map((task) => (
                <option key={task.id} value={task.id}>
                  {task.dataSourceName ?? task.dataSourceId} · {task.id}
                </option>
              ))}
            </select>
          </div>

          <fieldset className="flex flex-col gap-[var(--space-2)]">
            <legend className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
              报告格式 <span className="text-[var(--danger)]">*</span>
            </legend>
            {allFormats.map((format) => (
              <label
                key={format.value}
                className="flex cursor-pointer items-start gap-[var(--space-2)] rounded-[var(--radius-sm)] border border-[var(--border-soft)] px-[var(--space-3)] py-[var(--space-2)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)]"
              >
                <Checkbox
                  checked={formats.includes(format.value)}
                  onCheckedChange={() => toggle(format.value)}
                  aria-label={format.label}
                />
                <span className="flex flex-col gap-[2px]">
                  <span className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
                    {format.label}
                  </span>
                  <span className="text-[var(--text-xs)] text-[var(--muted)]">{format.desc}</span>
                </span>
              </label>
            ))}
          </fieldset>

          <p className="flex items-start gap-[var(--space-1)] rounded-[var(--radius-sm)] bg-[var(--surface-warm)] px-[var(--space-2)] py-[var(--space-2)] text-[var(--text-xs)] text-[var(--muted)]">
            <Lock size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
            报告中的样例值均为掩码值，不导出完整明文敏感数据；生成与下载都会写入审计日志。
          </p>

          {error ? (
            <p role="alert" className="text-[var(--text-sm)] text-[var(--danger)]">
              {error}
            </p>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={generating}>
            取消
          </Button>
          <Button onClick={onSubmit} loading={generating}>
            <Plus size={16} aria-hidden="true" />
            生成报告
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ---------------------------------------------------------------- 口径提示 */

/** 报告脱敏口径提示 */
export function ReportMaskingHint() {
  return (
    <p className="flex items-start gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
      <Lock size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
      报告样例均为掩码值，不含完整明文；生成与下载动作均记录审计日志。
    </p>
  );
}
