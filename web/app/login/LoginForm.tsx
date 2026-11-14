'use client';

/**
 * dbdlp 控制台 · 登录表单（04 文档 §2.3 P13、§3.6）
 *
 * 居中窄表单（400px），品牌标识 + 账号/密码 + 登录按钮；无营销文案。
 * 本期为单管理员 + 会话，不出现身份源选择、组织切换、角色分配等控件。
 */

import * as React from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { CircleX, Info, KeyRound, LoaderCircle, Lock, ShieldCheck, User } from 'lucide-react';
import { useAuthStore } from '@/stores/useAuthStore';
import { Button } from '@/components/ui/button';
import { Field, Input } from '@/components/ui/input';
import { Alert } from '@/components/ui/primitives';

/** 登录表单（中文错误近字段回显，§8.4） */
export function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const login = useAuthStore((s) => s.login);
  const pending = useAuthStore((s) => s.pending);
  const error = useAuthStore((s) => s.error);
  const clearError = useAuthStore((s) => s.clearError);

  const [username, setUsername] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [fieldErrors, setFieldErrors] = React.useState<{ username?: string; password?: string }>({});

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    clearError();
    const errors: { username?: string; password?: string } = {};

    if (!username.trim()) {
      errors.username = '请输入管理员账号';
    }
    if (!password) {
      errors.password = '请输入管理员密码';
    }
    setFieldErrors(errors);
    if (errors.username || errors.password) {
      // 焦点移到首个错误字段
      const first = document.getElementById(errors.username ? 'username' : 'password');
      first?.focus();
      return;
    }

    const ok = await login(username.trim(), password);
    if (ok) {
      const next = searchParams.get('next');
      router.replace(next && next.startsWith('/') ? next : '/');
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--bg)] px-[var(--gutter-phone)]">
      <div className="w-full max-w-[400px]">
        {/* 品牌标识 */}
        <div className="mb-[var(--space-6)] flex flex-col items-center gap-[var(--space-2)] text-center">
          <span className="inline-flex h-[48px] w-[48px] items-center justify-center rounded-[var(--radius-lg)] bg-[var(--surface-brand)]">
            <ShieldCheck size={24} className="text-[var(--fg-on-brand)]" aria-hidden="true" />
          </span>
          <h1 className="text-[var(--text-lg)] font-[var(--weight-announce)] text-[var(--fg)]">
            达梦敏感数据扫描发现平台
          </h1>
          <p className="text-[var(--text-xs)] text-[var(--muted)]">管理员登录</p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="flex flex-col gap-[var(--space-4)] rounded-[var(--card-radius)] border border-[var(--card-border)] bg-[var(--surface)] p-[var(--space-6)]"
          noValidate
        >
          {/* 顶层中文错误（会话过期/认证失败） */}
          {error ? (
            <Alert tone="danger" title="登录未通过">
              <span className="flex items-center gap-[var(--space-1)]">
                <CircleX size={16} aria-hidden="true" />
                {error}
              </span>
            </Alert>
          ) : null}

          <Field label="管理员账号" htmlFor="username" error={fieldErrors.username} required>
            <div className="relative">
              <User
                size={16}
                aria-hidden="true"
                className="pointer-events-none absolute left-[var(--space-3)] top-1/2 -translate-y-1/2 text-[var(--meta)]"
              />
              <Input
                id="username"
                name="username"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                aria-invalid={fieldErrors.username ? 'true' : undefined}
                aria-describedby={fieldErrors.username ? 'username-error' : undefined}
                placeholder="请输入管理员账号"
                className="pl-[var(--space-8)]"
              />
            </div>
          </Field>

          <Field label="管理员密码" htmlFor="password" error={fieldErrors.password} required>
            <div className="relative">
              <KeyRound
                size={16}
                aria-hidden="true"
                className="pointer-events-none absolute left-[var(--space-3)] top-1/2 -translate-y-1/2 text-[var(--meta)]"
              />
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-invalid={fieldErrors.password ? 'true' : undefined}
                aria-describedby={fieldErrors.password ? 'password-error' : undefined}
                placeholder="请输入管理员密码"
                className="pl-[var(--space-8)]"
              />
            </div>
          </Field>

          <Button type="submit" size="md" loading={pending} className="w-full">
            {pending ? (
              <>
                <LoaderCircle size={20} aria-hidden="true" className="animate-spin" />
                正在登录…
              </>
            ) : (
              <>
                <Lock size={20} aria-hidden="true" />
                登录
              </>
            )}
          </Button>

          <p className="flex items-start gap-[var(--space-2)] text-[var(--text-xs)] leading-[var(--leading-compact)] text-[var(--muted)]">
            <Info size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
            <span>连续登录失败由服务端递增锁定；本期为单管理员会话，不接入企业身份源。</span>
          </p>
        </form>
      </div>
    </div>
  );
}
