'use client';

/**
 * dbdlp 控制台 · 受保护页面壳层（04 文档 §2.2）
 *
 * Sidebar（5 组导航）+ Topbar + 内容区（最大宽 --container-max，12 列栅格）。
 */

import { AuthGuard } from '@/components/shell/AuthGuard';
import { Sidebar } from '@/components/shell/Sidebar';
import { Topbar } from '@/components/shell/Topbar';

export default function ConsoleLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthGuard>
      <div className="flex min-h-screen">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <Topbar />
          <main className="flex-1 bg-[var(--bg)]">
            <div className="mx-auto w-full max-w-[var(--container-max)] px-[var(--gutter-desktop)] py-[var(--section-y-desktop)]">
              {children}
            </div>
          </main>
        </div>
      </div>
    </AuthGuard>
  );
}
