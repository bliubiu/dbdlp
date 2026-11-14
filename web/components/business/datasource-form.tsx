'use client';

/**
 * dbdlp 控制台 · P03 数据源新建/编辑业务组件
 *
 * 包含：*DataSourceForm、*ConnectionTestPanel、*CredentialField、*ScopeEditor、*FormSummary
 *
 * 硬性门禁：连接测试通过才保存（后端原子保证，§3.3）。
 * 凭据：password 只输入、不回显、不缓存、不写日志（§3.7）。
 */

import * as React from 'react';
import {
  CircleCheck,
  CircleX,
  Eye,
  EyeOff,
  Info,
  KeyRound,
  LoaderCircle,
  PlugZap,
  Save,
  TriangleExclamationPoint,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Separator } from '@/components/ui/primitives';
import { Field, Input, Label, Textarea } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import type { TestState } from '@/stores/useDataSourceFormStore';

/* -------------------------------------------------- *ConnectionTestPanel */

/**
 * 连接测试状态机面板（§3.3）：
 * idle → testing → success | fail
 */
export function ConnectionTestPanel({
  state,
  message,
  latencyMs,
}: {
  state: TestState;
  message: string;
  latencyMs: number;
}) {
  return (
    <div
      className={cn(
        'flex items-start gap-[var(--space-2)] rounded-[var(--radius-md)] border px-[var(--space-3)] py-[var(--space-2)]',
        state === 'success'
          ? 'border-[var(--success-border)] bg-[var(--success-bg)] text-[var(--success)]'
          : state === 'fail'
            ? 'border-[var(--danger-border)] bg-[var(--danger-bg)] text-[var(--danger)]'
            : state === 'testing'
              ? 'border-[var(--info-border)] bg-[var(--info-bg)] text-[var(--info)]'
              : 'border-[var(--border-soft)] bg-[var(--surface-warm)] text-[var(--muted)]',
      )}
      role="status"
      aria-live="polite"
    >
      {state === 'success' ? (
        <CircleCheck size={20} aria-hidden="true" className="mt-[2px] shrink-0" />
      ) : state === 'fail' ? (
        <CircleX size={20} aria-hidden="true" className="mt-[2px] shrink-0" />
      ) : state === 'testing' ? (
        <LoaderCircle size={20} aria-hidden="true" className="mt-[2px] shrink-0 animate-spin" />
      ) : (
        <PlugZap size={20} aria-hidden="true" className="mt-[2px] shrink-0" />
      )}
      <div className="flex flex-col gap-[var(--space-1)]">
        <span className="text-[var(--text-sm)] font-[var(--weight-emphasize)]">
          {state === 'success'
            ? '连接成功'
            : state === 'fail'
              ? '连接未通过，配置未保存'
              : state === 'testing'
                ? '正在校验只读连接…'
                : '尚未验证连接'}
        </span>
        <span className="text-[var(--text-xs)] leading-[var(--leading-compact)]">
          {message ||
            (state === 'idle' ? '填写连接信息后可验证，验证通过后才会保存配置。' : '')}
        </span>
        {state === 'success' && latencyMs > 0 ? (
          <span className="font-[var(--font-mono)] text-[var(--text-xs)] tabular-nums">
            响应耗时 {latencyMs} ms
          </span>
        ) : null}
      </div>
    </div>
  );
}

/* ------------------------------------------------------- *CredentialField */

/**
 * 凭据字段（§3.7）：
 * - type=password，只输入不回显
 * - 编辑态显示「已配置，留空表示不修改」（后端返回占位串）
 */
export function CredentialField({
  id,
  value,
  onChange,
  placeholderConfigured = false,
  error,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  /** 编辑态：后端已配置凭据 */
  placeholderConfigured?: boolean;
  error?: string;
}) {
  const [visible, setVisible] = React.useState(false);

  return (
    <Field
      label="只读账号密码"
      htmlFor={id}
      error={error}
      hint="密码只做输入，不回显、不缓存、不写入日志；留空表示不修改"
      required={!placeholderConfigured}
    >
      <div className="relative">
        <KeyRound
          size={16}
          aria-hidden="true"
          className="pointer-events-none absolute left-[var(--space-3)] top-1/2 -translate-y-1/2 text-[var(--meta)]"
        />
        <Input
          id={id}
          type={visible ? 'text' : 'password'}
          value={value}
          autoComplete="new-password"
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholderConfigured ? '已配置，留空表示不修改' : '请输入只读账号密码'}
          aria-invalid={error ? 'true' : undefined}
          aria-describedby={error ? `${id}-error` : undefined}
          className="pl-[var(--space-8)] pr-[var(--space-10)]"
        />
        <button
          type="button"
          onClick={() => setVisible((prev) => !prev)}
          aria-label={visible ? '隐藏密码' : '显示密码'}
          className="absolute right-[var(--space-2)] top-1/2 inline-flex h-[32px] w-[32px] -translate-y-1/2 items-center justify-center rounded-[var(--radius-sm)] text-[var(--muted)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)] hover:text-[var(--fg)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
        >
          {visible ? <EyeOff size={16} aria-hidden="true" /> : <Eye size={16} aria-hidden="true" />}
        </button>
      </div>
    </Field>
  );
}

/* ----------------------------------------------------------- *ScopeEditor */

/** 扫描范围编辑器（纳入模式 / 排除表 / 排除字段） */
export function ScopeEditor({
  includeSchemas,
  onSchemasChange,
  excludeTables,
  onTablesChange,
  excludeColumns,
  onColumnsChange,
}: {
  includeSchemas: string[];
  onSchemasChange: (value: string[]) => void;
  excludeTables: string;
  onTablesChange: (value: string) => void;
  excludeColumns: string;
  onColumnsChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--space-4)]">
      <Field
        label="纳入模式"
        htmlFor="ds-schemas"
        hint="留空表示扫描非系统模式；多个模式用英文逗号分隔"
      >
        <Input
          id="ds-schemas"
          value={includeSchemas.join(',')}
          onChange={(e) =>
            onSchemasChange(
              e.target.value
                .split(',')
                .map((s) => s.trim())
                .filter(Boolean),
            )
          }
          placeholder="例如：SYSDBA,SALES,HR"
        />
      </Field>

      <Field
        label="排除表"
        htmlFor="ds-exclude-tables"
        hint="支持正则语法，每行一条；失败会在保存时近字段回显中文原因"
      >
        <Textarea
          id="ds-exclude-tables"
          value={excludeTables}
          onChange={(e) => onTablesChange(e.target.value)}
          placeholder={'例如：^TMP_.*\n^LOG_.*'}
        />
      </Field>

      <Field
        label="排除字段"
        htmlFor="ds-exclude-columns"
        hint="支持正则语法，每行一条"
      >
        <Textarea
          id="ds-exclude-columns"
          value={excludeColumns}
          onChange={(e) => onColumnsChange(e.target.value)}
          placeholder="例如：^CREATE_TIME$"
        />
      </Field>
    </div>
  );
}

/* ----------------------------------------------------------- *FormSummary */

/** 右侧 sticky 配置摘要 */
export function FormSummary({
  name,
  dialect,
  host,
  port,
  serviceName,
  username,
  includeSchemas,
}: {
  name: string;
  dialect: string;
  host: string;
  port: number;
  serviceName: string;
  username: string;
  includeSchemas: string[];
}) {
  const rows: [string, string][] = [
    ['实例标识', name || '—'],
    ['数据库方言', dialect || '—'],
    ['主机:端口', host ? `${host}:${port}` : '—'],
    ['服务名', serviceName || '—'],
    ['只读账号', username || '—'],
    ['纳入模式', includeSchemas.length > 0 ? includeSchemas.join(', ') : '全部非系统模式'],
  ];

  return (
    <Card className="sticky top-[var(--space-4)]">
      <CardHeader>
        <CardTitle>配置摘要</CardTitle>
        <CardDescription>保存前的配置确认，密码不会出现在摘要中</CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="flex flex-col gap-[var(--space-2)]">
          {rows.map(([label, value]) => (
            <div key={label} className="flex flex-col gap-[var(--space-1)]">
              <dt className="text-[var(--text-xs)] text-[var(--muted)]">{label}</dt>
              <dd className="font-[var(--font-mono)] text-[var(--text-sm)] text-[var(--fg)] break-all">
                {value}
              </dd>
            </div>
          ))}
        </dl>
        <Separator className="my-[var(--space-3)]" />
        <p className="flex items-start gap-[var(--space-2)] text-[var(--text-xs)] leading-[var(--leading-compact)] text-[var(--muted)]">
          <Info size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
          <span>主机与地址在列表与报告中按脱敏策略展示；127.0.0.1 与 localhost 保留完整。</span>
        </p>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------ *DataSourceForm */

/** 步骤指示（3 步表单，每步 ≤4 字段） */
export function StepIndicator({ steps, current }: { steps: string[]; current: number }) {
  return (
    <ol className="flex flex-wrap items-center gap-[var(--space-2)]" aria-label="表单步骤">
      {steps.map((label, index) => {
        const done = index + 1 < current;
        const active = index + 1 === current;
        return (
          <li key={label} className="flex items-center gap-[var(--space-2)]">
            <span
              className={cn(
                'inline-flex h-[24px] w-[24px] items-center justify-center rounded-[var(--radius-pill)] text-[var(--text-xs)] font-[var(--weight-emphasize)]',
                done
                  ? 'bg-[var(--success-bg)] text-[var(--success)]'
                  : active
                    ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
                    : 'bg-[var(--surface-warm)] text-[var(--meta)]',
              )}
              aria-hidden="true"
            >
              {done ? <CircleCheck size={14} /> : index + 1}
            </span>
            <span
              className={cn(
                'text-[var(--text-sm)]',
                active
                  ? 'font-[var(--weight-emphasize)] text-[var(--fg)]'
                  : 'text-[var(--muted)]',
              )}
              aria-current={active ? 'step' : undefined}
            >
              {label}
            </span>
            {index < steps.length - 1 ? (
              <span aria-hidden="true" className="text-[var(--meta)]">
                /
              </span>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}

/** 保存操作栏 */
export function FormActions({
  saving,
  onSave,
  onCancel,
  saveLabel = '测试连接并保存',
}: {
  saving: boolean;
  onSave: () => void;
  onCancel: () => void;
  saveLabel?: string;
}) {
  return (
    <div className="flex items-center justify-end gap-[var(--space-2)]">
      <Button variant="secondary" onClick={onCancel} disabled={saving}>
        取消
      </Button>
      <Button onClick={onSave} loading={saving}>
        {saving ? (
          <>
            <LoaderCircle size={20} aria-hidden="true" className="animate-spin" />
            正在校验连接…
          </>
        ) : (
          <>
            <Save size={20} aria-hidden="true" />
            {saveLabel}
          </>
        )}
      </Button>
    </div>
  );
}

/** 顶部提示条：连接测试门禁说明 */
export function GateNotice() {
  return (
    <div className="flex items-start gap-[var(--space-2)] rounded-[var(--radius-md)] border border-[var(--warn-border)] bg-[var(--warn-bg)] px-[var(--space-3)] py-[var(--space-2)] text-[var(--warn)]">
      <TriangleExclamationPoint size={20} aria-hidden="true" className="mt-[2px] shrink-0" />
      <p className="text-[var(--text-sm)] leading-[var(--leading-compact)]">
        连接测试通过才保存：由后端在写入前执行只读连接测试，测试未通过时整条配置不落库。
      </p>
    </div>
  );
}

/** 只读账号徽章 */
export function ReadOnlyBadge() {
  return (
    <Badge tone="info" className="inline-flex items-center gap-[var(--space-1)]">
      <Info size={12} aria-hidden="true" />
      仅接受只读账号
    </Badge>
  );
}
