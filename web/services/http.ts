/**
 * dbdlp 控制台 · 统一 API 调用封装
 *
 * 契约（02 文档 §13.1）：
 * - 基址 /api/v1，相对路径；静态导出后由 Go 同源托管，无独立 Node 服务、无跨域配置
 * - 统一响应 { code, data, message }，code=0 成功，非 0 抛 ApiError，message 一律中文
 * - 除 POST /api/v1/auth/login 外全部需 Authorization: Bearer <token>
 * - token 只放 sessionStorage 或内存，不落 localStorage
 */

import { ApiErrorCode, type ApiErrorCodeValue, type ApiResponse, type PageData } from '@/types/api';

const BASE = '/api/v1';
const TOKEN_KEY = 'dbdlp.session.token';

/** 后端返回的业务错误（message 已是中文，直接回显） */
export class ApiError extends Error {
  readonly code: ApiErrorCodeValue;
  readonly httpStatus: number;
  /**
   * 错误响应体中的原始 data。
   * 部分错误码在失败时仍带结构化明细（如批量导入 code=2001 的 {total,succeeded,failed,errors}），
   * 此处原样保留，供上层透出逐行错误，不得在 service 层丢弃。
   */
  readonly data: unknown;

  constructor(
    code: ApiErrorCodeValue,
    message: string,
    httpStatus: number,
    data: unknown = null,
  ) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.httpStatus = httpStatus;
    this.data = data ?? null;
  }

  /** 认证失败/会话过期：触发跳转 /login（04 文档 §2.2） */
  get isUnauthorized(): boolean {
    return this.code === ApiErrorCode.UNAUTHORIZED;
  }
}

/** 读取会话令牌（仅 sessionStorage） */
export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.sessionStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

/** 写入会话令牌（仅 sessionStorage；登出时清空） */
export function setToken(token: string): void {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.setItem(TOKEN_KEY, token);
  } catch {
    /* 隐私模式下写入失败不阻断流程，令牌仍保留在内存 store 中 */
  }
}

/** 清除会话令牌 */
export function clearToken(): void {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.removeItem(TOKEN_KEY);
  } catch {
    /* 忽略 */
  }
}

/** 认证失效回调（由 useAuthStore 注册，避免 service 层依赖 store 造成循环引用） */
type UnauthorizedHandler = () => void;
let onUnauthorized: UnauthorizedHandler | null = null;

export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  onUnauthorized = handler;
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE';
  /** 查询参数；值为 undefined/null/空串 的键自动忽略 */
  query?: QueryParams;
  /** JSON 请求体 */
  body?: unknown;
  /** FormData 请求体（批量导入） */
  formData?: FormData;
  /** 跳过 Bearer（仅 login 使用） */
  skipAuth?: boolean;
  /** 接收文件流（模板下载、报告下载） */
  raw?: boolean;
  signal?: AbortSignal;
}

/** 查询参数取值类型 */
export type QueryValue = string | number | boolean | undefined | null;

/** 查询参数集合（允许具名接口参数类型直接传入） */
export type QueryParams = Record<string, QueryValue>;

/** 组装查询串（跳过 undefined / null / 空串） */
function buildQuery(query?: QueryParams): string {
  if (!query) return '';
  const params = new URLSearchParams();
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    params.set(key, String(value));
  });
  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

/** 非 0 code → ApiError；网络异常 → 中文兜底消息 */
function toApiError(
  status: number,
  code: number,
  message: string,
  data: unknown = null,
): ApiError {
  const safeMessage = message || '系统内部错误，请联系管理员';
  return new ApiError(code as ApiErrorCodeValue, safeMessage, status, data);
}

/**
 * 发起请求并解包统一响应。
 * 返回 data 字段；失败时抛出 ApiError（message 为后端中文原文，data 保留原始响应体）。
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const envelope = await requestEnvelope<T>(path, options);
  return envelope.data;
}

/** 统一响应信封（成功分支）：data 为业务数据，message 为后端中文提示 */
export interface ApiEnvelope<T> {
  data: T;
  message: string;
}

/**
 * 发起请求并返回完整信封。
 * - code=0：返回 { data, message }
 * - code≠0：抛出 ApiError，且把原始 data 一并挂到 error.data（供上层透出结构化明细）
 */
export async function requestEnvelope<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiEnvelope<T>> {
  const { method = 'GET', query, body, formData, skipAuth, raw, signal } = options;

  const headers: Record<string, string> = {};
  if (!skipAuth) {
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  // FormData 由浏览器自动设置 boundary，不可手动设置 Content-Type
  if (body !== undefined && !formData) headers['Content-Type'] = 'application/json';

  let response: Response;
  try {
    response = await fetch(`${BASE}${path}${buildQuery(query)}`, {
      method,
      headers,
      body: formData ?? (body !== undefined ? JSON.stringify(body) : undefined),
      signal,
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new ApiError(
      ApiErrorCode.INTERNAL_ERROR,
      '无法连接到服务端，请检查网络或后端服务状态',
      0,
    );
  }

  // 文件流：模板下载 / 报告下载
  if (raw) {
    if (!response.ok) {
      throw new ApiError(
        ApiErrorCode.INTERNAL_ERROR,
        '下载失败，请稍后重试',
        response.status,
      );
    }
    return { data: (await response.blob()) as T, message: '' };
  }

  let payload: ApiResponse<T>;
  try {
    payload = (await response.json()) as ApiResponse<T>;
  } catch {
    throw new ApiError(
      ApiErrorCode.INTERNAL_ERROR,
      '服务端返回了无法解析的数据，请联系管理员',
      response.status,
    );
  }

  if (!response.ok || payload.code !== ApiErrorCode.OK) {
    // data 一并保留：错误响应体可能带结构化明细（如批量导入 2001 的逐行 errors）
    const error = toApiError(
      response.status,
      payload.code ?? ApiErrorCode.INTERNAL_ERROR,
      payload.message,
      payload.data,
    );
    if (error.isUnauthorized) {
      clearToken();
      onUnauthorized?.();
    }
    throw error;
  }

  return { data: payload.data, message: payload.message ?? '' };
}

/** GET 分页数据（服务端分页，参数 page/limit） */
export async function requestPage<T>(
  path: string,
  query?: RequestOptions['query'],
  signal?: AbortSignal,
): Promise<PageData<T>> {
  return request<PageData<T>>(path, { method: 'GET', query, signal });
}
