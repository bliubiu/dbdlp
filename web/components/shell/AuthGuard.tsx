'use client';

/**
 * dbdlp 控制台 · 认证拦截层（04 文档 §2.2、§2.3 P13）
 *
 * 除 POST /api/v1/auth/login 外全部端点需 Bearer 会话；错误码 1002 触发跳转 /login，
 * 中文文案「登录状态已失效，请重新登录」。本期为单管理员 + 会话，不做 SSO / LDAP。
 */

import * as React from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuthStore } from '@/stores/useAuthStore';
import { Skeleton } from '@/components/ui/primitives';

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname() ?? '/';
  const router = useRouter();
  const token = useAuthStore((s) => s.token);
  const checked = useAuthStore((s) => s.checked);
  const checkSession = useAuthStore((s) => s.checkSession);
  const invalidate = useAuthStore((s) => s.invalidate);
  const [verifying, setVerifying] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    const verify = async () => {
      setVerifying(true);
      const ok = await checkSession();
      if (cancelled) return;
      setVerifying(false);
      if (!ok && useAuthStore.getState().token === null) {
        invalidate('登录状态已失效，请重新登录');
        router.replace(`/login?next=${encodeURIComponent(pathname)}`);
      }
    };
    void verify();
    return () => {
      cancelled = true;
    };
  }, [checkSession, invalidate, pathname, router]);

  // 校验中或尚未确认登录态：展示骨架，不渲染受保护内容
  if (verifying || !checked) {
    return (
      <div className="flex flex-col gap-[var(--space-4)] p-[var(--gutter-desktop)]" aria-busy="true">
        <Skeleton className="h-[32px] w-[240px]" />
        <Skeleton className="h-[120px] w-full" />
        <Skeleton className="h-[280px] w-full" />
        <span className="sr-only">正在校验登录状态</span>
      </div>
    );
  }

  if (!token) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-[var(--space-3)]">
        <p className="text-[var(--text-sm)] text-[var(--muted)]">登录状态已失效，请重新登录</p>
        <button
          type="button"
          onClick={() => router.replace(`/login?next=${encodeURIComponent(pathname)}`)}
          className="inline-flex h-[var(--btn-height)] items-center rounded-[var(--btn-radius)] bg-[var(--btn-primary-bg)] px-[var(--btn-padding-x)] text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--btn-primary-fg)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--btn-primary-bg-hover)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
        >
          前往登录
        </button>
      </div>
    );
  }

  return <>{children}</>;
}
