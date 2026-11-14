/**
 * dbdlp 控制台 · 报告 / 规则 / 概览 / 审计 / 配置服务（02 文档 §13.2 其余端点）
 */

import { request, requestPage } from './http';
import type {
  AppConfig,
  AuditLog,
  DashboardSummary,
  PageData,
  ReportArtifact,
  ReportFormat,
  ReportGenerateResult,
  RuleListResult,
  RuleSnapshot,
} from '@/types/api';

/* ---------------------------------------------------------------- 报告 */

/** 报告列表 */
export function listReports(
  query: { taskId?: string; page?: number; limit?: number },
  signal?: AbortSignal,
): Promise<PageData<ReportArtifact>> {
  return requestPage<ReportArtifact>('/reports', query, signal);
}

/** 生成报告（可多格式） */
export function generateReport(input: {
  taskId: string;
  formats: ReportFormat[];
}): Promise<ReportGenerateResult[]> {
  return request<ReportGenerateResult[]>('/reports', { method: 'POST', body: input });
}

/** 下载报告（记录审计），返回文件流 */
export function downloadReport(id: string): Promise<Blob> {
  return request<Blob>(`/reports/${encodeURIComponent(id)}/download`, { method: 'GET', raw: true });
}

/* ---------------------------------------------------------------- 规则 */

/** 规则清单（当前或指定版本；P11 只读查看） */
export function listRules(version?: string, signal?: AbortSignal): Promise<RuleListResult> {
  return request<RuleListResult>('/rules', {
    method: 'GET',
    query: version ? { version } : undefined,
    signal,
  });
}

/** 规则版本快照列表 */
export function listRuleSnapshots(
  query: { page?: number; limit?: number },
  signal?: AbortSignal,
): Promise<PageData<RuleSnapshot>> {
  return requestPage<RuleSnapshot>('/rules/snapshots', query, signal);
}

/* ---------------------------------------------------------------- 概览 */

/** 概览聚合（ECharts 数据源） */
export function getDashboardSummary(
  taskId?: string,
  signal?: AbortSignal,
): Promise<DashboardSummary> {
  return request<DashboardSummary>('/dashboard/summary', {
    method: 'GET',
    query: taskId ? { taskId } : undefined,
    signal,
  });
}

/* ---------------------------------------------------------------- 审计 */

/** 审计日志（内容已脱敏） */
export function listAuditLogs(
  query: { page?: number; limit?: number; actor?: string; action?: string },
  signal?: AbortSignal,
): Promise<PageData<AuditLog>> {
  return requestPage<AuditLog>('/audit-logs', query, signal);
}

/* ---------------------------------------------------------------- 配置 */

/** 生效配置只读回显（不含密钥与 DSN） */
export function getConfig(signal?: AbortSignal): Promise<AppConfig> {
  return request<AppConfig>('/config', { method: 'GET', signal });
}
