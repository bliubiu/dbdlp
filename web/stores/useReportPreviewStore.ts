'use client';

/**
 * dbdlp 控制台 · 报告预览切片（04 文档 §2.5，P10）
 *
 * 职责：章节锚点、打印模式。持久化：否。
 */

import { create } from 'zustand';

/** 报告八章节锚点（对齐 §14.1 HTML 审计报告结构） */
export const REPORT_SECTIONS = [
  { id: 'overview', label: '扫描概况' },
  { id: 'risk', label: '风险汇总' },
  { id: 'type', label: '敏感类型分布' },
  { id: 'schema', label: '模式分布' },
  { id: 'ledger', label: '字段台账' },
  { id: 'hit', label: '命中明细' },
  { id: 'review', label: '复核记录' },
  { id: 'appendix', label: '附录' },
];

interface ReportPreviewState {
  activeSection: string;
  printing: boolean;
  setActiveSection: (id: string) => void;
  setPrinting: (printing: boolean) => void;
}

export const useReportPreviewStore = create<ReportPreviewState>((set) => ({
  activeSection: 'overview',
  printing: false,
  setActiveSection: (activeSection) => set({ activeSection }),
  setPrinting: (printing) => set({ printing }),
}));
