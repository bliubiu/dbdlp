'use client';

/**
 * dbdlp 控制台 · 字段明细切片（04 文档 §2.5，P08）
 *
 * 职责：当前 id、Tab、复核草稿。持久化：否。
 * 约束：复核结论与原识别结论分开留存，不覆盖原始结论（§2.3 P08）。
 */

import { create } from 'zustand';
import { getFinding, listFindingHits, listReviews, submitReview } from '@/services/findings';
import { ApiError } from '@/services/http';
import type {
  ColumnFinding,
  HitRecord,
  ReviewConclusion,
  ReviewRecord,
} from '@/types/api';

export type FindingDetailTab = 'basic' | 'hits' | 'review';

interface FindingDetailState {
  id: string | null;
  finding: ColumnFinding | null;
  hits: HitRecord[];
  reviews: ReviewRecord[];
  totalHits: number;
  tab: FindingDetailTab;
  loading: boolean;
  error: string | null;
  /** 复核草稿 */
  draft: { conclusion: ReviewConclusion; comment: string };
  submitting: boolean;
  notice: string | null;
  /** 最近一次提示的性质（成功 / 失败），用于近字段错误回显 */
  noticeTone: 'success' | 'error' | null;
  load: (id: string) => Promise<void>;
  setTab: (tab: FindingDetailTab) => void;
  setDraft: (draft: Partial<{ conclusion: ReviewConclusion; comment: string }>) => void;
  submit: (id: string) => Promise<boolean>;
  clearNotice: () => void;
}

export const useFindingDetailStore = create<FindingDetailState>((set, get) => ({
  id: null,
  finding: null,
  hits: [],
  reviews: [],
  totalHits: 0,
  tab: 'basic',
  loading: false,
  error: null,
  draft: { conclusion: 'pending', comment: '' },
  submitting: false,
  notice: null,
  noticeTone: null,

  load: async (id) => {
    set({ loading: true, error: null, id });
    try {
      const [finding, hits, reviews] = await Promise.all([
        getFinding(id),
        listFindingHits(id, { page: 1, limit: 50 }),
        listReviews(id, { page: 1, limit: 50 }),
      ]);
      set({
        finding,
        hits: hits.items ?? [],
        totalHits: hits.total ?? 0,
        reviews: reviews.items ?? [],
        loading: false,
      });
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '字段明细加载失败，请稍后重试';
      set({ loading: false, error: message, finding: null });
    }
  },

  setTab: (tab) => set({ tab }),

  setDraft: (draft) => set({ draft: { ...get().draft, ...draft } }),

  submit: async (id) => {
    const { draft } = get();
    if (!draft.comment.trim()) {
      set({ notice: '请填写复核说明，便于后续追溯', noticeTone: 'error' });
      return false;
    }
    set({ submitting: true, notice: null, noticeTone: null });
    try {
      await submitReview({
        findingId: id,
        conclusion: draft.conclusion,
        comment: draft.comment.trim(),
      });
      const reviews = await listReviews(id, { page: 1, limit: 50 });
      set({
        submitting: false,
        reviews: reviews.items ?? [],
        notice: '复核结论已提交，原识别结论保持不变',
        noticeTone: 'success',
        draft: { conclusion: 'pending', comment: '' },
      });
      return true;
    } catch (error) {
      const message = error instanceof ApiError ? error.message : '复核提交失败，请稍后重试';
      set({ submitting: false, notice: message, noticeTone: 'error' });
      return false;
    }
  },

  clearNotice: () => set({ notice: null, noticeTone: null }),
}));
