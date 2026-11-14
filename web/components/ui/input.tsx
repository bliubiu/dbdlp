'use client';

/**
 * dbdlp 控制台 · 基础组件：Input / Textarea / Label
 *
 * 规范（04 文档 §8.4）：每个输入框有可见中文 label，不以 placeholder 代替 label；
 * 错误提示紧贴字段下方，字段用 aria-invalid + aria-describedby 关联中文错误文案。
 */

import * as React from 'react';
import * as LabelPrimitive from '@radix-ui/react-label';
import { cn } from '@/lib/utils';

export const Label = React.forwardRef<
  React.ComponentRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root>
>(({ className, ...props }, ref) => (
  <LabelPrimitive.Root
    ref={ref}
    className={cn(
      'text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)] leading-[var(--leading-compact)]',
      className,
    )}
    {...props}
  />
));
Label.displayName = LabelPrimitive.Root.displayName;

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type = 'text', ...props }, ref) => (
    <input
      ref={ref}
      type={type}
      className={cn(
        'flex h-[var(--input-height)] w-full rounded-[var(--input-radius)] border border-[var(--input-border)] bg-[var(--input-bg)] px-[var(--space-3)] text-[var(--text-sm)] text-[var(--input-fg)] transition-colors duration-[var(--motion-fast)] ease-[var(--ease-standard)]',
        'placeholder:text-[var(--input-placeholder)]',
        'hover:border-[var(--input-border-hover)]',
        'focus-visible:border-[var(--input-border-focus)] focus-visible:outline-none focus-visible:shadow-[var(--input-ring-focus)]',
        'disabled:cursor-not-allowed disabled:border-[var(--input-border)] disabled:bg-[var(--input-disabled-bg)] disabled:text-[var(--input-disabled-fg)]',
        'aria-[invalid=true]:border-[var(--input-border-error)] aria-[invalid=true]:shadow-[var(--input-ring-error)]',
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = 'Input';

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    className={cn(
      'flex min-h-[80px] w-full rounded-[var(--input-radius)] border border-[var(--input-border)] bg-[var(--input-bg)] px-[var(--space-3)] py-[var(--space-2)] text-[var(--text-sm)] text-[var(--input-fg)] transition-colors duration-[var(--motion-fast)] ease-[var(--ease-standard)]',
      'placeholder:text-[var(--input-placeholder)]',
      'hover:border-[var(--input-border-hover)]',
      'focus-visible:border-[var(--input-border-focus)] focus-visible:outline-none focus-visible:shadow-[var(--input-ring-focus)]',
      'disabled:cursor-not-allowed disabled:bg-[var(--input-disabled-bg)] disabled:text-[var(--input-disabled-fg)]',
      'aria-[invalid=true]:border-[var(--input-border-error)] aria-[invalid=true]:shadow-[var(--input-ring-error)]',
      className,
    )}
    {...props}
  />
));
Textarea.displayName = 'Textarea';

/** 表单字段容器：label + 控件 + 近字段中文错误 */
export function Field({
  label,
  htmlFor,
  error,
  hint,
  required,
  children,
  className,
}: {
  label: string;
  htmlFor: string;
  error?: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex flex-col gap-[var(--space-1)]', className)}>
      <Label htmlFor={htmlFor}>
        {label}
        {required ? <span className="ml-[var(--space-1)] text-[var(--danger)]">*</span> : null}
      </Label>
      {children}
      {hint && !error ? (
        <p className="text-[var(--text-xs)] text-[var(--muted)]">{hint}</p>
      ) : null}
      {error ? (
        <p
          id={`${htmlFor}-error`}
          role="alert"
          className="text-[var(--text-xs)] text-[var(--danger)]"
        >
          {error}
        </p>
      ) : null}
    </div>
  );
}
