/**
 * dbdlp 控制台 · 认证服务（02 文档 §13.2 auth 三端点）
 */

import { request } from './http';
import type { CurrentUser, LoginRequest, LoginResult } from '@/types/api';

/** 登录：唯一无需 Bearer 的端点 */
export function login(input: LoginRequest): Promise<LoginResult> {
  return request<LoginResult>('/auth/login', {
    method: 'POST',
    body: input,
    skipAuth: true,
  });
}

/** 当前登录态 */
export function fetchCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
  return request<CurrentUser>('/auth/me', { method: 'GET', signal });
}

/** 登出：删除服务端会话 */
export function logout(): Promise<Record<string, never>> {
  return request<Record<string, never>>('/auth/logout', { method: 'POST' });
}
