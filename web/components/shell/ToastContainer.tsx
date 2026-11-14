'use client';

/**
 * dbdlp 控制台 · Toast 容器（04 文档 §2.2）
 *
 * 层级 --z-toast；用 aria-live 播报但不抢焦点（§8.3）。
 */

import { CircleCheck, CircleX, Info, TriangleExclamationPoint, X } from 'lucide-react';
import { useUIStore, type Toast } from '@/stores/useUIStore';
import { cn } from '@/lib/utils';

const TONE_STYLE: Record<Toast['tone'], { cls: string; Icon: typeof Info }> = {
  success: { cls: 'border-[var(--success-border)] bg-[var(--success-bg)] text-[var(--success)]', Icon: CircleCheck },
  error: { cls: 'border-[var(--danger-border)] bg-[var(--danger-bg)] text-[var(--danger)]', Icon: CircleX },
  warning: { cls: 'border-[var(--warn-border)] bg-[var(--warn-bg)] text-[var(--warn)]', Icon: TriangleExclamationPoint },
  info: { cls: 'border-[var(--info-border)] bg-[var(--info-bg)] text-[var(--info)]', Icon: Info },
};

export function ToastContainer() {
  const toasts = useUIStore((s) => s.toasts);
  const dismiss = useUIStore((s) => s.dismissToast);

  return (
    <div
      aria-live="polite"
      aria-atomic="false"
      className="no-print pointer-events-none fixed bottom-[var(--space-4)] right-[var(--space-4)] z-[var(--z-toast)] flex w-[min(360px,calc(100vw-32px))] flex-col gap-[var(--space-2)]"
    >
      {toasts.map((toast) => {
        const style = TONE_STYLE[toast.tone];
        const Icon = style.Icon;
        return (
          <div
            key={toast.id}
            role="status"
            className={cn(
              'pointer-events-auto flex items-start gap-[var(--space-2)] rounded-[var(--radius-md)] border px-[var(--space-3)] py-[var(--space-2)] text-[var(--text-sm)] shadow-[var(--toast-elev)]',
              style.cls,
            )}
          >
            <Icon size={16} aria-hidden="true" className="mt-[2px] shrink-0" />
            <span className="flex-1 leading-[var(--leading-compact)]">{toast.message}</span>
            <button
              type="button"
              onClick={() => dismiss(toast.id)}
              aria-label="关闭提示"
              className="shrink-0 rounded-[var(--radius-sm)] p-[2px] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
            >
              <X size={14} aria-hidden="true" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
