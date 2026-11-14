'use client';

/**
 * dbdlp 控制台 · P10 报告预览业务组件
 *
 * 包含：*ReportAnchorNav、*ReportSummaryKpis、*ReportChartSet、*RiskListTable、
 *       *ReportAppendix、*PrintToolbar
 *
 * 规范（04 文档 §2.3、§4.1、§4.7）：
 * - 章节顺序对齐 HTML 审计报告八章节结构
 * - 样例只渲染掩码值；打印隐藏交互控件
 * - 图表色值一律来自 chartTheme，不写裸 hex
 */

import * as React from 'react';
import Link from 'next/link';
import {
  ArrowLeft,
  ChartColumn,
  ChartLine,
  ChartPie,
  ClipboardList,
  Download,
  EyeOff,
  PanelTop,
  TriangleExclamationPoint,
} from 'lucide-react';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, NumericCell } from '@/components/ui/table';
import { EChart, buildBarOption, buildLineOption, buildPieOption } from '@/components/charts/EChart';
import { riskColorSeries } from '@/lib/chart-theme';
import { formatDateTime, formatPercent, toUiRisk, UI_RISK_LABEL } from '@/lib/format';
import { REPORT_SECTIONS } from '@/stores/useReportPreviewStore';
import type { ColumnFinding, DashboardSummary, ScanTask } from '@/types/api';

/* ------------------------------------------------------------ *ReportAnchorNav */

/** 章节锚点侧栏 */
export function ReportAnchorNav({
  activeSection,
  onSelect,
}: {
  activeSection: string;
  onSelect: (id: string) => void;
}) {
  return (
    <nav aria-label="报告章节" className="no-print sticky top-[var(--space-3)]">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <PanelTop size={20} aria-hidden="true" />
            章节
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-[var(--space-1)]">
          {REPORT_SECTIONS.map((section) => (
            <button
              key={section.id}
              type="button"
              onClick={() => onSelect(section.id)}
              aria-current={activeSection === section.id ? 'true' : undefined}
              className={
                'min-h-[var(--touch-target-min)] rounded-[var(--radius-sm)] px-[var(--space-2)] py-[var(--space-1)] text-left text-[var(--text-sm)] transition-colors duration-[var(--motion-fast)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)] ' +
                (activeSection === section.id
                  ? 'bg-[var(--surface-selected)] font-[var(--weight-emphasize)] text-[var(--accent)]'
                  : 'text-[var(--fg-2)] hover:bg-[var(--surface-hover)] hover:text-[var(--fg)]')
              }
            >
              {section.label}
            </button>
          ))}
        </CardContent>
      </Card>
    </nav>
  );
}

/* ----------------------------------------------------------- *ReportSummaryKpis */

/** 报告概况 KPI（≤4 个） */
export function ReportSummaryKpis({
  summary,
  task,
}: {
  summary: DashboardSummary | null;
  task: ScanTask | null;
}) {
  const hitColumns = summary?.hitColumnCount ?? 0;
  const totalColumns = summary?.columnCount ?? 0;
  const rate = totalColumns > 0 ? (hitColumns / totalColumns) * 100 : 0;

  const items = [
    { label: '扫描表数', value: summary?.tableCount ?? 0, hint: '已扫描的表对象数量' },
    { label: '扫描列数', value: totalColumns, hint: '已扫描列，不含 BLOB 跳过列' },
    { label: '命中列数', value: hitColumns, hint: '识别为敏感字段的列' },
    { label: '识别率', value: `${rate.toFixed(1)}%`, hint: '命中列数 / 已扫描列数' },
  ];

  return (
    <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] sm:grid-cols-2 lg:grid-cols-4">
      {items.map((item) => (
        <Card key={item.label}>
          <CardContent className="flex flex-col gap-[var(--space-1)] p-[var(--card-padding)]">
            <span className="text-[var(--text-xs)] text-[var(--muted)]">{item.label}</span>
            <span className="font-[var(--font-mono)] text-[var(--text-2xl)] font-[var(--weight-announce)] tabular-nums leading-[var(--leading-tight)] text-[var(--fg)]">
              {item.value}
            </span>
            <span className="text-[var(--text-xs)] text-[var(--meta)]">{item.hint}</span>
          </CardContent>
        </Card>
      ))}
      {task ? (
        <p className="text-[var(--text-xs)] text-[var(--muted)] sm:col-span-2 lg:col-span-4">
          任务 <span className="font-[var(--font-mono)]">{task.id}</span> · 数据源{' '}
          <span className="font-[var(--font-mono)]">
            {task.dataSourceName ?? task.dataSourceId}
          </span>{' '}
          · 完成时间 {formatDateTime(task.finishedAt)} · 规则版本{' '}
          <span className="font-[var(--font-mono)]">{task.ruleVersion ?? '—'}</span>
        </p>
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------ *ReportChartSet */

/** 报告图表组：风险分布（C1）、敏感类型 TOP（C3）、模式分布（C4） */
export function ReportChartSet({ summary }: { summary: DashboardSummary | null }) {
  const riskOption = React.useCallback(
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

  const typeOption = React.useCallback(
    (theme: Parameters<typeof buildBarOption>[0]) => {
      const entries = Object.entries(summary?.typeCounts ?? {})
        .sort((a, b) => b[1] - a[1])
        .slice(0, 10);
      return buildBarOption(
        theme,
        entries.map(([name]) => name),
        [{ name: '命中字段数', data: entries.map(([, value]) => value) }],
        'horizontal',
      );
    },
    [summary],
  );

  const schemaOption = React.useCallback(
    (theme: Parameters<typeof buildBarOption>[0]) => {
      const entries = Object.entries(summary?.schemaCounts ?? {}).sort(
        (a, b) => b[1] - a[1],
      );
      return buildBarOption(
        theme,
        entries.map(([name]) => name),
        [{ name: '字段数', data: entries.map(([, value]) => value) }],
      );
    },
    [summary],
  );

  return (
    <div className="flex flex-col gap-[var(--gutter-desktop)]">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <ChartPie size={20} aria-hidden="true" />
            风险等级分布
          </CardTitle>
          <CardDescription>按字段数统计，固定顺序 高 → 中 → 低 → 待复核</CardDescription>
        </CardHeader>
        <CardContent>
          <EChart option={riskOption} ariaLabel="风险等级分布环形图" height={260} />
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-[var(--space-2)]">
              <ChartColumn size={20} aria-hidden="true" />
              敏感类型 TOP 10
            </CardTitle>
            <CardDescription>按类型展开，同一字段命中多类型时分别计数（非去重字段数）</CardDescription>
          </CardHeader>
          <CardContent>
            <EChart option={typeOption} ariaLabel="敏感类型前十柱状图" height={280} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-[var(--space-2)]">
              <ChartLine size={20} aria-hidden="true" />
              模式分布
            </CardTitle>
            <CardDescription>按模式聚合的字段数，坐标轴从 0 起</CardDescription>
          </CardHeader>
          <CardContent>
            <EChart option={schemaOption} ariaLabel="模式字段数柱状图" height={280} />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

/* --------------------------------------------------------------- *RiskListTable */

/** 高风险与待复核字段清单 */
export function RiskListTable({
  findings,
  levels,
}: {
  findings: ColumnFinding[];
  levels: ('high' | 'pending')[];
}) {
  const rows = findings.filter((item) => {
    const ui = toUiRisk(item.riskLevel);
    return levels.includes(ui as 'high' | 'pending');
  });

  const title = levels.includes('high') ? '高风险字段清单' : '待复核字段清单';

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <TriangleExclamationPoint size={20} aria-hidden="true" />
          {title}
        </CardTitle>
        <CardDescription className="flex items-center gap-[var(--space-1)]">
          <EyeOff size={12} aria-hidden="true" />
          样例只渲染掩码值
        </CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>实例</TableHead>
              <TableHead>模式.表.字段</TableHead>
              <TableHead>敏感类型</TableHead>
              <TableHead className="w-[100px] text-right">命中率</TableHead>
              <TableHead>风险等级</TableHead>
              <TableHead>掩码样例</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableEmptyRow colSpan={6} message="当前报告范围内没有该等级的字段" />
            ) : (
              rows.slice(0, 50).map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">
                    {item.instanceId}
                  </TableCell>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">
                    {item.schemaName}.{item.tableName}.{item.columnName}
                  </TableCell>
                  <TableCell>
                    <span className="flex flex-wrap gap-[var(--space-1)]">
                      {item.sensitiveTypes.slice(0, 2).map((type) => (
                        <Badge key={type} tone="neutral">
                          {type}
                        </Badge>
                      ))}
                    </span>
                  </TableCell>
                  <NumericCell className="text-[var(--text-xs)]">
                    {formatPercent(item.hitRatio)}
                  </NumericCell>
                  <TableCell>
                    <Badge
                      tone={
                        toUiRisk(item.riskLevel) === 'high'
                          ? 'danger'
                          : toUiRisk(item.riskLevel) === 'medium'
                            ? 'warning'
                            : toUiRisk(item.riskLevel) === 'low'
                              ? 'success'
                              : 'neutral'
                      }
                    >
                      {UI_RISK_LABEL[toUiRisk(item.riskLevel)]}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <span className="rounded-[var(--radius-sm)] border border-[var(--sample-border)] bg-[var(--sample-bg)] px-[var(--space-1)] font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--sample-fg)]">
                      {item.sampleMasked}
                    </span>
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

/* -------------------------------------------------------------- *ReportAppendix */

/** 报告附录：口径说明 */
export function ReportAppendix() {
  const items = [
    { label: '字段计数口径', value: '按 risk_level 统计字段数，同一字段可同时计入多个敏感类型' },
    { label: '识别率口径', value: '命中列数 / 已扫描列数，不含 BLOB 跳过列' },
    { label: '样例脱敏口径', value: '样例一律为掩码值；127.0.0.1 与 localhost 保留完整' },
    { label: '报告生成', value: '服务端流式生成，写入 report_artifact 并记审计日志' },
    { label: '下载留痕', value: '每次下载记录操作者、动作与来源 IP（已脱敏）' },
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ClipboardList size={20} aria-hidden="true" />
          附录：口径说明
        </CardTitle>
        <CardDescription>指标口径与脱敏规则，便于报告接收方核对</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-2)]">
        {items.map((item) => (
          <div
            key={item.label}
            className="flex flex-col gap-[2px] border-b border-[var(--border-soft)] pb-[var(--space-2)] last:border-b-0"
          >
            <span className="text-[var(--text-xs)] text-[var(--muted)]">{item.label}</span>
            <span className="text-[var(--text-sm)] text-[var(--fg-2)]">{item.value}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

/* --------------------------------------------------------------- *PrintToolbar */

/** 打印 / 下载工具栏 */
export function PrintToolbar({
  onPrint,
  onDownload,
  downloading,
}: {
  onPrint: () => void;
  onDownload: () => void;
  downloading?: boolean;
}) {
  return (
    <div className="no-print flex flex-wrap items-center justify-between gap-[var(--space-3)]">
      <Button variant="ghost" size="sm" asChild>
        <Link href="/reports">
          <ArrowLeft size={16} aria-hidden="true" />
          返回报告中心
        </Link>
      </Button>
      <div className="flex items-center gap-[var(--space-2)]">
        <Button variant="secondary" size="sm" onClick={onPrint}>
          <Download size={16} aria-hidden="true" />
          打印报告
        </Button>
        <Button size="sm" loading={downloading} onClick={onDownload}>
          <Download size={16} aria-hidden="true" />
          下载报告
        </Button>
      </div>
    </div>
  );
}
