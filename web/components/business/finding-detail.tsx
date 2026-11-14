'use client';

/**
 * dbdlp 控制台 · P08 字段明细与复核业务组件
 *
 * 包含：*FindingDetailHeader、*FindingMetaGrid、*HitDetailTable、
 *       *ReviewForm、*ReviewHistory
 *
 * 规范（04 文档 §2.3）：
 * - 桌面端以右侧抽屉呈现并保留可分享深链（本实现用独立路由承载抽屉内容）
 * - 复核结论与原识别结论分开留存，不覆盖原始结论
 * - 命中明细只渲染掩码值
 */

import * as React from 'react';
import {
  BadgeCheck,
  Braces,
  CircleQuestionMark,
  ClipboardCheck,
  ClipboardList,
  EyeOff,
  Info,
  Save,
  ShieldCheck,
  ShieldX,
} from 'lucide-react';
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Field, Textarea } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { formatDateTime, RAW_RISK_LABEL, REVIEW_CONCLUSION_LABEL, STORAGE_FORM_LABEL, CONFIDENCE_LABEL } from '@/lib/format';
import { RiskBadge } from '@/components/business/findings';
import type {
  ColumnFinding,
  Confidence,
  HitRecord,
  ReviewConclusion,
  ReviewRecord,
  StorageForm,
} from '@/types/api';

/* -------------------------------------------------------- *FindingDetailHeader */

/** 字段明细头部摘要 */
export function FindingDetailHeader({ finding }: { finding: ColumnFinding }) {
  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <CardTitle className="font-[var(--font-mono)]">
            {finding.schemaName}.{finding.tableName}.{finding.columnName}
          </CardTitle>
          <CardDescription>
            {finding.instanceId} · {finding.dataType}
            {finding.columnComment ? ` · ${finding.columnComment}` : ''}
          </CardDescription>
        </div>
        <div className="flex flex-wrap items-center gap-[var(--space-2)]">
          <RiskBadge level={finding.riskLevel} />
          <Badge tone="neutral">{RAW_RISK_LABEL[finding.riskLevel] ?? '待复核'}</Badge>
        </div>
      </CardHeader>
    </Card>
  );
}

/* ----------------------------------------------------------- *FindingMetaGrid */

/** 基础信息网格（保留原始枚举值，可追溯） */
export function FindingMetaGrid({ finding }: { finding: ColumnFinding }) {
  const rows = [
    { label: '实例', value: finding.instanceId, mono: true },
    { label: '模式', value: finding.schemaName, mono: true },
    { label: '表名', value: finding.tableName, mono: true },
    { label: '字段名', value: finding.columnName, mono: true },
    { label: '字段类型', value: finding.dataType, mono: true },
    { label: '敏感类型', value: finding.sensitiveTypes.join('、') || '—' },
    {
      label: '命中率',
      value: `${Math.round((finding.hitRatio > 1 ? finding.hitRatio : finding.hitRatio * 100) * 10) / 10}%`,
      mono: true,
    },
    { label: '命中 / 样本', value: `${finding.sampleHit} / ${finding.sampleTotal}`, mono: true },
    {
      label: 'Verify',
      value: finding.verifyPassed ? '通过' : '未通过',
    },
    { label: '置信度', value: CONFIDENCE_LABEL[finding.confidence as Confidence] ?? '低' },
    { label: '存储形态', value: STORAGE_FORM_LABEL[finding.storageForm as StorageForm] ?? '—' },
    { label: '最后扫描', value: formatDateTime(finding.updatedAt), mono: true },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <Info size={20} aria-hidden="true" />
          基础信息
        </CardTitle>
        <CardDescription>原始风险枚举与识别结论一并保留，便于追溯</CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-[var(--space-3)] sm:grid-cols-2 lg:grid-cols-3">
          {rows.map((row) => (
            <div key={row.label} className="flex flex-col gap-[var(--space-1)]">
              <dt className="text-[var(--text-xs)] text-[var(--muted)]">{row.label}</dt>
              <dd
                className={
                  'text-[var(--text-sm)] text-[var(--fg)]' +
                  (row.mono ? ' font-[var(--font-mono)]' : '')
                }
              >
                {row.value}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

/* ----------------------------------------------------------- *HitDetailTable */

/** 命中明细表（仅掩码值） */
export function HitDetailTable({ hits }: { hits: HitRecord[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ClipboardList size={20} aria-hidden="true" />
          命中明细
        </CardTitle>
        <CardDescription className="flex items-center gap-[var(--space-1)]">
          <EyeOff size={12} aria-hidden="true" />
          全部为掩码值，平台不展示完整明文
        </CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[200px]">掩码样例</TableHead>
              <TableHead>规则</TableHead>
              <TableHead className="w-[140px]">命中时间</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {hits.length === 0 ? (
              <TableEmptyRow colSpan={3} message="该字段暂无命中记录" />
            ) : (
              hits.map((hit) => (
                <TableRow key={hit.id}>
                  <TableCell>
                    <span className="rounded-[var(--radius-sm)] border border-[var(--sample-border)] bg-[var(--sample-bg)] px-[var(--space-1)] font-[var(--font-mono)] text-[var(--text-xs)] text-[var(--sample-fg)]">
                      {hit.sampleMasked}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="inline-flex items-center gap-[var(--space-1)] text-[var(--text-xs)]">
                      <ShieldCheck size={12} aria-hidden="true" className="text-[var(--muted)]" />
                      {hit.ruleName}
                      <span className="font-[var(--font-mono)] text-[var(--meta)]">{hit.ruleId}</span>
                    </span>
                  </TableCell>
                  <TableCell className="text-[var(--muted)]">{formatDateTime(hit.matchedAt)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/* ---------------------------------------------------------------- *ReviewForm */

/** 复核表单：结论 + 说明，提交后不覆盖原始识别结论 */
export function ReviewForm({
  conclusion,
  comment,
  submitting,
  error,
  onConclusionChange,
  onCommentChange,
  onSubmit,
}: {
  conclusion: ReviewConclusion;
  comment: string;
  submitting: boolean;
  error: string | null;
  onConclusionChange: (value: ReviewConclusion) => void;
  onCommentChange: (value: string) => void;
  onSubmit: () => void;
}) {
  const options: { value: ReviewConclusion; label: string; Icon: typeof BadgeCheck; hint: string }[] = [
    {
      value: 'confirmed',
      label: '确认敏感',
      Icon: BadgeCheck,
      hint: '该字段确为敏感数据，保持原识别结论',
    },
    {
      value: 'false_positive',
      label: '误报',
      Icon: ShieldX,
      hint: '规则命中但业务上非敏感，记录误报结论',
    },
    {
      value: 'ignored',
      label: '忽略',
      Icon: CircleQuestionMark,
      hint: '暂不处理，记录忽略原因',
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ClipboardCheck size={20} aria-hidden="true" />
          提交复核
        </CardTitle>
        <CardDescription>复核结论单独留存，不覆盖识别引擎给出的原始结论</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-[var(--space-4)]">
        <div
          role="radiogroup"
          aria-label="复核结论"
          className="flex flex-col gap-[var(--space-2)]"
        >
          {options.map((option) => (
            <label
              key={option.value}
              className="flex cursor-pointer items-start gap-[var(--space-2)] rounded-[var(--radius-sm)] border border-[var(--border-soft)] px-[var(--space-3)] py-[var(--space-2)] transition-colors duration-[var(--motion-fast)] hover:bg-[var(--surface-hover)]"
            >
              <input
                type="radio"
                name="review-conclusion"
                value={option.value}
                checked={conclusion === option.value}
                onChange={() => onConclusionChange(option.value)}
                className="mt-[3px] h-[16px] w-[16px] accent-[var(--accent)] focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]"
              />
              <span className="flex flex-col gap-[2px]">
                <span className="inline-flex items-center gap-[var(--space-1)] text-[var(--text-sm)] font-[var(--weight-emphasize)] text-[var(--fg)]">
                  <option.Icon size={14} aria-hidden="true" />
                  {option.label}
                </span>
                <span className="text-[var(--text-xs)] text-[var(--muted)]">{option.hint}</span>
              </span>
            </label>
          ))}
        </div>

        <Field
          label="复核说明"
          htmlFor="review-comment"
          required
          hint="说明依据与处置方式，便于后续追溯"
          error={error ?? undefined}
        >
          <Textarea
            id="review-comment"
            value={comment}
            placeholder="例如：字段用于工号编码，不含个人敏感信息，判定为误报"
            onChange={(e) => onCommentChange(e.target.value)}
          />
        </Field>

        <div>
          <Button onClick={onSubmit} loading={submitting}>
            <Save size={16} aria-hidden="true" />
            保存复核结论
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------- *ReviewHistory */

/** 复核历史记录 */
export function ReviewHistory({ reviews }: { reviews: ReviewRecord[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-[var(--space-2)]">
          <ClipboardList size={20} aria-hidden="true" />
          复核记录
        </CardTitle>
        <CardDescription>历次复核结论与说明，按时间倒序</CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[120px]">结论</TableHead>
              <TableHead>说明</TableHead>
              <TableHead className="w-[120px]">复核人</TableHead>
              <TableHead className="w-[150px]">时间</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {reviews.length === 0 ? (
              <TableEmptyRow colSpan={4} message="该字段还没有复核记录" />
            ) : (
              reviews.map((review) => (
                <TableRow key={review.id}>
                  <TableCell>
                    <Badge
                      tone={
                        review.conclusion === 'confirmed'
                          ? 'success'
                          : review.conclusion === 'false_positive'
                            ? 'neutral'
                            : 'warning'
                      }
                    >
                      {REVIEW_CONCLUSION_LABEL[review.conclusion] ?? '待确认'}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-[var(--fg-2)]">{review.comment}</TableCell>
                  <TableCell className="font-[var(--font-mono)] text-[var(--text-xs)]">
                    {review.reviewer}
                  </TableCell>
                  <TableCell className="text-[var(--muted)]">{formatDateTime(review.reviewedAt)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/* -------------------------------------------------------------- 规则来源块 */

/** 命中来源说明 */
export function FindingSourceHint() {
  return (
    <p className="flex items-start gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
      <Braces size={14} aria-hidden="true" className="mt-[2px] shrink-0" />
      命中规则来自当前生效的规则版本，可在规则管理页查看完整清单。
    </p>
  );
}
