import { Suspense } from 'react';
import type { Metadata } from 'next';
import { LoginForm } from './LoginForm';

export const metadata: Metadata = {
  title: '管理员登录 · 达梦敏感数据扫描发现平台',
};

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-[var(--bg)]">
          <p className="text-[var(--text-sm)] text-[var(--muted)]">正在加载登录页…</p>
        </div>
      }
    >
      <LoginForm />
    </Suspense>
  );
}
