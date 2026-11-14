/**
 * dbdlp 控制台 · 后端接口契约类型
 *
 * 严格对齐 02-架构设计文档.md §13.1/§13.2/§13.3：
 * - 统一响应 `{ code, data, message }`，code=0 成功，message 一律中文
 * - 分页 data 固定 `{ items, total, page, limit, hasMore }`，分页参数用 page/limit（不用 pageSize）
 * - 除 POST /api/v1/auth/login 外全部需 Authorization: Bearer <token>
 * - 数据源响应中 password 恒为占位串，前端不得接触明文凭据
 *
 * 本文件仅描述契约，不含任何业务逻辑。
 */

/** 统一响应包裹 */
export interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

/** 分页数据体 */
export interface PageData<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
}

/** 后端错误码（02 文档 §13.3） */
export const ApiErrorCode = {
  OK: 0,
  PARAM_INVALID: 1001,
  UNAUTHORIZED: 1002,
  FORBIDDEN: 1003,
  NOT_FOUND: 1004,
  CONNECTION_TEST_FAILED: 2001,
  RULE_LOAD_FAILED: 2002,
  SECRET_UNAVAILABLE: 2003,
  TASK_STATE_CONFLICT: 3001,
  STORAGE_WRITE_FAILED: 3002,
  INTERNAL_ERROR: 5000,
} as const;

export type ApiErrorCodeValue = (typeof ApiErrorCode)[keyof typeof ApiErrorCode];

/* ------------------------------------------------------------------ 认证 */

/** POST /api/v1/auth/login 请求 */
export interface LoginRequest {
  username: string;
  password: string;
}

/** POST /api/v1/auth/login 响应 data */
export interface LoginResult {
  token: string;
  expiresAt: string;
  displayName: string;
}

/** GET /api/v1/auth/me 响应 data */
export interface CurrentUser {
  userId: string;
  username: string;
  role: string;
}

/* -------------------------------------------------------------- 数据源 */

/** 数据源方言（首个实现为达梦 dm） */
export type Dialect = 'dm' | 'mysql' | 'oracle' | 'postgresql' | 'kingbase';

/** 连接状态 */
export type ConnectionState = 'ok' | 'failed' | 'unknown' | 'testing';

/** 数据源实体（响应；password 恒为占位串） */
export interface DataSource {
  id: string;
  name: string;
  dialect: Dialect;
  /** 响应中已按 §4.6 脱敏 */
  host: string;
  port: number;
  serviceName: string;
  username: string;
  /** 恒为 "******"，前端不得回显 */
  password: string;
  includeSchemas: string[];
  excludeTables: string[];
  excludeColumns: string[];
  connectionState: ConnectionState;
  lastTestAt?: string;
  lastScanAt?: string;
}

/** POST /api/v1/datasources 请求体 */
export interface DataSourceInput {
  name: string;
  dialect: Dialect;
  host: string;
  port: number;
  serviceName: string;
  username: string;
  password: string;
  includeSchemas?: string[];
  excludeTables?: string[];
  excludeColumns?: string[];
}

/** POST /api/v1/datasources 响应 data */
export interface DataSourceCreateResult {
  id: string;
  lastTestOk: boolean;
}

/** POST /api/v1/datasources/{id}/test 响应 data */
export interface ConnectionTestResult {
  ok: boolean;
  latencyMs: number;
  message: string;
}

/** GET /api/v1/datasources/{id}/metadata 响应 data */
export interface DataSourceMetadata {
  schemas: string[];
  tables: string[];
  columnCount: number;
}

/* ------------------------------------------------------------ 批量导入 */

/** POST /api/v1/datasources/import 响应 data（逐条门禁 + 部分成功） */
export interface ImportResult {
  total: number;
  succeeded: number;
  failed: number;
  errors: ImportErrorRow[];
}

/**
 * 导入失败行（中文 message）
 * 后端当前回 {row, message}；row/index/line 与 message/error/reason 均做兼容归一。
 */
export interface ImportErrorRow {
  /** 行号（导入文件中的数据行号，不含表头） */
  row: number;
  /** 失败原因（中文，含分类 + 字段 + 建议） */
  message: string;
  /** 实例标识（后端若返回则展示，缺省为空） */
  name?: string;
}

/* -------------------------------------------------------------- 任务 */

/**
 * 任务六态：严格对齐后端 `port.TaskStatus`（§4.5 描述的是中文标签与
 * `--state-idle-*` / `--state-success-*` 令牌组，不是接口枚举值）。
 * 待执行 = pending，成功 = done；终态为 done / partial / failed。
 */
export type TaskStatus =
  | 'pending'
  | 'running'
  | 'paused'
  | 'done'
  | 'partial'
  | 'failed';

/** 扫描任务 */
export interface ScanTask {
  id: string;
  dataSourceId: string;
  dataSourceName?: string;
  status: TaskStatus;
  ruleVersion?: string;
  sampleRows?: number;
  batchSize?: number;
  concurrency?: number;
  lowPeakWindow?: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  summary?: TaskSummary;
}

/** 任务摘要 */
export interface TaskSummary {
  totalTables: number;
  doneTables: number;
  failedTables: number;
  skippedTables: number;
  totalColumns: number;
  hitColumns: number;
  failedObjects?: FailedObject[];
}

/** 失败对象（可重试）；id 为后端断点键「模式/表」，可直接作为 retry 的 targetIds */
export interface FailedObject {
  id: string;
  schemaName: string;
  tableName: string;
  reason: string;
}

/** GET /api/v1/tasks/{id}/progress 响应 data */
export interface TaskProgress {
  status: TaskStatus;
  doneTables: number;
  totalTables: number;
  doneColumns: number;
  totalColumns: number;
  failedCount: number;
}

/** POST /api/v1/tasks 请求体 */
export interface TaskCreateInput {
  dataSourceId: string;
  scope?: string;
  sampleRows?: number;
  batchSize?: number;
  concurrency?: number;
  lowPeakWindow?: string;
}

/* -------------------------------------------------------------- 台账 */

/** 后端六值风险枚举（04 文档 §4.4） */
export type RiskLevel =
  | 'high'
  | 'medium_high'
  | 'medium'
  | 'medium_low'
  | 'low'
  | 'pending_review';

/** 复核状态 */
export type ReviewStatus = 'pending' | 'confirmed' | 'false_positive' | 'ignored';

/** 存储形态 */
export type StorageForm = 'plaintext' | 'encrypted' | 'hash' | 'masked';

/** 置信度 */
export type Confidence = 'high' | 'medium' | 'low';

/** 字段级命中台账（18 列，§4.3） */
export interface ColumnFinding {
  id: string;
  instanceId: string;
  schemaName: string;
  tableName: string;
  columnName: string;
  columnComment: string;
  dataType: string;
  sensitiveTypes: string[];
  hitRatio: number;
  sampleHit: number;
  sampleTotal: number;
  verifyPassed: boolean;
  confidence: Confidence;
  storageForm: StorageForm;
  riskLevel: RiskLevel;
  reviewStatus: ReviewStatus;
  /** 仅掩码值，前端只消费此字段 */
  sampleMasked: string;
  updatedAt: string;
}

/** 命中明细（仅掩码值） */
export interface HitRecord {
  id: string;
  findingId: string;
  /** 仅掩码值 */
  sampleMasked: string;
  ruleId: string;
  ruleName: string;
  matchedAt: string;
}

/** 复核结论 */
export type ReviewConclusion = 'confirmed' | 'false_positive' | 'ignored' | 'pending';

/** POST /api/v1/reviews 请求体 */
export interface ReviewInput {
  findingId: string;
  conclusion: ReviewConclusion;
  comment: string;
}

/** 复核记录 */
export interface ReviewRecord {
  id: string;
  findingId: string;
  conclusion: ReviewConclusion;
  comment: string;
  reviewer: string;
  reviewedAt: string;
}

/* -------------------------------------------------------------- 规则 */

/** 规则条目（只读查看，P11） */
export interface RuleItem {
  id: string;
  name: string;
  sensitiveType: string;
  pattern: string;
  threshold?: number;
  enabled: boolean;
  validateStatus: 'valid' | 'invalid' | 'unknown';
  description?: string;
}

/** GET /api/v1/rules 响应 data */
export interface RuleListResult {
  version: string;
  rules: RuleItem[];
}

/** 规则版本快照 */
export interface RuleSnapshot {
  version: string;
  createdAt: string;
  ruleCount: number;
}

/* -------------------------------------------------------------- 报告 */

/** 报告格式 */
export type ReportFormat = 'excel' | 'html' | 'json';

/** 报告产物 */
export interface ReportArtifact {
  id: string;
  taskId: string;
  taskName?: string;
  format: ReportFormat;
  status: TaskStatus;
  filePath?: string;
  fileSize?: number;
  createdAt: string;
}

/** POST /api/v1/reports 响应 data */
export interface ReportGenerateResult {
  format: ReportFormat;
  artifactId: string;
  filePath: string;
}

/* -------------------------------------------------------------- 概览 */

/** GET /api/v1/dashboard/summary 响应 data（ECharts 数据源） */
export interface DashboardSummary {
  riskCounts: Record<string, number>;
  typeCounts: Record<string, number>;
  schemaCounts: Record<string, number>;
  tableCount: number;
  columnCount: number;
  hitColumnCount: number;
}

/* -------------------------------------------------------------- 审计 */

/** 审计日志（内容已脱敏） */
export interface AuditLog {
  id: string;
  actor: string;
  action: string;
  target: string;
  /** 已脱敏 */
  ip: string;
  detail?: string;
  createdAt: string;
}

/* -------------------------------------------------------------- 配置 */

/** GET /api/v1/config 响应 data（只读回显，不含密钥与 DSN） */
export interface AppConfig {
  logLevel: string;
  sampleRows: number;
  storageDriver: string;
  cryptoAlgorithm: string;
  retentionDays: number;
}
