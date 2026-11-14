/**
 * dbdlp 控制台 · 数据源服务（02 文档 §13.2 datasources 端点组）
 *
 * 凭据不出后端：响应中 password 恒为占位串，前端不回显、不缓存。
 */

import { request, requestPage, type QueryParams } from './http';
import type {
  ConnectionTestResult,
  DataSource,
  DataSourceCreateResult,
  DataSourceInput,
  DataSourceMetadata,
  PageData,
} from '@/types/api';

export interface DataSourceQuery extends QueryParams {
  page?: number;
  limit?: number;
  dialect?: string;
  keyword?: string;
  sort?: string;
}

/** 数据源列表（服务端分页） */
export function listDataSources(
  query: DataSourceQuery,
  signal?: AbortSignal,
): Promise<PageData<DataSource>> {
  return requestPage<DataSource>('/datasources', query, signal);
}

/** 数据源详情（password 为占位串） */
export function getDataSource(id: string, signal?: AbortSignal): Promise<DataSource> {
  return request<DataSource>(`/datasources/${encodeURIComponent(id)}`, { method: 'GET', signal });
}

/**
 * 新建数据源：后端先执行只读连接测试，通过才保存（02 文档 §13.2、04 文档 §3.3 硬性门禁）。
 * 失败返回 2001 中文 message，整条不落库。
 */
export function createDataSource(input: DataSourceInput): Promise<DataSourceCreateResult> {
  return request<DataSourceCreateResult>('/datasources', { method: 'POST', body: input });
}

/** 编辑数据源：含凭据变更时后端强制重新连接测试 */
export function updateDataSource(
  id: string,
  input: Partial<DataSourceInput>,
): Promise<{ id: string }> {
  return request<{ id: string }>(`/datasources/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: input,
  });
}

/** 删除数据源（软删；运行中有任务时后端拒绝并回中文提示） */
export function deleteDataSource(id: string): Promise<Record<string, never>> {
  return request<Record<string, never>>(`/datasources/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

/** 只读探针重测：不改动配置 */
export function testDataSource(id: string): Promise<ConnectionTestResult> {
  return request<ConnectionTestResult>(`/datasources/${encodeURIComponent(id)}/test`, {
    method: 'POST',
  });
}

/** 元数据采集/读取（可强制刷新） */
export function getDataSourceMetadata(
  id: string,
  refresh = false,
  signal?: AbortSignal,
): Promise<DataSourceMetadata> {
  return request<DataSourceMetadata>(`/datasources/${encodeURIComponent(id)}/metadata`, {
    method: 'GET',
    query: refresh ? { refresh: 'true' } : undefined,
    signal,
  });
}

/**
 * 批量导入：逐条连接测试 + 逐条独立事务，部分成功。
 *
 * 两种后端口径（docs/06 §6.3、§6.4；internal/interfaces/http/import_handler.go）：
 * - code=0 / HTTP 200：部分成功或全部成功，data = {total,succeeded,failed,errors}
 * - code=2001 / HTTP 422：**整批全部失败**，零写入，但 data 仍带同一结构
 *   —— 逐行 errors 必须透出到界面，不能只剩一句「导入失败」。
 *   该分支以 ImportAllFailedError 抛出（继承 ApiError，保留 code 与中文 message），
 *   同时在 error.result 上给出结构化的 {total,succeeded,failed,errors}。
 */
export async function importDataSources(file: File): Promise<ImportResult> {
  const formData = new FormData();
  formData.append('file', file);
  try {
    const envelope = await requestEnvelope<Partial<ImportResult>>('/datasources/import', {
      method: 'POST',
      formData,
    });
    return normalizeImportResult(envelope.data);
  } catch (error) {
    if (error instanceof ApiError && error.code === ApiErrorCode.CONNECTION_TEST_FAILED) {
      // 整批失败：后端 data 仍带 {total,succeeded,failed,errors}
      throw new ImportAllFailedError(normalizeImportResult(error.data), error.message, error.httpStatus);
    }
    throw error;
  }
}

/** 整批导入失败（后端 code=2001）：保留逐行明细，供界面展开回显 */
export class ImportAllFailedError extends ApiError {
  readonly result: ImportResult;

  constructor(result: ImportResult, message: string, httpStatus: number) {
    super(ApiErrorCode.CONNECTION_TEST_FAILED, message, httpStatus, result);
    this.name = 'ImportAllFailedError';
    this.result = result;
  }
}

/** 归一化导入结果：兼容 {row,message} 与 {index,name,error} 两种逐行字段写法 */
function normalizeImportResult(data: unknown): ImportResult {
  const raw = (data ?? {}) as Partial<ImportResult> & {
    errors?: unknown;
  };
  const list = Array.isArray(raw.errors) ? raw.errors : [];
  const errors: ImportErrorRow[] = list.map((item, index) => {
    const row = item as Record<string, unknown>;
    const rowNo = toPositiveInt(row.row ?? row.index ?? row.line) ?? index + 1;
    const message =
      toText(row.message ?? row.error ?? row.reason) ?? '该行未通过校验，未入库';
    const name = toText(row.name ?? row.dataSourceName);
    return name ? { row: rowNo, message, name } : { row: rowNo, message };
  });
  const total = toPositiveInt(raw.total) ?? errors.length;
  const succeeded = toPositiveInt(raw.succeeded) ?? 0;
  const failed = toPositiveInt(raw.failed) ?? errors.length;
  return { total, succeeded, failed, errors };
}

function toPositiveInt(value: unknown): number | null {
  const num = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(num) && num >= 0 ? Math.trunc(num) : null;
}

function toText(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

/** 模板下载（excel / yaml），返回文件流 */
export function downloadImportTemplate(format: 'excel' | 'yaml'): Promise<Blob> {
  return request<Blob>('/datasources/import/template', {
    method: 'GET',
    query: { format },
    raw: true,
  });
}
