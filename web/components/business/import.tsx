'use client';

/**
 * dbdlp 控制台 · P04 批量导入业务组件
 *
 * 包含：*ImportDropzone、*ImportProgressPanel、*ImportResultSummary、
 *       *ImportResultGrid、*ImportErrorPanel
 *
 * 口径（04 文档 §3.4）：四阶段纵向流程（准备 → 提交门禁 → 结果回显 → 失败处置），
 * 逐条门禁 + 部分成功——通过的行入库，失败行不入库并逐行中文回显。
 */

import * as React from 'react';
import {
  ChevronDown,
  ChevronRight,
  CircleCheck,
  CircleDot,
  CircleSlash2,
  CircleX,
  FileBraces,
  FileSpreadsheet,
  Info,
  LoaderCircle,
  Trash,
  TriangleExclamationPoint,
  Upload,
} from 'lucide-react';
import { Alert, Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Progress } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, NumericCell } from '@/components/ui/table';
import { formatBytes } from '@/lib/format';
import type { ImportErrorRow, ImportPhase } from '@/stores/useImportStore';

/* ------------------------------------------------------- 阶段定义（四阶段） */

interface StageDef {
  key: ImportPhase;
  index: number;
  title: string;
  desc: string;
}

/** §3.4 四阶段（准备 → 提交门禁 → 结果回显 → 失败处置） */
export const IMPORT_STAGES: StageDef[] = [
  { key: 'idle', index: 1, title: '准备文件', desc: '选择 Excel 或 YAML 文件，前端预检格式与体积' },
  { key: 'selected', index: 2, title: '提交门禁', desc: '服务端逐条连接测试并逐条独立事务入库' },
  { key: 'uploading', index: 3, title: '结果回显', desc: '展示成功与失败行数，失败行逐行中文说明' },
  { key: 'done', index: 4, title: '失败处置', desc: '按失败原因修正后重新导入，重复导入幂等去重' },
];

/** 阶段序号映射 */
export function stageIndexOf(phase: ImportPhase): number {
  switch (phase) {
    case 'idle':
      return 1;
    case 'selected':
    case 'uploading':
    case 'processing':
      return 2;
    case 'done':
    case 'failed':
      return 4;
    default:
      return 1;
  }
}

/* ---------------------------------------------------------- *ImportDropzone */

/** 文件投放区：扩展名与体积预检在 store 内完成（§3.4 阶段 1） */
export function ImportDropzone({
  fileName,
  fileSize,
  error,
  disabled,
  onSelect,
  onClear,
}: {
  fileName: string;
  fileSize: number;
  error: string | null;
  disabled?: boolean;
  onSelect: (file: File) => void;
  onClear: () => void;
}) {
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const [dragging, setDragging] = React.useState(false);

  const handleFiles = (files: FileList | null) => {
    const file = files?.[0];
    if (file) onSelect(file);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Upload size={20} aria-hidden="true" />
          选择导入文件
        </CardTitle>
        <CardDescription>
          支持 .xlsx、.xls、.yaml、.yml，单文件不超过 10MB；文件内容仅上传到后端，不落地前端存储
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-3)]">
        <div
          onDragOver={(e) => {
            e.preventDefault();
            if (!disabled) setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragging(false);
            if (!disabled) handleFiles(e.dataTransfer.files);
          }}
          className={
            'flex min-h-[160px] flex-col items-center justify-center gap-[var(--space-2)] rounded-[var(--radius-md)] border border-dashed px-[var(--space-4)] py-[var(--space-6)] text-center transition-colors duration-[var(--motion-fast)] ' +
            (dragging
              ? 'border-[var(--accent)] bg-[var(--accent-soft)]'
              : 'border-[var(--border-strong)] bg-[var(--surface-warm)]')
          }
        >
          <FileSpreadsheet size={24} aria-hidden="true" className="text-[var(--muted)]" />
          <p className="text-[var(--text-sm)] text-[var(--fg)]">把文件拖到这里，或从本机选择</p>
          <p className="text-[var(--text-xs)] text-[var(--muted)]">
            Excel 模板含字段说明；YAML 文件按 dataSources 列表组织
          </p>
          <input
            ref={inputRef}
            id="import-file"
            type="file"
            accept=".xlsx,.xls,.yaml,.yml"
            className="sr-only"
            disabled={disabled}
            onChange={(e) => handleFiles(e.target.files)}
          />
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={disabled}
            onClick={() => inputRef.current?.click()}
          >
            <Upload size={16} aria-hidden="true" />
            选择文件
          </Button>
        </div>

        {fileName ? (
          <div className="flex items-center justify-between gap-[var(--space-3)] rounded-[var(--radius-sm)] border border-[var(--border-soft)] px-[var(--space-3)] py-[var(--space-2)]">
            <span className="flex min-w-0 items-center gap-[var(--space-2)] text-[var(--text-sm)] text-[var(--fg)]">
              <FileBraces size={16} aria-hidden="true" className="shrink-0 text-[var(--muted)]" />
              <span className="truncate font-[var(--font-mono)]">{fileName}</span>
              <span className="shrink-0 text-[var(--text-xs)] text-[var(--muted)]">{formatBytes(fileSize)}</span>
            </span>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              disabled={disabled}
              onClick={onClear}
              aria-label={`移除已选文件 ${fileName}`}
            >
              <Trash size={16} aria-hidden="true" />
            </Button>
          </div>
        ) : null}

        {error ? (
          <Alert tone="danger" title="文件校验未通过">
            {error}
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  );
}

/* ----------------------------------------------------- *ImportProgressPanel */

/** 阶段进度面板：四阶段纵向推进 */
export function ImportProgressPanel({
  phase,
  progress,
  fileName,
}: {
  phase: ImportPhase;
  progress: number;
  fileName: string;
}) {
  const current = stageIndexOf(phase);
  const busy = phase === 'uploading' || phase === 'processing';

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          {busy ? (
            <LoaderCircle size={20} aria-hidden="true" className="animate-spin" />
          ) : (
            <CircleDot size={20} aria-hidden="true" />
          )}
          导入流程
        </CardTitle>
        <CardDescription>逐条门禁：每行独立做连接测试，通过的行入库，失败行不入库</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-3)]">
        <ol className="flex flex-col gap-[var(--space-2)]">
          {IMPORT_STAGES.map((stage) => {
            const done = current > stage.index;
            const active = current === stage.index;
            return (
              <li key={stage.title} className="flex items-start gap-[var(--space-3)]">
                <span
                  aria-hidden="true"
                  className={
                    'mt-[2px] flex h-[20px] w-[20px] shrink-0 items-center justify-center rounded-[var(--radius-pill)] border text-[var(--text-xs)] ' +
                    (done
                      ? 'border-[var(--success-border)] bg-[var(--success-bg)] text-[var(--success)]'
                      : active
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] text-[var(--accent)]'
                        : 'border-[var(--border)] text-[var(--meta)]')
                  }
                >
                  {done ? <CircleCheck size={12} /> : stage.index}
                </span>
                <span className="flex flex-col gap-[2px]">
                  <span
                    className={
                      'text-[var(--text-sm)] font-[var(--weight-emphasize)] ' +
                      (active ? 'text-[var(--fg)]' : 'text-[var(--fg-2)]')
                    }
                  >
                    {stage.title}
                  </span>
                  <span className="text-[var(--text-xs)] text-[var(--muted)]">{stage.desc}</span>
                </span>
              </li>
            );
          })}
        </ol>

        {busy ? (
          <Progress value={progress} label={`正在处理导入文件（${progress}%），服务端逐行做连接测试`} />
        ) : null}

        {fileName ? (
          <p className="text-[var(--text-xs)] text-[var(--muted)]">
            当前文件：<span className="font-[var(--font-mono)]">{fileName}</span>
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}

/* --------------------------------------------------- *ImportResultSummary */

/** 结果摘要：总数 / 成功 / 失败三数并列 */
export function ImportResultSummary({
  total,
  succeeded,
  failed,
}: {
  total: number;
  succeeded: number;
  failed: number;
}) {
  const items = [
    { label: '解析总行数', value: total, tone: 'var(--fg)' },
    { label: '导入成功', value: succeeded, tone: 'var(--success)' },
    { label: '导入失败', value: failed, tone: 'var(--danger)' },
  ];
  return (
    <div className="grid grid-cols-1 gap-[var(--gutter-tablet)] sm:grid-cols-3">
      {items.map((item) => (
        <Card key={item.label}>
          <CardContent className="flex flex-col gap-[var(--space-1)] p-[var(--card-padding)]">
            <span className="text-[var(--text-xs)] text-[var(--muted)]">{item.label}</span>
            <span
              className="font-[var(--font-mono)] text-[var(--text-2xl)] font-[var(--weight-announce)] tabular-nums leading-[var(--leading-tight)]"
              style={{ color: item.tone }}
            >
              {item.value}
            </span>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

/* -------------------------------------------------------- *ImportResultGrid */

/** 结果回显网格：成功/失败两类结果并列展示 */
export function ImportResultGrid({
  total,
  succeeded,
  failed,
}: {
  total: number;
  succeeded: number;
  failed: number;
}) {
  return (
    <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <CircleCheck size={20} aria-hidden="true" />
            成功入库
          </CardTitle>
          <CardDescription>逐条通过连接测试并写入后端存储库的数据源</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="font-[var(--font-mono)] text-[var(--text-3xl)] font-[var(--weight-announce)] tabular-nums text-[var(--success)]">
            {succeeded}
          </p>
          <p className="mt-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
            占解析总行数 {total > 0 ? Math.round((succeeded / total) * 100) : 0}%
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <CircleX size={20} aria-hidden="true" />
            未入库
          </CardTitle>
          <CardDescription>连接测试未通过或字段不合规的行，整条不落库</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="font-[var(--font-mono)] text-[var(--text-3xl)] font-[var(--weight-announce)] tabular-nums text-[var(--danger)]">
            {failed}
          </p>
          <p className="mt-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
            失败原因逐行列在下方清单，可按原因修正后重新导入
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

/* ------------------------------------------------------- *ImportErrorPanel */

/**
 * 失败处置清单：可展开的逐行明细表（默认展开，避免逐行原因被折叠隐藏）。
 * code=0 部分成功与 code=2001 整批失败都复用此组件，逐行显示行号 / 实例标识 / 中文原因。
 */
export function ImportErrorPanel({
  errors,
  defaultOpen = true,
}: {
  errors: ImportErrorRow[];
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = React.useState(defaultOpen);
  const hasName = errors.some((item) => Boolean(item.name));
  const colSpan = hasName ? 3 : 2;

  return (
    <Card>
      <CardHeader>
        <button
          type="button"
          aria-expanded={open}
          aria-controls="import-error-detail"
          onClick={() => setOpen((value) => !value)}
          className="flex w-full items-center justify-between gap-[var(--space-2)] rounded-[var(--radius-sm)] text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--accent)]"
        >
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <TriangleExclamationPoint size={20} aria-hidden="true" />
            失败行清单（{errors.length} 行）
          </CardTitle>
          <span className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
            {open ? '收起明细' : '展开明细'}
            {open ? <ChevronDown size={16} aria-hidden="true" /> : <ChevronRight size={16} aria-hidden="true" />}
          </span>
        </button>
        <CardDescription>
          行号对应导入文件中的数据行（不含表头），逐行原因由后端中文回显，全部失败时零写入
        </CardDescription>
      </CardHeader>
      <CardContent className={open ? 'p-0' : 'hidden'} id="import-error-detail">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[96px]">行号</TableHead>
              {hasName ? <TableHead className="w-[160px]">实例标识</TableHead> : null}
              <TableHead>失败原因</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {errors.length === 0 ? (
              <TableEmptyRow colSpan={colSpan} message="没有失败行，全部数据源均已入库" />
            ) : (
              errors.map((item, index) => (
                <TableRow key={`${item.row}-${index}-${item.message}`}>
                  <NumericCell>
                    <span className="text-[var(--fg)]">{item.row}</span>
                  </NumericCell>
                  {hasName ? (
                    <TableCell className="font-[var(--font-mono)] text-[var(--fg-2)]">
                      {item.name ?? '—'}
                    </TableCell>
                  ) : null}
                  <TableCell className="text-[var(--danger)]">{item.message}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/* --------------------------------------------------------------- 状态提示 */

/** 阶段提示条（用于页面顶部） */
export function ImportPhaseNotice({
  phase,
  failed,
  allFailed = false,
  total = 0,
  serverMessage = null,
}: {
  phase: ImportPhase;
  failed: number;
  /** 后端 code=2001：整批零写入，逐行明细仍须回显 */
  allFailed?: boolean;
  total?: number;
  serverMessage?: string | null;
}) {
  if (allFailed) {
    return (
      <Alert tone="danger" title="全部未入库">
        <span className="flex flex-col gap-[var(--space-1)]">
          <span>
            共解析 {total} 行，成功 0 行、失败 {failed} 行；后端逐条门禁均未通过，本次零写入。
          </span>
          {serverMessage ? <span>{serverMessage}</span> : null}
          <span>下方失败行清单逐行给出行号与原因，修正后可重新导入同一文件。</span>
        </span>
      </Alert>
    );
  }
  if (phase === 'failed') {
    return (
      <Alert tone="danger" title="导入未完成">
        服务端返回了失败结果，请查看失败行清单。
      </Alert>
    );
  }
  if (phase === 'done' && failed > 0) {
    return (
      <Alert tone="warning" title="部分成功">
        成功的数据源已入库，失败 {failed} 行未入库。修正后可重新导入同一文件，重复项按幂等去重处理。
      </Alert>
    );
  }
  if (phase === 'done') {
    return (
      <Alert tone="success" title="导入完成">
        全部数据源均通过连接测试并入库。
      </Alert>
    );
  }
  return (
    <Alert tone="info" title="导入口径">
      <span className="inline-flex items-start gap-[var(--space-1)]">
        <Info size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
        <span>
          批量导入采用逐条门禁：每一行单独做连接测试并单独入库，任一行失败不影响其它行。
        </span>
      </span>
    </Alert>
  );
}

/** 跳过的行标记（用于结果说明） */
export function SkippedHint() {
  return (
    <span className="inline-flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
      <CircleSlash2 size={14} aria-hidden="true" />
      重复导入同一实例标识时按幂等去重处理，不产生重复数据源
    </span>
  );
}

/** 徽章：导入成功/失败计数（P09 复用） */
export function ImportCountBadge({ succeeded, failed }: { succeeded: number; failed: number }) {
  if (failed === 0) {
    return (
      <Badge tone="success">
        <CircleCheck size={12} aria-hidden="true" />
        全部成功
      </Badge>
    );
  }
  return (
    <Badge tone="warning">
      <TriangleExclamationPoint size={12} aria-hidden="true" />
      部分成功
    </Badge>
  );
}
