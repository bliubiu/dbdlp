'use client';

/**
 * dbdlp 控制台 · 页面级密度覆盖（04 文档 §2.6）
 *
 * 规格约定：
 * - compact：P02 / P04 / P05 / P06 / P07 / P11
 * - default：P01 / P03 / P08 / P09 / P10 / P12 / P13
 *
 * 用户在 P12 显式选择密度后，页面级默认不再覆盖用户选择。
 */

import * as React from 'react';
import { useUIStore, type Density } from '@/stores/useUIStore';

/** 页面级密度覆盖组件 */
export function PageDensity({ density }: { density: Density }) {
  const densityExplicit = useUIStore((s) => s.densityExplicit);

  React.useEffect(() => {
    if (densityExplicit) return;
    if (typeof document === 'undefined') return;
    document.documentElement.setAttribute('data-density', density);
  }, [density, densityExplicit]);

  return null;
}
