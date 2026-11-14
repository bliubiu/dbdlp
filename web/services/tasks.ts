/**
 * dbdlp 控制台 · 扫描任务服务（02 文档 §13.2 tasks 端点组）
 */

import { request, requestPage, type QueryParams } from './http';
import type { PageData, ScanTask, TaskCreateInput, TaskProgress, TaskStatus } from '@/types/api';

export interface TaskQuery extends QueryParams {
  page?: number;
  limit?: number;
  status?: string;
  dataSourceId?: string;
}

/** 任务列表 */
export function listTasks(query: TaskQuery, signal?: AbortSignal): Promise<PageData<ScanTask>> {
  return requestPage<ScanTask>('/tasks', query, signal);
}

/** 任务详情（含 summary） */
export function getTask(id: string, signal?: AbortSignal): Promise<ScanTask> {
  return request<ScanTask>(`/tasks/${encodeURIComponent(id)}`, { method: 'GET', signal });
}

/** 进度轮询 */
export function getTaskProgress(id: string, signal?: AbortSignal): Promise<TaskProgress> {
  return request<TaskProgress>(`/tasks/${encodeURIComponent(id)}/progress`, {
    method: 'GET',
    signal,
  });
}

/** 发起扫描任务（参数缺省取后端 toml） */
export function createTask(input: TaskCreateInput): Promise<{ taskId: string }> {
  return request<{ taskId: string }>('/tasks', { method: 'POST', body: input });
}

/** 暂停并落检查点 */
export function pauseTask(id: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/tasks/${encodeURIComponent(id)}/pause`, { method: 'POST' });
}

/** 断点续扫 */
export function resumeTask(id: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/tasks/${encodeURIComponent(id)}/resume`, { method: 'POST' });
}

/** POST /tasks/{id}/retry 响应：真实重扫后的统计（targetIds 为空表示全部失败表） */
export interface RetryTaskResult {
  /** 本次实际重扫的表数 */
  retried: number;
  /** 本次新增的台账条数 */
  newFindings: number;
  /** 重扫后仍失败的表数 */
  remaining: number;
  /** 重扫后的任务状态 */
  status: TaskStatus;
}

/** 重试失败对象：同步重扫失败表，返回成功后刷新详情 */
export function retryTask(id: string, targetIds: string[]): Promise<RetryTaskResult> {
  return request<RetryTaskResult>(`/tasks/${encodeURIComponent(id)}/retry`, {
    method: 'POST',
    body: { targetIds },
  });
}
