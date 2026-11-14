'use client';

/**
 * dbdlp 控制台 · P06 扫描任务详情业务组件
 *
 * 包含：*TaskHeader、*TaskProgressSegments、*ObjectProgressTable、
 *       *FailedObjectTable、*AuditLogViewer、*ParamSnapshot
 *
 * 规范（04 文档 §2.3、§4.1 C9）：扫描进度不用 ECharts，用分段条 + 数字。
 * 暂停 / 继续 / 重试均有中文二次确认；审计日志标注「已脱敏」。
 */

import * as React from 'react';
import {
  Activity,
  CircleCheck,
  CircleSlash2,
  CircleX,
  ClipboardList,
  Clock,
  GitBranch,
  Lock,
  Pause,
  Play,
  RotateCcw,
  SlidersHorizontal,
  TriangleExclamationPoint,
} from 'lucide-react';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/overlays';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, NumericCell } from '@/components/ui/table';
import { formatDateTime } from '@/lib/format';
import type { AuditLog, FailedObject, ScanTask, TaskProgress } from '@/types/api';
import { TaskStatusBadge } from '@/components/business/tasks';

/* --------------------------------------------------------------- *TaskHeader */

/** 任务摘要卡 */
export function TaskHeader({
  task,
  onPause,
  onResume,
  onRetry,
  busy,
}: {
  task: ScanTask;
  onPause: () => void;
  onResume: () => void;
  onRetry: () => void;
  busy?: boolean;
}) {
  const canPause = task.status === 'running';
  const canResume = task.status === 'paused';
  const failedCount = task.summary?.failedTables ?? 0;

  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <CardTitle className="flex items-center gap-[var(--space-2)] font-[var(--font-mono)]">
            {task.id}
          </CardTitle>
          <CardDescription>
            数据源 {task.dataSourceName ?? task.dataSourceId} · 创建于{' '}
            {formatDateTime(task.createdAt)}
          </CardDescription>
        </div>
        <div className="flex flex-wrap items-center gap-[var(--space-2)]">
          <TaskStatusBadge status={task.status} />
          {canPause ? (
            <Button variant="secondary" size="sm" disabled={busy} onClick={onPause}>
              <Pause size={16} aria-hidden="true" />
              暂停任务
            </Button>
          ) : null}
          {canResume ? (
            <Button variant="secondary" size="sm" disabled={busy} onClick={onResume}>
              <Play size={16} aria-hidden="true" />
              继续执行
            </Button>
          ) : null}
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || failedCount === 0}
            onClick={onRetry}
          >
            <RotateCcw size={16} aria-hidden="true" />
            重试失败对象（{failedCount}）
          </Button>
        </div>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-[var(--space-3)] sm:grid-cols-4">
        <Metric label="已扫描表" value={`${task.summary?.doneTables ?? 0} / ${task.summary?.totalTables ?? 0}`} />
        <Metric label="命中字段" value={`${task.summary?.hitColumns ?? 0} / ${task.summary?.totalColumns ?? 0}`} />
        <Metric label="失败表" value={String(task.summary?.failedTables ?? 0)} tone="var(--danger)" />
        <Metric label="跳过表" value={String(task.summary?.skippedTables ?? 0)} />
      </CardContent>
    </Card>
  );
}

/** 单个指标 */
function Metric({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="flex flex-col gap-[var(--space-1)] rounded-[var(--radius-sm)] bg-[var(--surface-warm)] px-[var(--space-3)] py-[var(--space-2)]">
      <span className="text-[var(--text-xs)] text-[var(--muted)]">{label}</span>
      <span
        className="font-[var(--font-mono)] text-[var(--text-lg)] font-[var(--weight-emphasize)] tabular-nums text-[var(--fg)]"
        style={tone ? { color: tone } : undefined}
      >
        {value}
      </span>
    </div>
  );
}

/* ------------------------------------------------------ *TaskProgressSegments */

/** 扫描进度分段条（§4.1 C9：不用 ECharts） */
export function TaskProgressSegments({ progress }: { progress: TaskProgress | null }) {
  const total = progress?.totalTables ?? 0;
  const done = progress?.doneTables ?? 0;
  const failed = progress?.failedCount ?? 0;
  const skipped = Math.max(0, total - done - failed);

  const segments = [
    { key: 'done', label: '已完成', value: done, tone: 'var(--progress-fill)' },
    { key: 'failed', label: '失败', value: failed, tone: 'var(--danger)' },
    { key: 'skipped', label: '跳过', value: skipped, tone: 'var(--border-strong)' },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Activity size={20} aria-hidden="true" />
          扫描进度
        </CardTitle>
        <CardDescription>按表统计；口径为已扫描表，不含 BLOB 跳过列</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-3)]">
        <div
          role="progressbar"
          aria-valuenow={total > 0 ? Math.round((done / total) * 100) : 0}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label="扫描表完成进度"
          className="flex h-[8px] w-full overflow-hidden rounded-[var(--progress-radius)] bg-[var(--progress-track)]"
        >
          {segments.map((seg) => {
            const pct = total > 0 ? (seg.value / total) * 100 : 0;
            if (pct <= 0) return null;
            return (
              <span
                key={seg.key}
                style={{ width: `${pct}%`, backgroundColor: seg.tone }}
                className="h-full"
              />
            );
          })}
        </div>

        <div className="flex flex-wrap gap-[var(--space-4)]">
          {segments.map((seg) => (
            <span key={seg.key} className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--fg-2)]">
              <span
                aria-hidden="true"
                className="h-[8px] w-[8px] rounded-[var(--radius-pill)]"
                style={{ backgroundColor: seg.tone }}
              />
              {seg.label}
              <span className="font-[var(--font-mono)] tabular-nums text-[var(--fg)]">{seg.value}</span>
            </span>
          ))}
        </div>

        <div className="flex flex-wrap gap-[var(--space-4)] text-[var(--text-xs)] text-[var(--muted)]">
          <span>
            已扫描列：
            <span className="font-[var(--font-mono)] tabular-nums text-[var(--fg-2)]">
              {progress?.doneColumns ?? 0} / {progress?.totalColumns ?? 0}
            </span>
          </span>
          <span>
            完成度：
            <span className="font-[var(--font-mono)] tabular-nums text-[var(--fg-2)]">
              {total > 0 ? Math.round((done / total) * 100) : 0}%
            </span>
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------- *ObjectProgressTable */

/** 表级进度明细 */
export function ObjectProgressTable({
  totalTables,
  doneTables,
  failedTables,
  skippedTables,
}: {
  totalTables: number;
  doneTables: number;
  failedTables: number;
  skippedTables: number;
}) {
  const rows = [
    { label: '已完成表', value: doneTables, Icon: CircleCheck, tone: 'var(--success)' },
    { label: '失败表', value: failedTables, Icon: CircleX, tone: 'var(--danger)' },
    { label: '跳过表', value: skippedTables, Icon: CircleSlash2, tone: 'var(--muted)' },
    { label: '总表数', value: totalTables, Icon: Activity, tone: 'var(--fg)' },
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Activity size={20} aria-hidden="true" />
          表进度汇总
        </CardTitle>
        <CardDescription>失败表记录在任务摘要中，可整任务重试</CardDescription>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-[var(--space-3)] sm:grid-cols-4">
        {rows.map((row) => (
          <div
            key={row.label}
            className="flex flex-col gap-[var(--space-1)] rounded-[var(--radius-sm)] border border-[var(--border-soft)] px-[var(--space-3)] py-[var(--space-2)]"
          >
            <span className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
              <row.Icon size={12} aria-hidden="true" style={{ color: row.tone }} />
              {row.label}
            </span>
            <span
              className="font-[var(--font-mono)] text-[var(--text-xl)] font-[var(--weight-emphasize)] tabular-nums"
              style={{ color: row.tone }}
            >
              {row.value}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

/* -------------------------------------------------------- *FailedObjectTable */

/** 失败 / 跳过对象清单（可重试） */
export function FailedObjectTable({
  objects,
  onRetry,
}: {
  objects: FailedObject[];
  onRetry?: (ids: string[]) => void;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <div className="flex flex-col gap-[var(--space-1)]">
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <TriangleExclamationPoint size={20} aria-hidden="true" />
            失败与跳过对象
          </CardTitle>
          <CardDescription>失败对象可在任务详情中整批重试，重试会重新建立读连接</CardDescription>
        </div>
        {objects.length > 0 && onRetry ? (
          <Button variant="secondary" size="sm" onClick={() => onRetry(objects.map((o) => o.id))}>
            <RotateCcw size={16} aria-hidden="true" />
            重试全部
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[160px]">模式</TableHead>
              <TableHead className="w-[180px]">表名</TableHead>
              <TableHead>失败原因</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {objects.length === 0 ? (
              <TableEmptyRow colSpan={3} message="本次任务没有失败或跳过的对象" />
            ) : (
              objects.map((obj) => (
                <TableRow key={obj.id}>
                  <TableCell className="font-[var(--font-mono)]">{obj.schemaName}</TableCell>
                  <TableCell className="font-[var(--font-mono)]">{obj.tableName}</TableCell>
                  <TableCell className="text-[var(--danger)]">{obj.reason}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/* ----------------------------------------------------------- *AuditLogViewer */

/** 审计日志查看器（内容已脱敏） */
export function AuditLogViewer({ logs }: { logs: AuditLog[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ClipboardList size={20} aria-hidden="true" />
          审计日志
        </CardTitle>
        <CardDescription className="flex items-center gap-[var(--space-1)]">
          <Lock size={12} aria-hidden="true" />
          日志内容已脱敏，IP 前两段以星号替换
        </CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[150px]">时间</TableHead>
              <TableHead className="w-[120px]">操作者</TableHead>
              <TableHead className="w-[140px]">动作</TableHead>
              <TableHead>对象</TableHead>
              <TableHead className="w-[140px]">来源 IP</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.length === 0 ? (
              <TableEmptyRow colSpan={5} message="暂无该任务的审计记录" />
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="text-[var(--muted)]">{formatDateTime(log.createdAt)}</TableCell>
                  <TableCell className="font-[var(--font-mono)]">{log.actor}</TableCell>
                  <TableCell>{log.action}</TableCell>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">{log.target}</TableCell>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">{log.ip}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/* -------------------------------------------------------------- *ParamSnapshot */

/** 参数快照（发起时的实际生效参数） */
export function ParamSnapshot({ task }: { task: ScanTask }) {
  const rows = [
    { label: '采样行数', value: String(task.sampleRows ?? '—（取服务端默认）') },
    { label: '批大小', value: String(task.batchSize ?? '—（取服务端默认）') },
    { label: '并发度', value: String(task.concurrency ?? '—（取服务端默认）') },
    { label: '低峰窗口', value: task.lowPeakWindow || '不限时段' },
    { label: '规则版本', value: task.ruleVersion || '—' },
    { label: '开始时间', value: formatDateTime(task.startedAt) },
    { label: '结束时间', value: formatDateTime(task.finishedAt) },
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <SlidersHorizontal size={20} aria-hidden="true" />
          参数快照
        </CardTitle>
        <CardDescription>任务发起时的实际生效参数，用于复盘与审计</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-2)]">
        {rows.map((row) => (
          <div
            key={row.label}
            className="flex items-center justify-between gap-[var(--space-3)] border-b border-[var(--border-soft)] py-[var(--space-1)] last:border-b-0"
          >
            <span className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
              {row.label === '规则版本' ? (
                <GitBranch size={12} aria-hidden="true" />
              ) : row.label.includes('时间') ? (
                <Clock size={12} aria-hidden="true" />
              ) : null}
              {row.label}
            </span>
            <span className="font-[var(--font-mono)] text-[var(--text-sm)] tabular-nums text-[var(--fg-2)]">
              {row.value}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------- 操作二次确认对话框 */

/** 暂停 / 继续 / 重试的二次确认 */
export function TaskActionDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  pending,
  error,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  confirmLabel: string;
  pending: boolean;
  error: string | null;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {error ? (
          <p role="alert" className="text-[var(--text-sm)] text-[var(--danger)]">
            {error}
          </p>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={pending}>
            取消
          </Button>
          <Button onClick={onConfirm} loading={pending}>
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** 状态徽章（用于 Tab 区） */
export function TaskStateTag({ children }: { children: React.ReactNode }) {
  return <Badge tone="neutral">{children}</Badge>;
}
