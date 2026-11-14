'use client';

/**
 * dbdlp 控制台 · 应用壳层 · 全局搜索面板（04 文档 §2.2）
 *
 * Cmd/Ctrl + K 打开；跨数据源 / 任务 / 字段三级检索并直达深链。
 * 检索结果只含脱敏字段（样例列不进入检索索引）。
 */

import * as React from 'react';
import { useRouter } from 'next/navigation';
import { Database, Inbox, ScanSearch, Search, Table2, X } from 'lucide-react';
import { useUIStore } from '@/stores/useUIStore';
import { useDataSourceStore } from '@/stores/useDataSourceStore';
import { useTaskStore } from '@/stores/useTaskStore';
import { useFindingStore } from '@/stores/useFindingStore';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/overlays';
import { Input } from '@/components/ui/input';
import { formatRelative } from '@/lib/format';

interface Hit {
  kind: 'datasource' | 'task' | 'finding';
  label: string;
  detail: string;
  href: string;
  Icon: typeof Database;
}

export function CommandPalette() {
  const router = useRouter();
  const open = useUIStore((s) => s.commandOpen);
  const setOpen = useUIStore((s) => s.setCommandOpen);
  const datasources = useDataSourceStore((s) => s.items);
  const tasks = useTaskStore((s) => s.items);
  const findings = useFindingStore((s) => s.items);
  const [keyword, setKeyword] = React.useState('');
  const inputRef = React.useRef<HTMLInputElement | null>(null);

  // 打开时预取三类数据，保证检索范围是「当前视图快照」
  React.useEffect(() => {
    if (!open) return;
    void useDataSourceStore.getState().fetchList({ page: 1, limit: 50 });
    void useTaskStore.getState().fetchList({ page: 1, limit: 50 });
    void useFindingStore.getState().fetchList({ page: 1, limit: 50 });
    setKeyword('');
    window.setTimeout(() => inputRef.current?.focus(), 50);
  }, [open]);

  const hits = React.useMemo<Hit[]>(() => {
    const kw = keyword.trim().toLowerCase();
    const all: Hit[] = [
      ...datasources.map((item) => ({
        kind: 'datasource' as const,
        label: item.name,
        detail: `${item.dialect} · ${item.host}:${item.port}`,
        href: `/datasources/${item.id}/edit`,
        Icon: Database,
      })),
      ...tasks.map((item) => ({
        kind: 'task' as const,
        label: item.id,
        detail: `${item.dataSourceName ?? item.dataSourceId} · ${formatRelative(item.createdAt)}`,
        href: `/tasks/${item.id}`,
        Icon: ScanSearch,
      })),
      ...findings.map((item) => ({
        kind: 'finding' as const,
        label: `${item.schemaName}.${item.tableName}.${item.columnName}`,
        detail: `${item.instanceId} · ${item.sensitiveTypes.join('、') || '未分类'}`,
        href: `/findings/${item.id}`,
        Icon: Table2,
      })),
    ];
    if (!kw) return all.slice(0, 12);
    return all
      .filter(
        (hit) =>
          hit.label.toLowerCase().includes(kw) || hit.detail.toLowerCase().includes(kw),
      )
      .slice(0, 12);
  }, [datasources, tasks, findings, keyword]);

  const handleSelect = (href: string) => {
    setOpen(false);
    router.push(href);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="top-[18%] translate-y-0 p-[var(--space-4)]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-[var(--space-2)]">
            <Search size={20} aria-hidden="true" />
            全局搜索
          </DialogTitle>
          <DialogDescription>
            在数据源、扫描任务与字段台账中检索；结果只含脱敏信息
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-[var(--space-3)]">
          <Input
            ref={inputRef}
            value={keyword}
            placeholder="输入实例标识、任务编号或模式.表.字段"
            onChange={(e) => setKeyword(e.target.value)}
            aria-label="搜索关键词"
          />

          <ul className="flex max-h-[320px] flex-col gap-[var(--space-1)] overflow-y-auto">
            {hits.length === 0 ? (
              <li className="flex flex-col items-center gap-[var(--space-2)] py-[var(--space-6)]">
                <Inbox size={24} aria-hidden="true" className="text-[var(--meta)]" />
                <p className="text-[var(--text-sm)] text-[var(--muted)]">没有匹配的结果</p>
              </li>
            ) : (
              hits.map((hit) => (
                <li key={`${hit.kind}-${hit.href}`}>
                  <button
                    type="button"
                    onClick={() => handleSelect(hit.href)}
                    className="flex w-full items-center gap-[var(--space-3)] rounded-[var(--radius-sm)] px-[var(--space-2)] py-[var(--space-2)] text-left transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
                  >
                    <hit.Icon size={16} aria-hidden="true" className="shrink-0 text-[var(--muted)]" />
                    <span className="flex min-w-0 flex-col gap-[2px]">
                      <span className="truncate font-[var(--font-mono)] text-[var(--text-sm)] text-[var(--fg)]">
                        {hit.label}
                      </span>
                      <span className="truncate text-[var(--text-xs)] text-[var(--muted)]">{hit.detail}</span>
                    </span>
                    <span className="ml-auto shrink-0 text-[var(--text-xs)] text-[var(--meta)]">
                      {hit.kind === 'datasource' ? '数据源' : hit.kind === 'task' ? '任务' : '字段'}
                    </span>
                  </button>
                </li>
              ))
            )}
          </ul>

          <div className="flex items-center justify-end">
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="inline-flex items-center gap-[var(--space-1)] rounded-[var(--radius-sm)] px-[var(--space-2)] py-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)] transition-colors duration-[var(--motion-fast)] hover:text-[var(--fg)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
            >
              <X size={12} aria-hidden="true" />
              关闭
            </button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
