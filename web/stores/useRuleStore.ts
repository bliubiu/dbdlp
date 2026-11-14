'use client';

/**
 * dbdlp 控制台 · 规则切片（04 文档 §2.5，P11）
 *
 * 职责：规则列表、选中版本、快照列表。持久化：否。
 * 边界：P11 **只读查看**，不提供在线编辑（§2.3、§10 禁止项 10）。
 */

import { create } from 'zustand';
import { listRules, listRuleSnapshots } from '@/services/system';
import { ApiError } from '@/services/http';
import type { RuleItem, RuleSnapshot } from '@/types/api';

interface RuleState {
  rules: RuleItem[];
  version: string;
  snapshots: RuleSnapshot[];
  totalSnapshots: number;
  /** 选中的规则 id（右侧详情面板） */
  selectedId: string | null;
  loading: boolean;
  error: string | null;
  fetchRules: (version?: string) => Promise<void>;
  fetchSnapshots: () => Promise<void>;
  setSelected: (id: string | null) => void;
}

export const useRuleStore = create<RuleState>((set) => ({
  rules: [],
  version: '',
  snapshots: [],
  totalSnapshots: 0,
  selectedId: null,
  loading: false,
  error: null,

  fetchRules: async (version) => {
    set({ loading: true, error: null });
    try {
      const result = await listRules(version);
      set({
        rules: result.rules ?? [],
        version: result.version ?? '',
        loading: false,
      });
    } catch (error) {
      // 2002 规则加载失败 → 中文 Alert（§2.4）
      const message = error instanceof ApiError ? error.message : '规则清单加载失败，请稍后重试';
      set({ loading: false, error: message, rules: [] });
    }
  },

  fetchSnapshots: async () => {
    try {
      const result = await listRuleSnapshots({ page: 1, limit: 50 });
      set({ snapshots: result.items ?? [], totalSnapshots: result.total ?? 0 });
    } catch {
      /* 快照列表失败不阻断主表 */
    }
  },

  setSelected: (selectedId) => set({ selectedId }),
}));
