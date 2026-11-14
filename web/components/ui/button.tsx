'use client';

/**
 * dbdlp 控制台 · 基础组件层（shadcn/ui registry 风格，按锁定版本官方用法接入）
 *
 * 组件样式全部引用 design-tokens CSS 变量，不出现裸 hex。
 */

import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

/* ------------------------------------------------------------- Button */

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[var(--btn-radius)] text-[var(--text-sm)] font-[var(--weight-emphasize)] transition-colors duration-[var(--motion-fast)] ease-[var(--ease-standard)] focus-visible:outline-none disabled:pointer-events-none disabled:opacity-100 [&_svg]:pointer-events-none [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        primary:
          'bg-[var(--btn-primary-bg)] text-[var(--btn-primary-fg)] hover:bg-[var(--btn-primary-bg-hover)] active:bg-[var(--btn-primary-bg-active)]',
        secondary:
          'border border-[var(--btn-secondary-border)] bg-[var(--btn-secondary-bg)] text-[var(--btn-secondary-fg)] hover:bg-[var(--btn-secondary-bg-hover)]',
        ghost:
          'bg-[var(--btn-ghost-bg)] text-[var(--btn-ghost-fg)] hover:bg-[var(--btn-ghost-bg-hover)]',
        destructive:
          'bg-[var(--btn-destructive-bg)] text-[var(--btn-destructive-fg)] hover:bg-[var(--btn-destructive-bg-hover)]',
      },
      size: {
        sm: 'h-[var(--btn-height)] px-[var(--btn-padding-x)]',
        md: 'h-[var(--btn-height-lg)] px-[var(--btn-padding-x)]',
        icon: 'h-[var(--btn-height)] w-[var(--btn-height)] p-0',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
);

export interface ButtonProps
  extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'disabled'>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
  /** 加载态：显示中文进行态文案并禁用 */
  loading?: boolean;
  /** 禁用态 */
  disabled?: boolean;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, disabled, asChild = false, loading = false, children, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button';
    return (
      <Comp
        ref={ref}
        className={cn(
          buttonVariants({ variant, size }),
          disabled || loading
            ? 'bg-[var(--btn-disabled-bg)] text-[var(--btn-disabled-fg)]'
            : '',
          className,
        )}
        disabled={disabled || loading}
        aria-busy={loading || undefined}
        {...props}
      >
        {children}
      </Comp>
    );
  },
);
Button.displayName = 'Button';

export { buttonVariants };
