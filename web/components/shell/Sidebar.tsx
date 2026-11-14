'use client';

/**
 * dbdlp 控制台 · 应用壳层 · 侧栏（04 文档 §2.2）
 *
 * 按任务链分 5 组（认知负荷 ≤5）：概览 / 资产 / 识别 / 结果 / 交付。
 * 激活态用 --surface-selected 背景 + --accent 图标与文字 + 强调字重，
 * **不用侧边条纹或彩色左边框**。单项高度 ≥ --touch-target-min。
 */

import * as React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  ChevronDown,
  Database,
  FileText,
  LayoutDashboard,
  PanelLeftClose,
  PanelLeftOpen,
  ScanSearch,
  Settings,
  ShieldCheck,
  Table2,
} from 'lucide-react';
import { useUIStore } from '@/stores/useUIStore';
import { cn } from '@/lib/utils';

interface NavItem {
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

/** 5 组导航（与 §2.2 完全一致） */
export const NAV_GROUPS: NavGroup[] = [
  {
    label: '概览',
    items: [{ href: '/', label: '总览工作台', icon: LayoutDashboard }],
  },
  {
    label: '资产',
    items: [
      { href: '/datasources', label: '数据源管理', icon: Database },
      { href: '/datasources/import', label: '批量导入', icon: ScanSearch },
    ],
  },
  {
    label: '识别',
    items: [
      { href: '/tasks', label: '扫描任务', icon: ScanSearch },
      { href: '/rules', label: '规则管理', icon: ShieldCheck },
    ],
  },
  {
    label: '结果',
    items: [{ href: '/findings', label: '结果台账', icon: Table2 }],
  },
  {
    label: '交付',
    items: [
      { href: '/reports', label: '报告中心', icon: FileText },
      { href: '/config', label: '系统设置', icon: Settings },
    ],
  },
];

/** 判断某路径是否命中当前导航项 */
function isActive(pathname: string, href: string): boolean {
  if (href === '/') return pathname === '/';
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function Sidebar() {
  const pathname = usePathname() ?? '/';
  const collapsed = useUIStore((s) => s.sidebarCollapsed);
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);
  const [openGroups, setOpenGroups] = React.useState<Record<string, boolean>>({});

  return (
    <aside
      className={cn(
        'no-print flex shrink-0 flex-col border-r border-[var(--border-on-brand)] bg-[var(--surface-brand)] transition-[width] duration-[var(--motion-base)] ease-[var(--ease-standard)]',
        collapsed ? 'w-[var(--sidebar-w-collapsed)]' : 'w-[var(--sidebar-w)]',
      )}
      aria-label="主导航"
    >
      {/* 品牌区 */}
      <div className="flex h-[var(--topbar-h)] items-center gap-[var(--space-2)] border-b border-[var(--border-on-brand)] px-[var(--space-4)]">
        <ShieldCheck size={20} className="shrink-0 text-[var(--fg-on-brand)]" aria-hidden="true" />
        {collapsed ? null : (
          <span className="truncate text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg-on-brand)]">
            达梦敏感数据扫描
          </span>
        )}
      </div>

      {/* 导航组 */}
      <nav className="flex-1 overflow-y-auto px-[var(--space-2)] py-[var(--space-3)]">
        {NAV_GROUPS.map((group) => {
          const open = openGroups[group.label] ?? true;
          const groupActive = group.items.some((item) => isActive(pathname, item.href));
          return (
            <div key={group.label} className="mb-[var(--space-3)]">
              {collapsed ? (
                <div className="mb-[var(--space-1)] px-[var(--space-2)] text-[var(--text-xs)] text-[var(--border-on-brand)]">
                  {group.label}
                </div>
              ) : (
                <button
                  type="button"
                  onClick={() => setOpenGroups((prev) => ({ ...prev, [group.label]: !open }))}
                  aria-expanded={open}
                  className="flex w-full items-center justify-between rounded-[var(--radius-sm)] px-[var(--space-2)] py-[var(--space-1)] text-[var(--text-xs)] font-[var(--weight-emphasize)] text-[var(--border-on-brand)] transition-colors duration-[var(--motion-fast)] hover:text-[var(--fg-on-brand)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
                >
                  <span>{group.label}</span>
                  <ChevronDown
                    size={14}
                    aria-hidden="true"
                    className={cn('transition-transform duration-[var(--motion-fast)]', open ? '' : '-rotate-90')}
                  />
                </button>
              )}

              {open || collapsed ? (
                <ul className="mt-[var(--space-1)] flex flex-col gap-[var(--space-1)]">
                  {group.items.map((item) => {
                    const active = isActive(pathname, item.href);
                    const Icon = item.icon;
                    return (
                      <li key={item.href}>
                        <Link
                          href={item.href}
                          aria-current={active ? 'page' : undefined}
                          title={collapsed ? item.label : undefined}
                          className={cn(
                            'flex min-h-[var(--touch-target-min)] items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-3)] py-[var(--space-2)] text-[var(--text-sm)] transition-colors duration-[var(--motion-fast)] ease-[var(--ease-standard)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]',
                            active
                              ? 'bg-[var(--surface-selected)] font-[var(--weight-emphasize)] text-[var(--accent)]'
                              : 'text-[var(--fg-on-brand)] hover:bg-[var(--surface-hover)] hover:text-[var(--fg-on-brand)]',
                          )}
                        >
                          <Icon size={20} aria-hidden="true" className="shrink-0" />
                          {collapsed ? (
                            <span className="sr-only">{item.label}</span>
                          ) : (
                            <span className="truncate">{item.label}</span>
                          )}
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              ) : null}
            </div>
          );
        })}
      </nav>

      {/* 折叠开关 */}
      <div className="border-t border-[var(--border-on-brand)] p-[var(--space-2)]">
        <button
          type="button"
          onClick={toggleSidebar}
          aria-label={collapsed ? '展开侧栏' : '收起侧栏'}
          className="flex min-h-[var(--touch-target-min)] w-full items-center justify-center gap-[var(--space-2)] rounded-[var(--radius-md)] text-[var(--text-sm)] text-[var(--fg-on-brand)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
        >
          {collapsed ? (
            <PanelLeftOpen size={20} aria-hidden="true" />
          ) : (
            <>
              <PanelLeftClose size={20} aria-hidden="true" />
              <span>收起侧栏</span>
            </>
          )}
        </button>
      </div>
    </aside>
  );
}
