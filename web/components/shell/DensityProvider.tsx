'use client';

/**
 * dbdlp 控制台 · 密度档同步（04 文档 §5.6）
 *
 * 将 useUIStore 的密度档写入 <html data-density>，驱动 design-tokens.css 的
 * 页面级令牌覆盖（--table-row-h / --table-cell-py）。
 */

import * as React from 'react';
import { useUIStore } from '@/stores/useUIStore';

export function DensityProvider({ children }: { children: React.ReactNode }) {
  const density = useUIStore((s) => s.density);

  React.useEffect(() => {
    document.documentElement.setAttribute('data-density', density);
  }, [density]);

  return <>{children}</>;
}
