'use client';

/**
 * dbdlp 控制台 · P11 规则管理业务组件
 *
 * 包含：*RuleTable、*RuleDetailPanel、*RuleVersionBadge、*RuleSnapshotList
 *
 * 边界（04 文档 §2.3、§10 禁止项 10）：**规则只读查看**，不提供在线编辑列。
 * 规则加载失败（2002）由页面以中文 Alert 呈现。
 */

import * as React from 'react';
import {
  BadgeCheck,
  Braces,
  CircleCheck,
  CircleSlash2,
  Gauge,
  GitBranch,
  Info,
  Lock,
  ShieldCheck,
  ShieldExclamationPoint,
} from 'lucide-react';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Skeleton } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, NumericCell } from '@/components/ui/table';
import { formatDateTime } from '@/lib/format';
import type { RuleItem, RuleSnapshot } from '@/types/api';

/* ----------------------------------------------------------- *RuleVersionBadge */

/** 规则版本徽章 */
export function RuleVersionBadge({ version }: { version: string }) {
  return (
    <Badge tone="accent">
      <GitBranch size={12} aria-hidden="true" />
      {version || '—'}
    </Badge>
  );
}

/* ------------------------------------------------------------------ *RuleTable */

/** 规则清单表（只读） */
export function RuleTable({
  rules,
  loading,
  selectedId,
  onSelect,
}: {
  rules: RuleItem[];
  loading?: boolean;
  selectedId: string | null;
  onSelect: (id: string) => void;
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
        <table className="w-full border-collapse text-[var(--text-sm)]">
          <thead className="sticky top-0 z-[var(--z-sticky)] bg-[var(--table-header-bg)]">
            <tr>
              <th className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]">
                规则名称
              </th>
              <th className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]">
                敏感类型
              </th>
              <th className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]">
                匹配模式
              </th>
              <th className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-right align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]">
                阈值
              </th>
              <th className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]">
                状态
              </th>
              <th className="h-[var(--table-row-h)] px-[var(--table-cell-px)] py-[var(--table-cell-py)] text-left align-middle text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--table-header-fg)]">
                校验
              </th>
            </tr>
          </thead>
          <tbody>
            {rules.length === 0 ? (
              <tr>
                <td colSpan={6} className="h-[160px] text-center">
                  <div className="flex flex-col items-center gap-[var(--space-2)]">
                    <ShieldCheck size={24} aria-hidden="true" className="text-[var(--meta)]" />
                    <p className="text-[var(--text-sm)] text-[var(--muted)]">该版本没有可用规则</p>
                  </div>
                </td>
              </tr>
            ) : (
              rules.map((rule) => (
                <tr
                  key={rule.id}
                  onClick={() => onSelect(rule.id)}
                  aria-selected={selectedId === rule.id}
                  className={
                    'cursor-pointer bg-[var(--table-row-bg)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--table-row-bg-hover)] ' +
                    (selectedId === rule.id ? 'bg-[var(--table-row-bg-selected)]' : '')
                  }
                >
                  <td className="px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle text-[var(--fg)]">
                    {rule.name}
                  </td>
                  <td className="px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle text-[var(--fg-2)]">
                    {rule.sensitiveType}
                  </td>
                  <td className="px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle">
                    <span className="block max-w-[280px] truncate font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--fg-2)]">
                      {rule.pattern}
                    </span>
                  </td>
                  <NumericCell className="text-[var(--text-xs)]">
                    {rule.threshold ?? '—'}
                  </NumericCell>
                  <td className="px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle">
                    {rule.enabled ? (
                      <Badge tone="success">
                        <CircleCheck size={12} aria-hidden="true" />
                        启用
                      </Badge>
                    ) : (
                      <Badge tone="neutral">
                        <CircleSlash2 size={12} aria-hidden="true" />
                        禁用
                      </Badge>
                    )}
                  </td>
                  <td className="px-[var(--table-cell-px)] py-[var(--table-cell-py)] align-middle">
                    {rule.validateStatus === 'valid' ? (
                      <Badge tone="info">
                        <BadgeCheck size={12} aria-hidden="true" />
                        通过
                      </Badge>
                    ) : rule.validateStatus === 'invalid' ? (
                      <Badge tone="danger">
                        <ShieldExclamationPoint size={12} aria-hidden="true" />
                        未通过
                      </Badge>
                    ) : (
                      <Badge tone="neutral">未校验</Badge>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------- *RuleDetailPanel */

/** 规则详情面板（只读） */
export function RuleDetailPanel({ rule }: { rule: RuleItem | null }) {
  if (!rule) {
    return (
      <Card className="sticky top-[var(--space-3)]">
        <CardHeader>
          <CardTitle className="flex items-center gap-[var(--space-2)]">
            <Info size={20} aria-hidden="true" />
            规则详情
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">在左侧清单中选择一条规则查看详情。</p>
        </CardContent>
      </Card>
    );
  }

  const rows = [
    { label: '规则编号', value: rule.id, mono: true },
    { label: '规则名称', value: rule.name },
    { label: '敏感类型', value: rule.sensitiveType },
    { label: '匹配模式', value: rule.pattern, mono: true },
    { label: '命中阈值', value: rule.threshold !== undefined ? String(rule.threshold) : '—' },
    {
      label: '启用状态',
      value: rule.enabled ? '启用' : '禁用',
    },
    {
      label: '校验状态',
      value:
        rule.validateStatus === 'valid'
          ? '通过'
          : rule.validateStatus === 'invalid'
            ? '未通过'
            : '未校验',
    },
    { label: '规则说明', value: rule.description || '—' },
  ];

  return (
    <Card className="sticky top-[var(--space-3)]">
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ShieldCheck size={20} aria-hidden="true" />
          规则详情
        </CardTitle>
        <CardDescription className="flex items-center gap-[var(--space-1)]">
          <Lock size={12} aria-hidden="true" />
          只读查看，规则变更需在服务端配置文件调整并重新发布版本
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-2)]">
        {rows.map((row) => (
          <div
            key={row.label}
            className="flex flex-col gap-[2px] border-b border-[var(--border-soft)] pb-[var(--space-2)] last:border-b-0"
          >
            <span className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
              {row.label === '匹配模式' ? (
                <Braces size={12} aria-hidden="true" />
              ) : row.label === '命中阈值' ? (
                <Gauge size={12} aria-hidden="true" />
              ) : null}
              {row.label}
            </span>
            <span
              className={
                'break-all text-[var(--text-sm)] text-[var(--fg-2)]' +
                (row.mono ? ' font-[var(--font-mono)] text-[var(--text-xs)]' : '')
              }
            >
              {row.value}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

/* ----------------------------------------------------------- *RuleSnapshotList */

/** 规则版本快照列表 */
export function RuleSnapshotList({
  snapshots,
  currentVersion,
  onSelect,
}: {
  snapshots: RuleSnapshot[];
  currentVersion: string;
  onSelect: (version: string) => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <GitBranch size={20} aria-hidden="true" />
          版本快照
        </CardTitle>
        <CardDescription>选择历史版本查看当时的规则清单</CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>版本</TableHead>
              <TableHead className="w-[100px] text-right">规则数</TableHead>
              <TableHead className="w-[160px]">生成时间</TableHead>
              <TableHead className="w-[100px] text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {snapshots.length === 0 ? (
              <TableEmptyRow colSpan={4} message="暂无版本快照" />
            ) : (
              snapshots.map((snapshot) => (
                <TableRow key={snapshot.version}>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">
                    {snapshot.version}
                    {snapshot.version === currentVersion ? (
                      <span className="ml-[var(--space-1)]">
                        <Badge tone="accent">当前</Badge>
                      </span>
                    ) : null}
                  </TableCell>
                  <NumericCell className="text-[var(--text-xs)]">{snapshot.ruleCount}</NumericCell>
                  <TableCell className="text-[var(--muted)]">
                    {formatDateTime(snapshot.createdAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onSelect(snapshot.version)}
                      disabled={snapshot.version === currentVersion}
                    >
                      查看
                    </Button>
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
