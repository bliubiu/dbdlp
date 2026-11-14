'use client';

/**
 * dbdlp 控制台 · 基础组件：Card / Badge / Alert / Progress / Skeleton / Separator
 *
 * 卡片规范（04 文档 §5.3）：1px 边框环（--card-elev），圆角 --card-radius（12px），
 * **不同时用 1px 实线边框与 blur≥16px 阴影**；不用彩色左边框、发光边框、毛玻璃。
 */

import * as React from 'react';
import { cn } from '@/lib/utils';

/* ---------------------------------------------------------------- Card */

export const Card = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & { interactive?: boolean }
>(({ className, interactive = false, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      'rounded-[var(--card-radius)] border border-[var(--card-border)] bg-[var(--card-bg)] text-[var(--card-fg)]',
      interactive
        ? 'transition-colors duration-[var(--motion-fast)] ease-[var(--ease-standard)] hover:border-[var(--card-border-hover)]'
        : '',
      className,
    )}
    {...props}
  />
));
Card.displayName = 'Card';

export const CardHeader = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('flex flex-col gap-[var(--space-1)] p-[var(--card-padding)]', className)}
      {...props}
    />
  ),
);
CardHeader.displayName = 'CardHeader';

export const CardTitle = React.forwardRef<
  HTMLHeadingElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h3
    ref={ref}
    className={cn(
      'text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)] leading-[var(--leading-tight)]',
      className,
    )}
    {...props}
  />
));
CardTitle.displayName = 'CardTitle';

export const CardDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <p
    ref={ref}
    className={cn('text-[var(--text-xs)] text-[var(--muted)]', className)}
    {...props}
  />
));
CardDescription.displayName = 'CardDescription';

export const CardContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('p-[var(--card-padding)] pt-0', className)} {...props} />
  ),
);
CardContent.displayName = 'CardContent';

export const CardFooter = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('flex items-center gap-[var(--space-2)] p-[var(--card-padding)] pt-0', className)}
      {...props}
    />
  ),
);
CardFooter.displayName = 'CardFooter';

/* --------------------------------------------------------------- Badge */

export type BadgeTone =
  | 'neutral'
  | 'info'
  | 'success'
  | 'warning'
  | 'danger'
  | 'accent';

const BADGE_TONE: Record<BadgeTone, string> = {
  neutral: 'bg-[var(--badge-bg)] text-[var(--badge-fg)] border-[var(--badge-border)]',
  info: 'bg-[var(--info-bg)] text-[var(--info)] border-[var(--info-border)]',
  success: 'bg-[var(--success-bg)] text-[var(--success)] border-[var(--success-border)]',
  warning: 'bg-[var(--warn-bg)] text-[var(--warn)] border-[var(--warn-border)]',
  danger: 'bg-[var(--danger-bg)] text-[var(--danger)] border-[var(--danger-border)]',
  accent: 'bg-[var(--accent-soft)] text-[var(--accent)] border-[var(--accent-border)]',
};

export function Badge({
  className,
  tone = 'neutral',
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: BadgeTone }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-[var(--space-1)] whitespace-nowrap rounded-[var(--badge-radius)] border px-[var(--badge-padding-x)] py-[var(--badge-padding-y)] text-[var(--text-xs)] font-[var(--weight-emphasize)] leading-[var(--leading-compact)]',
        BADGE_TONE[tone],
        className,
      )}
      {...props}
    />
  );
}

/* --------------------------------------------------------------- Alert */

export type AlertTone = 'info' | 'success' | 'warning' | 'danger';

const ALERT_TONE: Record<AlertTone, string> = {
  info: 'bg-[var(--info-bg)] text-[var(--info)] border-[var(--info-border)]',
  success: 'bg-[var(--success-bg)] text-[var(--success)] border-[var(--success-border)]',
  warning: 'bg-[var(--warn-bg)] text-[var(--warn)] border-[var(--warn-border)]',
  danger: 'bg-[var(--danger-bg)] text-[var(--danger)] border-[var(--danger-border)]',
};

export function Alert({
  className,
  tone = 'info',
  title,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & { tone?: AlertTone; title?: string }) {
  return (
    <div
      role="alert"
      className={cn(
        'flex flex-col gap-[var(--space-1)] rounded-[var(--radius-md)] border px-[var(--space-3)] py-[var(--space-2)] text-[var(--text-sm)]',
        ALERT_TONE[tone],
        className,
      )}
      {...props}
    >
      {title ? <span className="font-[var(--weight-emphasize)]">{title}</span> : null}
      {children}
    </div>
  );
}

/* ------------------------------------------------------------- Progress */

export function Progress({
  className,
  value = 0,
  label,
}: {
  className?: string;
  value: number;
  /** 中文进度说明（无障碍：进度以文本表达） */
  label?: string;
}) {
  const pct = Math.max(0, Math.min(100, Math.round(value)));
  return (
    <div className={cn('flex flex-col gap-[var(--space-1)]', className)}>
      <div
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={label ?? '进度'}
        className="h-[6px] w-full overflow-hidden rounded-[var(--progress-radius)] bg-[var(--progress-track)]"
      >
        <div
          className="h-full rounded-[var(--progress-radius)] bg-[var(--progress-fill)] transition-[width] duration-[var(--motion-base)] ease-[var(--ease-standard)]"
          style={{ width: `${pct}%` }}
        />
      </div>
      {label ? <span className="text-[var(--text-xs)] text-[var(--muted)]">{label}</span> : null}
    </div>
  );
}

/* ------------------------------------------------------------- Skeleton */

export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('animate-pulse rounded-[var(--radius-sm)] bg-[var(--skeleton-bg)]', className)}
      {...props}
    />
  );
}

/* ----------------------------------------------------------- Separator */

export function Separator({
  className,
  orientation = 'horizontal',
  ...props
}: React.HTMLAttributes<HTMLDivElement> & { orientation?: 'horizontal' | 'vertical' }) {
  return (
    <div
      role="separator"
      aria-orientation={orientation}
      className={cn(
        'bg-[var(--border-soft)]',
        orientation === 'horizontal' ? 'h-px w-full' : 'h-full w-px',
        className,
      )}
      {...props}
    />
  );
}
