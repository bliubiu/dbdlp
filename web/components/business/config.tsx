'use client';

/**
 * dbdlp 控制台 · P12 系统设置业务组件
 *
 * 包含：*ConfigReadOnlyPanel、*AuditLogTable、*AppearancePanel
 *
 * 边界（04 文档 §9、§2.6）：
 * - 生效配置只读回显，不含密钥与 DSN
 * - **不提供深色主题切换**；外观仅含行密度与侧栏折叠
 * - 审计日志内容已脱敏
 */

import * as React from 'react';
import { ClipboardList, Layers, LockKeyhole, Rows3, SlidersHorizontal } from 'lucide-react';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Skeleton } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { formatDateTime } from '@/lib/format';
import { cn } from '@/lib/utils';
import { useUIStore, type Density } from '@/stores/useUIStore';
import type { AppConfig, AuditLog } from '@/types/api';

/* --------------------------------------------------------- *ConfigReadOnlyPanel */

/** 生效配置只读回显（每组 ≤4 字段） */
export function ConfigReadOnlyPanel({
  config,
  loading,
}: {
  config: AppConfig | null;
  loading?: boolean;
}) {
  if (loading) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-[var(--space-2)] p-[var(--card-padding)]">
          <Skeleton className="h-[28px] w-full" />
          <Skeleton className="h-[28px] w-full" />
          <Skeleton className="h-[28px] w-full" />
        </CardContent>
      </Card>
    );
  }

  const rows = [
    { label: '日志级别', value: config?.logLevel ?? '—', Icon: SlidersHorizontal },
    { label: '采样行数', value: config?.sampleRows !== undefined ? String(config.sampleRows) : '—', Icon: Layers },
    { label: '存储驱动', value: config?.storageDriver ?? '—', Icon: Layers },
    { label: '加密算法', value: config?.cryptoAlgorithm ?? '—', Icon: LockKeyhole },
    { label: '留存天数', value: config?.retentionDays !== undefined ? String(config.retentionDays) : '—', Icon: Layers },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <SlidersHorizontal size={20} aria-hidden="true" />
          生效配置
        </CardTitle>
        <CardDescription className="flex items-center gap-[var(--space-1)]">
          <LockKeyhole size={12} aria-hidden="true" />
          只读回显，不含密钥与数据库连接串
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-2)]">
        {rows.map((row) => (
          <div
            key={row.label}
            className="flex items-center justify-between gap-[var(--space-3)] border-b border-[var(--border-soft)] py-[var(--space-1)] last:border-b-0"
          >
            <span className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
              <row.Icon size={12} aria-hidden="true" />
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

/* ---------------------------------------------------------------- *AuditLogTable */

/** 审计日志表（内容已脱敏） */
export function AuditLogTable({
  logs,
  total,
}: {
  logs: AuditLog[];
  total: number;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ClipboardList size={20} aria-hidden="true" />
          审计日志
        </CardTitle>
        <CardDescription>共 {total} 条；IP 前两段以星号替换，日志内容已脱敏</CardDescription>
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
              <TableEmptyRow colSpan={5} message="暂无审计记录" />
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="text-[var(--muted)]">{formatDateTime(log.createdAt)}</TableCell>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">{log.actor}</TableCell>
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

/* --------------------------------------------------------------- *AppearancePanel */

/** 外观偏好：行密度 + 侧栏折叠（不提供深色主题切换） */
export function AppearancePanel() {
  const density = useUIStore((s) => s.density);
  const setDensity = useUIStore((s) => s.setDensity);
  const sidebarCollapsed = useUIStore((s) => s.sidebarCollapsed);
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);

  const densities: { value: Density; label: string }[] = [
    { value: 'compact', label: '紧凑' },
    { value: 'default', label: '标准' },
    { value: 'comfortable', label: '宽松' },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Rows3 size={20} aria-hidden="true" />
          外观偏好
        </CardTitle>
        <CardDescription>偏好仅保存在当前浏览器会话，不影响服务端配置</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-4)]">
        <div className="flex flex-col gap-[var(--space-2)]">
          <span className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
            表格行密度
          </span>
          <div
            role="group"
            aria-label="表格行密度"
            className="inline-flex w-fit items-center gap-[var(--space-1)] rounded-[var(--radius-md)] border border-[var(--border-soft)] bg-[var(--surface-warm)] p-[var(--space-1)]"
          >
            {densities.map((item) => (
              <button
                key={item.value}
                type="button"
                onClick={() => setDensity(item.value)}
                aria-pressed={density === item.value}
                className={cn(
                  'min-h-[32px] rounded-[var(--radius-sm)] px-[var(--space-3)] text-[var(--text-xs)] font-[var(--weight-emphasize)] transition-colors duration-[var(--motion-fast)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]',
                  density === item.value
                    ? 'bg-[var(--surface)] text-[var(--fg)]'
                    : 'text-[var(--muted)] hover:text-[var(--fg)]',
                )}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>

        <div className="flex items-center justify-between gap-[var(--space-3)] border-t border-[var(--border-soft)] pt-[var(--space-3)]">
          <span className="flex flex-col gap-[2px]">
            <span className="text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
              收起侧栏
            </span>
            <span className="text-[var(--text-xs)] text-[var(--muted)]">
              收起后侧栏只显示图标，悬停显示中文标签
            </span>
          </span>
          <Button variant="secondary" size="sm" onClick={toggleSidebar}>
            {sidebarCollapsed ? '展开侧栏' : '收起侧栏'}
          </Button>
        </div>

        <p className="flex items-start gap-[var(--space-1)] border-t border-[var(--border-soft)] pt-[var(--space-3)] text-[var(--text-xs)] text-[var(--muted)]">
          <LockKeyhole size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
          本期控制台仅提供浅色主题，不提供深色主题切换。
        </p>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------ 审计筛选（操作者 / 动作） */

/** 审计筛选栏 */
export function AuditFilterBar({
  actor,
  action,
  onActorChange,
  onActionChange,
}: {
  actor: string;
  action: string;
  onActorChange: (value: string) => void;
  onActionChange: (value: string) => void;
}) {
  return (
    <Card>
      <CardContent className="flex flex-wrap items-end gap-[var(--space-3)] p-[var(--space-4)]">
        <div className="flex min-w-[200px] flex-col gap-[var(--space-1)]">
          <label
            htmlFor="audit-actor"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            操作者
          </label>
          <Input
            id="audit-actor"
            value={actor}
            placeholder="输入账号筛选"
            onChange={(e) => onActorChange(e.target.value)}
          />
        </div>
        <div className="flex min-w-[200px] flex-col gap-[var(--space-1)]">
          <label
            htmlFor="audit-action"
            className="text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--fg)]"
          >
            动作
          </label>
          <Input
            id="audit-action"
            value={action}
            placeholder="输入动作关键字"
            onChange={(e) => onActionChange(e.target.value)}
          />
        </div>
      </CardContent>
    </Card>
  );
}

/** 会话说明 */
export function SessionHint() {
  return (
    <p className="text-[var(--text-xs)] text-[var(--muted)]">
      <Badge tone="neutral">单管理员会话</Badge>
      <span className="ml-[var(--space-2)]">
        会话失效后仅提供重新登录，不接入企业身份源。
      </span>
    </p>
  );
}
