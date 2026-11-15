/**
 * dbdlp 控制台 · 业务格式化与语义映射
 *
 * 统一承载：风险等级六级→UI 四级收敛（§4.4）、任务六态（§4.5）、
 * 复核状态、存储形态、置信度等中文映射与展示格式。
 */

import type {
  Confidence,
  ReviewConclusion,
  ReviewStatus,
  RiskLevel,
  StorageForm,
  TaskStatus,
} from '@/types/api';

/** UI 风险四级（后端六值收敛，§4.4） */
export type UiRisk = 'high' | 'medium' | 'low' | 'pending';

/** 后端六值 → UI 四级 */
const RISK_TO_UI: Record<RiskLevel, UiRisk> = {
  high: 'high',
  medium_high: 'medium',
  medium: 'medium',
  medium_low: 'medium',
  low: 'low',
  pending_review: 'pending',
};

/** UI 四级中文标签 */
export const UI_RISK_LABEL: Record<UiRisk, string> = {
  high: '高',
  medium: '中',
  low: '低',
  pending: '待复核',
};

/** 风险等级权重（排序用，禁止字符串序，§4.3） */
export const UI_RISK_WEIGHT: Record<UiRisk, number> = {
  high: 4,
  medium: 3,
  low: 2,
  pending: 1,
};

/** 原始枚举中文标签（保留可追溯性） */
export const RAW_RISK_LABEL: Record<RiskLevel, string> = {
  high: '高风险',
  medium_high: '中高风险',
  medium: '中风险',
  medium_low: '中低风险',
  low: '低风险',
  pending_review: '待复核',
};

/** 风险等级 → UI 四级 */
export function toUiRisk(level: RiskLevel | string): UiRisk {
  return RISK_TO_UI[level as RiskLevel] ?? 'pending';
}

/** 敏感类型中文标签（含语义类 ADR-004；未知枚举原样回退） */
export const SENSITIVE_TYPE_LABEL: Record<string, string> = {
  MOBILE_PHONE: '手机号',
  ID_CARD: '身份证',
  BANK_CARD: '银行卡',
  SOCIAL_CREDIT_CODE: '统一社会信用代码',
  EMAIL: '邮箱',
  IPV4: 'IPv4',
  IPV6: 'IPv6',
  PASSPORT: '护照',
  PLATE: '车牌',
  POSTCODE: '邮编',
  FIXED_PHONE: '固话',
  MAC: 'MAC 地址',
  DOMAIN: '域名',
  NAME: '人名',
  COMPANY: '公司名',
  ADDRESS: '地址',
  EMPLOYEE_NO: '内部工号',
  CONTRACT_NO: '合同号',
  CUSTOMER_NO: '客户编号',
  DATE: '日期',
};

export function sensitiveTypeLabel(t: string): string {
  return SENSITIVE_TYPE_LABEL[t] ?? t;
}

/** 语义类类型集合（ADR-004：无结构化校验概念，Verify 列中性展示） */
export const SEMANTIC_TYPES = ['NAME', 'COMPANY', 'ADDRESS'];

export function isSemanticType(t: string): boolean {
  return SEMANTIC_TYPES.includes(t);
}

/** 行级判定：该字段的全部敏感类型均为语义类 → Verify 列与复核引导走中性文案 */
export function isSemanticFinding(types: string[] | undefined | null): boolean {
  return Boolean(types && types.length > 0 && types.every(isSemanticType));
}

/** 置信度权重 */
export const CONFIDENCE_WEIGHT: Record<Confidence, number> = {
  high: 3,
  medium: 2,
  low: 1,
};

export const CONFIDENCE_LABEL: Record<Confidence, string> = {
  high: '高',
  medium: '中',
  low: '低',
};

export const STORAGE_FORM_LABEL: Record<StorageForm, string> = {
  plaintext: '明文',
  encrypted: '透明加密',
  hash: '哈希',
  masked: '掩码',
};

export const REVIEW_STATUS_LABEL: Record<ReviewStatus, string> = {
  pending: '待复核',
  confirmed: '确认敏感',
  false_positive: '误报',
  ignored: '已忽略',
};

export const REVIEW_CONCLUSION_LABEL: Record<ReviewConclusion, string> = {
  pending: '待确认',
  confirmed: '确认敏感',
  false_positive: '误报',
  ignored: '忽略',
};

/** 任务六态中文标签（§4.5；键为后端 port.TaskStatus 枚举值） */
export const TASK_STATUS_LABEL: Record<TaskStatus, string> = {
  pending: '待执行',
  running: '运行中',
  paused: '已暂停',
  done: '成功',
  partial: '部分失败',
  failed: '失败',
};

/** 任务状态是否终态（与后端 port.TaskStatus 对齐，轮询据此停止） */
export function isTerminalStatus(status: TaskStatus | string): boolean {
  return ['done', 'failed', 'partial'].includes(status);
}

/** 格式化本地时间 */
export function formatDateTime(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** 格式化日期 */
export function formatDate(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** 相对时间（用于「最后扫描」Tooltip） */
export function formatRelative(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const diff = Date.now() - date.getTime();
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diff < minute) return '刚刚';
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`;
  return formatDate(value);
}

/** 百分比展示（保留一位小数） */
export function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '0.0%';
  const ratio = value > 1 ? value : value * 100;
  return `${ratio.toFixed(1)}%`;
}

/** 数值展示（千分位） */
export function formatNumber(value?: number): string {
  if (value === undefined || value === null || !Number.isFinite(value)) return '—';
  return value.toLocaleString('zh-CN');
}

/** 文件大小展示 */
export function formatBytes(bytes?: number): string {
  if (bytes === undefined || !Number.isFinite(bytes)) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** 连接状态中文 */
export const CONNECTION_STATE_LABEL: Record<string, string> = {
  ok: '连接正常',
  failed: '连接异常',
  unknown: '未验证',
  testing: '验证中',
};
