'use client';

/**
 * dbdlp 控制台 · P01 概览业务组件
 *
 * 包含：*OverviewKpiStrip、*DataSourceStatusTable、*ScanProgressCard、
 *       *RiskDistributionCard、*RecentFindingsTable、*LowPeakHint
 *
 * 首屏直接展示真实产品内容，无营销 Hero（§1.3、§10 应做 1）。
 */

import * as React from 'react';
import Link from 'next/link';
import {
  Activity,
  CircleExclamationPoint,
  Database,
  LoaderCircle,
  Table2,
  TrendingUp,
  TriangleExclamationPoint,
} from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle, Badge, Skeleton } from '@/components/ui/primitives';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, NumericCell } from '@/components/ui/table';
import { EChart, buildPieOption } from '@/components/charts/EChart';
import { riskColorSeries } from '@/lib/chart-theme';
import { formatDateTime, formatPercent, toUiRisk, UI_RISK_LABEL } from '@/lib/format';
import type { ColumnFinding, DataSource, DashboardSummary, ScanTask } from '@/types/api';

/* ------------------------------------------------------- *OverviewKpiStrip */

export function OverviewKpiStrip({
  instanceTotal,
  abnormalCount,
  runningTasks,
  highRiskFields,
  loading,
}: {
  instanceTotal: number;
  abnormalCount: number;
  runningTasks: number;
  highRiskFields: number;
  loading?: boolean;
}) {
  const items = [
    {
      label: '数据源总数',
      value: instanceTotal,
      Icon: Database,
      tone: 'var(--fg)',
      href: '/datasources',
    },
    {
      label: '连接异常',
      value: abnormalCount,
      Icon: CircleExclamationPoint,
      tone: 'var(--danger)',
      href: '/datasources',
    },
    {
      label: '运行中任务',
      value: runningTasks,
      Icon: LoaderCircle,
      tone: 'var(--info)',
      href: '/tasks',
    },
    {
      label: '高风险字段',
      value: highRiskFields,
      Icon: TriangleExclamationPoint,
      tone: 'var(--risk-high-fg)',
      href: '/findings',
    },
  ];

  return (
    <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] sm:grid-cols-2 lg:grid-cols-4">
      {items.map((item) => (
        <Link key={item.label} href={item.href} className="focus-visible:outline-none">
          <Card interactive className="h-full">
            <CardContent className="flex items-start justify-between gap-[var(--space-3)] p-[var(--card-padding)]">
              <div className="flex flex-col gap-[var(--space-1)]">
                <span className="text-[var(--text-xs)] text-[var(--muted)]">{item.label}</span>
                {loading ? (
                  <Skeleton className="h-[28px] w-[72px]" />
                ) : (
                  <span
                    className="font-[var(--font-mono)] text-[var(--text-2xl)] font-[var(--weight-announce)] tabular-nums leading-[var(--leading-tight)]"
                    style={{ color: item.tone }}
                  >
                    {item.value}
                  </span>
                )}
              </div>
              <item.Icon size={24} aria-hidden="true" style={{ color: item.tone }} />
            </CardContent>
          </Card>
        </Link>
      ))}
    </div>
  );
}

/* -------------------------------------------------- *DataSourceStatusTable */

export function DataSourceStatusTable({
  items,
  loading,
}: {
  items: DataSource[];
  loading?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <div className="flex flex-col gap-[var(--space-1)]">
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <Database size={20} aria-hidden="true" />
            数据源列表
          </CardTitle>
          <CardDescription>主机与地址按脱敏策略展示，样例不显示完整明文</CardDescription>
        </div>
        <Link
          href="/datasources"
          className="rounded-[var(--radius-sm)] px-[var(--space-2)] py-[var(--space-1)] text-[var(--text-sm)] text-[var(--accent)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
        >
          查看全部
        </Link>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex flex-col gap-[var(--space-2)]">
            <Skeleton className="h-[32px] w-full" />
            <Skeleton className="h-[32px] w-full" />
            <Skeleton className="h-[32px] w-full" />
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center gap-[var(--space-2)] py-[var(--space-6)]">
            <Database size={24} aria-hidden="true" className="text-[var(--meta)]" />
            <p className="text-[var(--text-sm)] text-[var(--muted)]">配置第一个数据源</p>
            <Link
              href="/datasources/new"
              className="text-[var(--text-sm)] text-[var(--accent)] underline underline-offset-2"
            >
              新建数据源
            </Link>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>实例标识</TableHead>
                <TableHead>主机:端口</TableHead>
                <TableHead>只读账号</TableHead>
                <TableHead>连接状态</TableHead>
                <TableHead>最近扫描</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.slice(0, 8).map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-[var(--font-mono)] text-[var(--fg)]">{item.name}</TableCell>
                  <TableCell className="font-[var(--font-mono)]">
                    {item.host}:{item.port}
                  </TableCell>
                  <TableCell className="font-[var(--font-mono)]">{item.username}</TableCell>
                  <TableCell>
                    <ConnectionStateBadge state={item.connectionState} />
                  </TableCell>
                  <TableCell className="text-[var(--muted)]">
                    {formatDateTime(item.lastScanAt)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

/** 连接状态徽章（图标 + 中文文字，不靠颜色单独表达） */
export function ConnectionStateBadge({ state }: { state: string }) {
  if (state === 'ok') {
    return <Badge tone="success">连接正常</Badge>;
  }
  if (state === 'failed') {
    return <Badge tone="danger">连接异常</Badge>;
  }
  if (state === 'testing') {
    return <Badge tone="info">验证中</Badge>;
  }
  return <Badge tone="neutral">未验证</Badge>;
}

/* ------------------------------------------------------ *ScanProgressCard */

export function ScanProgressCard({
  tasks,
  loading,
}: {
  tasks: ScanTask[];
  loading?: boolean;
}) {
  const running = tasks.find((t) => t.status === 'running');
  const total = running?.summary?.totalTables ?? 0;
  const done = running?.summary?.doneTables ?? 0;
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Activity size={20} aria-hidden="true" />
          扫描进度
        </CardTitle>
        <CardDescription>运行中任务的表级完成情况</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-3)]">
        {loading ? (
          <Skeleton className="h-[64px] w-full" />
        ) : running ? (
          <>
            <div className="flex items-center justify-between">
              <span className="font-[var(--font-mono)] text-[var(--text-sm)] text-[var(--fg)]">
                {running.dataSourceName ?? running.dataSourceId}
              </span>
              <Badge tone="info">运行中</Badge>
            </div>
            <div
              role="progressbar"
              aria-valuenow={pct}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-label="扫描表完成进度"
              className="h-[6px] w-full overflow-hidden rounded-[var(--progress-radius)] bg-[var(--progress-track)]"
            >
              <div
                className="h-full rounded-[var(--progress-radius)] bg-[var(--progress-fill)]"
                style={{ width: `${pct}%` }}
              />
            </div>
            <p className="text-[var(--text-xs)] text-[var(--muted)]">
              已完成 {done} / {total} 张表（{pct}%）
            </p>
            <Link
              href={`/tasks/${running.id}`}
              className="text-[var(--text-sm)] text-[var(--accent)] underline underline-offset-2"
            >
              查看任务详情
            </Link>
          </>
        ) : (
          <p className="py-[var(--space-4)] text-center text-[var(--text-sm)] text-[var(--muted)]">
            当前没有运行中的扫描任务
          </p>
        )}
        <LowPeakHint />
      </CardContent>
    </Card>
  );
}

/** *LowPeakHint：低峰窗口提示 */
export function LowPeakHint() {
  return (
    <p className="rounded-[var(--radius-sm)] bg-[var(--surface-warm)] px-[var(--space-2)] py-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
      建议在业务低峰期执行扫描任务，避免影响生产库读写。
    </p>
  );
}

/* ---------------------------------------------------- *RiskDistributionCard */

export function RiskDistributionCard({
  summary,
  loading,
}: {
  summary: DashboardSummary | null;
  loading?: boolean;
}) {
  const option = React.useCallback(
    (theme: Parameters<typeof buildPieOption>[0]) => {
      const counts = summary?.riskCounts ?? {};
      const data = [
        { name: UI_RISK_LABEL.high, value: counts.high ?? 0 },
        { name: UI_RISK_LABEL.medium, value: counts.medium ?? 0 },
        { name: UI_RISK_LABEL.low, value: counts.low ?? 0 },
        { name: UI_RISK_LABEL.pending, value: counts.pending ?? 0 },
      ];
      const total = data.reduce((sum, item) => sum + item.value, 0);
      return buildPieOption(theme, data, riskColorSeries(theme), String(total));
    },
    [summary],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Table2 size={20} aria-hidden="true" />
          风险分布
        </CardTitle>
        <CardDescription>按字段数统计，高→中→低→待复核固定顺序</CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-[260px] w-full" />
        ) : (
          <EChart option={option} ariaLabel="风险等级分布环形图" height={260} />
        )}
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------ *RecentFindingsTable */

export function RecentFindingsTable({
  items,
  loading,
}: {
  items: ColumnFinding[];
  loading?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <div className="flex flex-col gap-[var(--space-1)]">
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <TrendingUp size={20} aria-hidden="true" />
            最近字段台账
          </CardTitle>
          <CardDescription>最近扫描命中的字段，样例仅显示掩码值</CardDescription>
        </div>
        <Link
          href="/findings"
          className="rounded-[var(--radius-sm)] px-[var(--space-2)] py-[var(--space-1)] text-[var(--text-sm)] text-[var(--accent)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
        >
          查看全部
        </Link>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-[200px] w-full" />
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center gap-[var(--space-2)] py-[var(--space-6)]">
            <Table2 size={24} aria-hidden="true" className="text-[var(--meta)]" />
            <p className="text-[var(--text-sm)] text-[var(--muted)]">暂无命中字段</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>实例</TableHead>
                <TableHead>模式.表.字段</TableHead>
                <TableHead>敏感类型</TableHead>
                <TableHead>命中率</TableHead>
                <TableHead>风险等级</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-[var(--font-mono)]">{item.instanceId}</TableCell>
                  <TableCell className="font-[var(--font-mono)]">
                    {item.schemaName}.{item.tableName}.{item.columnName}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-[var(--space-1)]">
                      {item.sensitiveTypes.slice(0, 2).map((type) => (
                        <Badge key={type} tone="neutral">
                          {type}
                        </Badge>
                      ))}
                      {item.sensitiveTypes.length > 2 ? (
                        <Badge tone="neutral">+{item.sensitiveTypes.length - 2}</Badge>
                      ) : null}
                    </div>
                  </TableCell>
                  <NumericCell>{formatPercent(item.hitRatio)}</NumericCell>
                  <TableCell>
                    <RiskBadge level={item.riskLevel} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

/** *RiskBadge：色块 + 图标 + 中文文字三重编码（§4.4 规则 ④） */
export function RiskBadge({ level }: { level: string }) {
  const ui = toUiRisk(level);
  const tone =
    ui === 'high' ? 'danger' : ui === 'medium' ? 'warning' : ui === 'low' ? 'success' : 'neutral';
  const Icon =
    ui === 'high'
      ? CircleExclamationPoint
      : ui === 'medium'
        ? Table2
        : ui === 'low'
          ? Activity
          : LoaderCircle;
  return (
    <Badge tone={tone}>
      <Icon size={12} aria-hidden="true" />
      {UI_RISK_LABEL[ui]}
    </Badge>
  );
}
