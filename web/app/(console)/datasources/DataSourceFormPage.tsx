'use client';

/**
 * dbdlp 控制台 · P03 数据源表单页（新建 / 编辑共用）
 *
 * 3 步表单（每步 ≤4 字段）+ 右侧 sticky 配置摘要 + 底部操作栏。
 * 新建：/datasources/new；编辑：/datasources/[id]/edit
 */

import * as React from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { useDataSourceFormStore } from '@/stores/useDataSourceFormStore';
import { useUIStore } from '@/stores/useUIStore';
import { getDataSource } from '@/services/datasources';
import { Button } from '@/components/ui/button';
import { Alert, Card, CardContent, CardHeader, CardTitle } from '@/components/ui/primitives';
import { Field, Input } from '@/components/ui/input';
import {
  ConnectionTestPanel,
  CredentialField,
  FormActions,
  FormSummary,
  GateNotice,
  ReadOnlyBadge,
  ScopeEditor,
  StepIndicator,
} from '@/components/business/datasource-form';

const STEPS = ['连接信息', '扫描范围', '确认提交'];

export function DataSourceFormPage({ id }: { id?: string }) {
  const router = useRouter();
  const store = useDataSourceFormStore();
  const pushToast = useUIStore((s) => s.pushToast);
  const isEdit = Boolean(id);

  // 编辑态：回填（password 不回填，后端返回占位串）
  React.useEffect(() => {
    if (!id) {
      store.reset();
      return;
    }
    let cancelled = false;
    const load = async () => {
      try {
        const detail = await getDataSource(id);
        if (cancelled) return;
        store.hydrate({
          name: detail.name,
          dialect: detail.dialect,
          host: detail.host,
          port: detail.port,
          serviceName: detail.serviceName,
          username: detail.username,
          password: '',
          includeSchemas: detail.includeSchemas ?? [],
          excludeTables: (detail.excludeTables ?? []).join('\n'),
          excludeColumns: (detail.excludeColumns ?? []).join('\n'),
        });
      } catch {
        if (!cancelled) {
          pushToast('数据源详情加载失败，请返回列表重试', 'error');
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  /** 失焦校验（中文错误近字段回显） */
  const validateStep = (step: number): boolean => {
    const { values, setFieldError, clearErrors } = store;
    clearErrors();
    let valid = true;

    if (step === 1) {
      if (!values.name.trim()) {
        setFieldError('name', '请输入实例标识，长度 2 到 64 位，仅支持字母、数字、下划线与连字符');
        valid = false;
      } else if (!/^[A-Za-z0-9_-]{2,64}$/.test(values.name.trim())) {
        setFieldError('name', '实例标识只能包含字母、数字、下划线与连字符，且长度为 2 到 64 位');
        valid = false;
      }
      if (!values.host.trim()) {
        setFieldError('host', '请输入主机名或 IPv4 地址');
        valid = false;
      }
      if (!Number.isInteger(values.port) || values.port < 1 || values.port > 65535) {
        setFieldError('port', '端口必须是 1 到 65535 之间的整数');
        valid = false;
      }
      if (!values.serviceName.trim()) {
        setFieldError('serviceName', '请输入服务名或库名，长度不超过 128 位');
        valid = false;
      }
      if (!values.username.trim()) {
        setFieldError('username', '请输入只读账号');
        valid = false;
      }
      if (!isEdit && !values.password) {
        setFieldError('password', '请输入只读账号密码');
        valid = false;
      }
    }

    if (valid) {
      store.setStep(Math.min(3, step + 1));
    }
    return valid;
  };

  const handleSave = async () => {
    // 提交前全量校验
    const { values, setFieldError, clearErrors } = store;
    clearErrors();
    const errors: Record<string, string> = {};
    if (!values.name.trim()) errors.name = '请输入实例标识，长度 2 到 64 位';
    if (!values.host.trim()) errors.host = '请输入主机名或 IPv4 地址';
    if (!values.serviceName.trim()) errors.serviceName = '请输入服务名或库名';
    if (!values.username.trim()) errors.username = '请输入只读账号';
    if (!isEdit && !values.password) errors.password = '请输入只读账号密码';

    if (Object.keys(errors).length > 0) {
      Object.entries(errors).forEach(([field, message]) => setFieldError(field, message));
      store.setStep(1);
      // 焦点移到首个错误字段
      const firstField = Object.keys(errors)[0];
      document.getElementById(`ds-${firstField}`)?.focus();
      return;
    }

    const savedId = await store.submit(id);
    if (savedId) {
      pushToast('配置已保存，连接测试通过', 'success');
      router.push('/datasources');
    }
  };

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-2)]">
          <Button variant="ghost" size="sm" asChild className="w-fit">
            <a href="/datasources">
              <ArrowLeft size={16} aria-hidden="true" />
              返回数据源列表
            </a>
          </Button>
          <h1>{isEdit ? '编辑数据源' : '新建数据源'}</h1>
        </div>
        <StepIndicator steps={STEPS} current={store.step} />
      </div>

      <GateNotice />

      {store.submitError ? (
        <Alert tone="danger" title="保存失败">
          {store.submitError}
        </Alert>
      ) : null}

      <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] lg:grid-cols-12">
        {/* 左侧表单 */}
        <div className="flex flex-col gap-[var(--gutter-desktop)] lg:col-span-8">
          {/* 第 1 步：连接信息（≤4 字段 + 凭据） */}
          {store.step === 1 ? (
            <Card>
              <CardHeader>
                <CardTitle>连接信息</CardTitle>
                <p className="text-[var(--text-xs)] text-[var(--muted)]">
                  填写实例标识、主机端口、服务名与只读账号；密码仅做输入
                </p>
              </CardHeader>
              <CardContent className="flex flex-col gap-[var(--space-4)]">
                <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-2">
                  <Field
                    label="实例标识"
                    htmlFor="ds-name"
                    required
                    error={store.errors.name}
                    hint="业务可读名，需全局唯一"
                  >
                    <Input
                      id="ds-name"
                      value={store.values.name}
                      onChange={(e) => store.setValue('name', e.target.value)}
                      placeholder="例如：dameng_prod_main"
                      aria-invalid={store.errors.name ? 'true' : undefined}
                      aria-describedby={store.errors.name ? 'ds-name-error' : undefined}
                    />
                  </Field>

                  <Field label="数据库方言" htmlFor="ds-dialect-field" required error={store.errors.dialect}>
                    <div className="flex items-center gap-[var(--space-2)]">
                      <Input
                        id="ds-dialect-field"
                        value={store.values.dialect}
                        onChange={(e) =>
                          store.setValue('dialect', e.target.value as typeof store.values.dialect)
                        }
                        placeholder="dm"
                        className="font-[var(--font-mono)]"
                      />
                      <ReadOnlyBadge />
                    </div>
                  </Field>

                  <Field label="主机" htmlFor="ds-host" required error={store.errors.host}>
                    <Input
                      id="ds-host"
                      value={store.values.host}
                      onChange={(e) => store.setValue('host', e.target.value)}
                      placeholder="主机名或 IPv4 地址"
                      aria-invalid={store.errors.host ? 'true' : undefined}
                      aria-describedby={store.errors.host ? 'ds-host-error' : undefined}
                    />
                  </Field>

                  <Field label="端口" htmlFor="ds-port" required error={store.errors.port}>
                    <Input
                      id="ds-port"
                      type="number"
                      min={1}
                      max={65535}
                      value={String(store.values.port)}
                      onChange={(e) => store.setValue('port', Number(e.target.value))}
                      className="font-[var(--font-mono)]"
                      aria-invalid={store.errors.port ? 'true' : undefined}
                      aria-describedby={store.errors.port ? 'ds-port-error' : undefined}
                    />
                  </Field>

                  <Field
                    label="服务名 / 库名"
                    htmlFor="ds-serviceName"
                    required
                    error={store.errors.serviceName}
                  >
                    <Input
                      id="ds-serviceName"
                      value={store.values.serviceName}
                      onChange={(e) => store.setValue('serviceName', e.target.value)}
                      placeholder="例如：DAMENG"
                      aria-invalid={store.errors.serviceName ? 'true' : undefined}
                      aria-describedby={store.errors.serviceName ? 'ds-serviceName-error' : undefined}
                    />
                  </Field>

                  <Field label="只读账号" htmlFor="ds-username" required error={store.errors.username}>
                    <Input
                      id="ds-username"
                      value={store.values.username}
                      onChange={(e) => store.setValue('username', e.target.value)}
                      placeholder="仅接受只读权限账号"
                      className="font-[var(--font-mono)]"
                      aria-invalid={store.errors.username ? 'true' : undefined}
                      aria-describedby={store.errors.username ? 'ds-username-error' : undefined}
                    />
                  </Field>
                </div>

                <CredentialField
                  id="ds-password"
                  value={store.values.password}
                  onChange={(value) => store.setValue('password', value)}
                  placeholderConfigured={isEdit}
                  error={store.errors.password}
                />
              </CardContent>
            </Card>
          ) : null}

          {/* 第 2 步：扫描范围 */}
          {store.step === 2 ? (
            <Card>
              <CardHeader>
                <CardTitle>扫描范围</CardTitle>
                <p className="text-[var(--text-xs)] text-[var(--muted)]">
                  留空表示扫描非系统模式；排除规则支持正则语法
                </p>
              </CardHeader>
              <CardContent>
                <ScopeEditor
                  includeSchemas={store.values.includeSchemas}
                  onSchemasChange={(value) => store.setValue('includeSchemas', value)}
                  excludeTables={store.values.excludeTables}
                  onTablesChange={(value) => store.setValue('excludeTables', value)}
                  excludeColumns={store.values.excludeColumns}
                  onColumnsChange={(value) => store.setValue('excludeColumns', value)}
                />
              </CardContent>
            </Card>
          ) : null}

          {/* 第 3 步：确认提交 */}
          {store.step === 3 ? (
            <Card>
              <CardHeader>
                <CardTitle>确认提交</CardTitle>
                <p className="text-[var(--text-xs)] text-[var(--muted)]">
                  提交后由后端执行只读连接测试，测试未通过时整条配置不落库
                </p>
              </CardHeader>
              <CardContent className="flex flex-col gap-[var(--space-4)]">
                <ConnectionTestPanel
                  state={store.testState}
                  message={store.testMessage}
                  latencyMs={store.testLatencyMs}
                />
                <p className="text-[var(--text-sm)] text-[var(--muted)]">
                  点击「测试连接并保存」后进入连接校验流程；校验失败会给出中文失败分类与修正建议，
                  表单内容保留，不会丢失已填写的配置。
                </p>
              </CardContent>
            </Card>
          ) : null}

          {/* 步骤导航 */}
          <div className="flex items-center justify-between gap-[var(--space-2)]">
            <Button
              variant="secondary"
              onClick={() => store.setStep(Math.max(1, store.step - 1))}
              disabled={store.step === 1}
            >
              上一步
            </Button>
            <div className="flex items-center gap-[var(--space-2)]">
              {store.step < 3 ? (
                <Button onClick={() => validateStep(store.step)}>下一步</Button>
              ) : (
                <FormActions
                  saving={store.submitting}
                  onSave={() => void handleSave()}
                  onCancel={() => router.push('/datasources')}
                />
              )}
            </div>
          </div>
        </div>

        {/* 右侧摘要 */}
        <div className="lg:col-span-4">
          <FormSummary
            name={store.values.name}
            dialect={store.values.dialect}
            host={store.values.host}
            port={store.values.port}
            serviceName={store.values.serviceName}
            username={store.values.username}
            includeSchemas={store.values.includeSchemas}
          />
        </div>
      </div>
    </div>
  );
}
