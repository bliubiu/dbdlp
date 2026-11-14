'use client';

/**
 * dbdlp 控制台 · ECharts 封装（04 文档 §4.1、§4.2）
 *
 * 约束：
 * - option 内不写字面色值，统一引用 chartTheme（从 CSS 变量读取）
 * - 读取在客户端挂载后执行，避免 SSR 阶段 document 不可用
 * - 图表有标题、口径副标题、图例（含文字）、tooltip（数量+占比+口径说明）
 * - 坐标轴从 0 起；网格线 --chart-grid、轴标签 --chart-label、轴线 --chart-axis
 */

import * as React from 'react';
import * as echarts from 'echarts/core';
import { BarChart, LineChart, PieChart } from 'echarts/charts';
import {
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
} from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { buildChartTheme, type ChartTheme } from '@/lib/chart-theme';

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
  CanvasRenderer,
]);

export interface EChartProps {
  /** ECharts option；色值必须来自 chartTheme，不得写裸 hex */
  option: (theme: ChartTheme) => echarts.EChartsCoreOption;
  /** 无障碍标题（图表本身的替代文本） */
  ariaLabel: string;
  className?: string;
  /** 图表高度 */
  height?: number;
}

/**
 * 图表容器：挂载后初始化 ECharts，主题从 CSS 变量读取。
 * 卸载时销毁实例，避免内存泄漏。
 */
export function EChart({ option, ariaLabel, className, height = 260 }: EChartProps) {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const chartRef = React.useRef<echarts.ECharts | null>(null);
  const [theme, setTheme] = React.useState<ChartTheme | null>(null);

  // 挂载后读取 CSS 变量（客户端）
  React.useEffect(() => {
    setTheme(buildChartTheme());
  }, []);

  React.useEffect(() => {
    if (!theme || !containerRef.current) return;
    const chart = echarts.init(containerRef.current, undefined, { renderer: 'canvas' });
    chartRef.current = chart;

    const resize = () => chart.resize();
    window.addEventListener('resize', resize);

    return () => {
      window.removeEventListener('resize', resize);
      chart.dispose();
      chartRef.current = null;
    };
  }, [theme]);

  React.useEffect(() => {
    if (!chartRef.current || !theme) return;
    chartRef.current.setOption(option(theme), true);
  }, [option, theme]);

  return (
    <div
      ref={containerRef}
      role="img"
      aria-label={ariaLabel}
      className={className}
      style={{ height: `${height}px`, width: '100%' }}
    />
  );
}

/** 通用饼图 option（风险四色固定顺序，§4.1 C1/C5） */
export function buildPieOption(
  theme: ChartTheme,
  data: { name: string; value: number }[],
  colors: string[],
  centerLabel?: string,
) {
  const total = data.reduce((sum, item) => sum + item.value, 0);
  return {
    tooltip: {
      trigger: 'item',
      formatter: (params: unknown) => {
        const p = params as { name: string; value: number; percent: number };
        return `${p.name}<br/>字段数：${p.value}<br/>占比：${p.percent}%`;
      },
    },
    legend: {
      bottom: 0,
      icon: 'circle',
      itemWidth: 8,
      itemHeight: 8,
      textStyle: { color: theme.textColor, fontSize: 12, fontFamily: theme.fontFamily },
    },
    series: [
      {
        type: 'pie',
        radius: centerLabel ? ['55%', '75%'] : ['0%', '68%'],
        center: ['50%', '45%'],
        avoidLabelOverlap: true,
        label: {
          show: Boolean(centerLabel),
          position: 'center',
          formatter: centerLabel ?? '',
          color: theme.textColor,
          fontSize: 12,
          fontFamily: theme.fontFamily,
        },
        emphasis: { label: { show: Boolean(centerLabel) } },
        labelLine: { show: false },
        data: data.map((item, index) => ({
          name: item.name,
          value: item.value,
          itemStyle: { color: colors[index % colors.length] },
        })),
        ...(centerLabel
          ? {
              graphic: {
                type: 'text',
                left: 'center',
                top: 'center',
                style: {
                  text: `${centerLabel}\n${total}`,
                  textAlign: 'center',
                  fill: theme.textColor,
                  fontSize: 12,
                  lineHeight: 18,
                  fontFamily: theme.fontFamily,
                },
              },
            }
          : {}),
      },
    ],
  };
}

/** 通用柱状图 option（坐标轴从 0 起） */
export function buildBarOption(
  theme: ChartTheme,
  categories: string[],
  series: { name: string; data: number[]; colorIndex?: number }[],
  orientation: 'horizontal' | 'vertical' = 'vertical',
) {
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: {
      bottom: 0,
      icon: 'circle',
      itemWidth: 8,
      itemHeight: 8,
      textStyle: { color: theme.textColor, fontSize: 12, fontFamily: theme.fontFamily },
    },
    grid: { left: 8, right: 16, top: 16, bottom: 32, containLabel: true },
    xAxis: {
      type: 'category',
      data: categories,
      min: 0,
      axisLine: { lineStyle: { color: theme.axis } },
      axisTick: { show: false },
      axisLabel: { color: theme.label, fontSize: 12, fontFamily: theme.fontFamily },
    },
    yAxis: {
      type: 'value',
      min: 0,
      splitLine: { lineStyle: { color: theme.grid } },
      axisLabel: { color: theme.label, fontSize: 12, fontFamily: theme.fontFamily },
    },
    series: series.map((item, index) => ({
      name: item.name,
      type: 'bar',
      data: item.data,
      itemStyle: {
        color: theme.series[(item.colorIndex ?? index) % theme.series.length],
        borderRadius: [2, 2, 0, 0],
      },
      barMaxWidth: 28,
    })),
    ...(orientation === 'horizontal'
      ? {
          xAxis: {
            type: 'value',
            min: 0,
            splitLine: { lineStyle: { color: theme.grid } },
            axisLabel: { color: theme.label, fontSize: 12, fontFamily: theme.fontFamily },
          },
          yAxis: {
            type: 'category',
            data: categories,
            axisLine: { lineStyle: { color: theme.axis } },
            axisTick: { show: false },
            axisLabel: { color: theme.label, fontSize: 12, fontFamily: theme.fontFamily },
          },
        }
      : {}),
  };
}

/** 通用折线图 option（双序列趋势） */
export function buildLineOption(
  theme: ChartTheme,
  categories: string[],
  series: { name: string; data: number[] }[],
) {
  return {
    tooltip: { trigger: 'axis' },
    legend: {
      bottom: 0,
      icon: 'circle',
      itemWidth: 8,
      itemHeight: 8,
      textStyle: { color: theme.textColor, fontSize: 12, fontFamily: theme.fontFamily },
    },
    grid: { left: 8, right: 16, top: 16, bottom: 32, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: categories,
      axisLine: { lineStyle: { color: theme.axis } },
      axisTick: { show: false },
      axisLabel: { color: theme.label, fontSize: 12, fontFamily: theme.fontFamily },
    },
    yAxis: {
      type: 'value',
      min: 0,
      splitLine: { lineStyle: { color: theme.grid } },
      axisLabel: { color: theme.label, fontSize: 12, fontFamily: theme.fontFamily },
    },
    series: series.map((item, index) => ({
      name: item.name,
      type: 'line',
      smooth: false,
      data: item.data,
      symbolSize: 5,
      lineStyle: { width: 2, color: theme.series[index % theme.series.length] },
      itemStyle: { color: theme.series[index % theme.series.length] },
    })),
  };
}
