/**
 * dbdlp 控制台 · 台账与复核服务（02 文档 §13.2 findings / reviews 端点组）
 *
 * 掩码约束（04 文档 §4.6）：前端只消费 sampleMasked，接口契约不返回 raw_value 等明文字段。
 */

import { request, requestPage, type QueryParams } from './http';
import type {
  ColumnFinding,
  HitRecord,
  PageData,
  ReviewInput,
  ReviewRecord,
} from '@/types/api';

export interface FindingQuery extends QueryParams {
  taskId?: string;
  riskLevel?: string;
  reviewStatus?: string;
  keyword?: string;
  page?: number;
  limit?: number;
  sort?: string;
  order?: 'asc' | 'desc';
}

/** 字段台账（服务端分页） */
export function listFindings(
  query: FindingQuery,
  signal?: AbortSignal,
): Promise<PageData<ColumnFinding>> {
  return requestPage<ColumnFinding>('/findings', query, signal);
}

/** 字段详情（含掩码样例） */
export function getFinding(id: string, signal?: AbortSignal): Promise<ColumnFinding> {
  return request<ColumnFinding>(`/findings/${encodeURIComponent(id)}`, { method: 'GET', signal });
}

/** 命中明细（仅掩码值） */
export function listFindingHits(
  id: string,
  query: { page?: number; limit?: number },
  signal?: AbortSignal,
): Promise<PageData<HitRecord>> {
  return requestPage<HitRecord>(`/findings/${encodeURIComponent(id)}/hits`, query, signal);
}

/** 提交复核（不覆盖原始识别结论） */
export function submitReview(input: ReviewInput): Promise<{ id: string }> {
  return request<{ id: string }>('/reviews', { method: 'POST', body: input });
}

/** 复核历史 */
export function listReviews(
  findingId: string,
  query: { page?: number; limit?: number },
  signal?: AbortSignal,
): Promise<PageData<ReviewRecord>> {
  return requestPage<ReviewRecord>('/reviews', { ...query, findingId }, signal);
}
