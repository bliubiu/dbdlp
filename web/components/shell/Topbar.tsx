'use client';

/**
 * dbdlp 控制台 · 应用壳层 · 顶栏（04 文档 §2.2）
 *
 * 面包屑、全局搜索（Cmd/Ctrl + K）、当前规则版本徽章、通知、当前管理员账号与登出。
 */

import * as React from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { Bell, LogOut, Search, User } from 'lucide-react';
import { useAuthStore } from '@/stores/useAuthStore';
import { useUIStore } from '@/stores/useUIStore';
import { useRuleStore } from '@/stores/useRuleStore';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/primitives';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/overlays';

/** 路径 → 中文面包屑 */
function buildBreadcrumb(pathname: string): { label: string; href: string }[] {
  const segments = pathname.split('/').filter(Boolean);
  const crumbs: { label: string; href: string }[] = [{ label: '总览工作台', href: '/' }];

  const labelMap: Record<string, string> = {
    datasources: '数据源管理',
    new: '新建数据源',
    edit: '编辑数据源',
    import: '批量导入',
    tasks: '扫描任务',
    findings: '结果台账',
    reports: '报告中心',
    rules: '规则管理',
    config: '系统设置',
    login: '登录',
  };

  let acc = '';
  segments.forEach((segment, index) => {
    acc += `/${segment}`;
    // 动态 id 段不单独成条目
    if (/^[0-9a-f-]{8,}$/i.test(segment)) return;
    const isLast = index === segments.length - 1;
    crumbs.push({
      label: labelMap[segment] ?? segment,
      href: isLast ? acc : acc,
    });
  });
  return crumbs;
}

export function Topbar() {
  const pathname = usePathname() ?? '/';
  const router = useRouter();
  const displayName = useAuthStore((s) => s.displayName);
  const logout = useAuthStore((s) => s.logout);
  const setCommandOpen = useUIStore((s) => s.setCommandOpen);
  const ruleVersion = useRuleStore((s) => s.version);
  const fetchRules = useRuleStore((s) => s.fetchRules);
  const crumbs = React.useMemo(() => buildBreadcrumb(pathname), [pathname]);

  // 顶栏展示当前生效规则版本（取自 /api/v1/rules，不写死版本号）
  React.useEffect(() => {
    if (ruleVersion) return;
    void fetchRules();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ruleVersion]);

  // Cmd/Ctrl + K 打开全局搜索
  React.useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setCommandOpen(true);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [setCommandOpen]);

  const handleLogout = async () => {
    await logout();
    router.push('/login');
  };

  return (
    <header className="no-print flex h-[var(--topbar-h)] shrink-0 items-center gap-[var(--space-4)] border-b border-[var(--border-soft)] bg-[var(--surface)] px-[var(--gutter-desktop)]">
      {/* 面包屑 */}
      <nav aria-label="面包屑" className="flex min-w-0 items-center gap-[var(--space-1)]">
        {crumbs.map((crumb, index) => (
          <React.Fragment key={`${crumb.href}-${index}`}>
            {index > 0 ? (
              <span aria-hidden="true" className="text-[var(--meta)]">
                /
              </span>
            ) : null}
            <span
              className={
                index === crumbs.length - 1
                  ? 'truncate text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]'
                  : 'truncate text-[var(--text-sm)] text-[var(--muted)]'
              }
            >
              {crumb.label}
            </span>
          </React.Fragment>
        ))}
      </nav>

      <div className="flex-1" />

      {/* 全局搜索 */}
      <button
        type="button"
        onClick={() => setCommandOpen(true)}
        aria-label="打开全局搜索"
        className="inline-flex h-[var(--btn-height)] min-w-[200px] items-center gap-[var(--space-2)] rounded-[var(--radius-md)] border border-[var(--border-soft)] bg-[var(--surface-warm)] px-[var(--space-3)] text-[var(--text-sm)] text-[var(--meta)] transition-colors duration-[var(--motion-fast)] hover:border-[var(--border-strong)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
      >
        <Search size={16} aria-hidden="true" />
        <span>搜索数据源、任务与字段</span>
        <kbd className="ml-auto rounded-[var(--radius-sm)] border border-[var(--border-soft)] bg-[var(--surface)] px-[var(--space-1)] font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--meta)]">
          Ctrl K
        </kbd>
      </button>

      {/* 规则版本徽章（取自当前生效规则版本） */}
      <Badge tone="info" className="shrink-0">
        规则版本 {ruleVersion || '加载中'}
      </Badge>

      {/* 通知 */}
      <button
        type="button"
        aria-label="通知"
        className="inline-flex h-[var(--btn-height)] w-[var(--btn-height)] items-center justify-center rounded-[var(--radius-md)] text-[var(--muted)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] hover:text-[var(--fg)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
      >
        <Bell size={20} aria-hidden="true" />
      </button>

      {/* 账号菜单 */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" aria-label="管理员账号菜单">
            <User size={16} aria-hidden="true" />
            <span>{displayName ?? '管理员'}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled>
            <span className="text-[var(--text-xs)] text-[var(--muted)]">单管理员会话，不提供多角色权限</span>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => void handleLogout()}>
            <LogOut size={16} aria-hidden="true" />
            退出登录
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  );
}
