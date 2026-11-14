'use client';

/**
 * dbdlp 控制台 · ECharts 主题桥接（04 文档 §4.2）
 *
 * 关键约束：ECharts 需要具体色值，但 option 内**不写字面色值**。
 * 统一通过 getComputedStyle 从 CSS 变量读取，色值全部来自 design-tokens。
 *
 * 读取必须在客户端组件挂载后执行，避免 SSR 阶段 document 不可用。
 */

/** 从 :root 读取 CSS 变量值 */
export function readCssVar(name: string): string {
  if (typeof window === 'undefined') return '';
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

/** 图表主题（与 §4.2 代码块一致） */
export interface ChartTheme {
  risk: {
    high: string;
    medium: string;
    low: string;
    pending: string;
  };
  series: string[];
  axis: string;
  grid: string;
  label: string;
  fontFamily: string;
  textColor: string;
  fontMono: string;
}

/** 构建图表主题 */
export function buildChartTheme(): ChartTheme {
  return {
    risk: {
      high: readCssVar('--risk-high-fill'),
      medium: readCssVar('--risk-medium-fill'),
      low: readCssVar('--risk-low-fill'),
      pending: readCssVar('--risk-pending-fill'),
    },
    series: [1, 2, 3, 4, 5, 6].map((i) => readCssVar(`--chart-${i}`)),
    axis: readCssVar('--chart-axis'),
    grid: readCssVar('--chart-grid'),
    label: readCssVar('--chart-label'),
    fontFamily: readCssVar('--font-body'),
    textColor: readCssVar('--muted'),
    fontMono: readCssVar('--font-mono'),
  };
}

/** 风险四色固定顺序（高→中→低→待复核，§4.1 C1/C4） */
export function riskColorSeries(theme: ChartTheme): string[] {
  return [theme.risk.high, theme.risk.medium, theme.risk.low, theme.risk.pending];
}
