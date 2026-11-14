'use client';

/**
 * dbdlp 控制台 · 数据源表单切片（04 文档 §2.5，P03）
 *
 * 职责：分步草稿、中文错误、测试状态机。
 * 持久化：会话存储，且**必须剔除 password**（04 文档 §2.5、§3.7、§4.6）。
 *
 * 凭据不出后端：password 只做输入，不回显、不缓存、不写日志。
 */

import { create } from 'zustand';
import { createDataSource, updateDataSource } from '@/services/datasources';
import { ApiError } from '@/services/http';
import type { DataSourceInput, Dialect } from '@/types/api';

/** 连接测试状态机（§3.3）：idle → testing → success | fail */
export type TestState = 'idle' | 'testing' | 'success' | 'fail';

export interface DataSourceFormValues {
  name: string;
  dialect: Dialect;
  host: string;
  port: number;
  serviceName: string;
  username: string;
  /** 永不持久化 */
  password: string;
  includeSchemas: string[];
  excludeTables: string;
  excludeColumns: string;
}

export const emptyFormValues: DataSourceFormValues = {
  name: '',
  dialect: 'dm',
  host: '',
  port: 5236,
  serviceName: '',
  username: '',
  password: '',
  includeSchemas: [],
  excludeTables: '',
  excludeColumns: '',
};

const DRAFT_KEY = 'dbdlp.datasource.draft';

/** 剔除 password 后的草稿（唯一允许进入持久化层的形态） */
type PersistedDraft = Omit<DataSourceFormValues, 'password'>;

/** 读取草稿（不含凭据） */
function readDraft(): PersistedDraft | null {
  if (typeof window === 'undefined') return null;
  try {
    const raw = window.sessionStorage.getItem(DRAFT_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as PersistedDraft & { password?: string };
    // 双保险：即便历史草稿里混入了 password，也在读取时剔除
    const { password: _omit, ...safe } = parsed;
    return safe;
  } catch {
    return null;
  }
}

interface DataSourceFormState {
  values: DataSourceFormValues;
  /** 字段级中文错误：字段名 → 中文提示 */
  errors: Record<string, string>;
  /** 当前步骤 1~3 */
  step: number;
  testState: TestState;
  /** 测试结论（中文） */
  testMessage: string;
  testLatencyMs: number;
  submitting: boolean;
  /** 提交失败的顶层中文错误 */
  submitError: string | null;
  setValue: <K extends keyof DataSourceFormValues>(
    key: K,
    value: DataSourceFormValues[K],
  ) => void;
  setValues: (values: Partial<DataSourceFormValues>) => void;
  setFieldError: (field: string, message: string | null) => void;
  clearErrors: () => void;
  setStep: (step: number) => void;
  setTestState: (state: TestState, message?: string, latencyMs?: number) => void;
  /** 载入编辑态初始值（password 不回填，后端返回占位串） */
  hydrate: (values: Partial<DataSourceFormValues>) => void;
  reset: () => void;
  /** 提交：新建或编辑；成功后返回数据源 id */
  submit: (id?: string) => Promise<string | null>;
}

/** 写入草稿（剔除 password） */
function writeDraft(values: DataSourceFormValues): void {
  if (typeof window === 'undefined') return;
  const { password: _omit, ...safe } = values;
  try {
    window.sessionStorage.setItem(DRAFT_KEY, JSON.stringify(safe));
  } catch {
    /* 忽略存储失败 */
  }
}

export const useDataSourceFormStore = create<DataSourceFormState>((set, get) => ({
  values: { ...emptyFormValues, ...(readDraft() ?? {}) },
  errors: {},
  step: 1,
  testState: 'idle',
  testMessage: '',
  testLatencyMs: 0,
  submitting: false,
  submitError: null,

  setValue: (key, value) => {
    const values = { ...get().values, [key]: value };
    set({ values });
    writeDraft(values);
  },

  setValues: (partial) => {
    const values = { ...get().values, ...partial };
    set({ values });
    writeDraft(values);
  },

  setFieldError: (field, message) => {
    const errors = { ...get().errors };
    if (message === null) delete errors[field];
    else errors[field] = message;
    set({ errors });
  },

  clearErrors: () => set({ errors: {} }),

  setStep: (step) => set({ step }),

  setTestState: (state, message = '', latencyMs = 0) =>
    set({ testState: state, testMessage: message, testLatencyMs: latencyMs }),

  hydrate: (partial) => {
    const values = { ...get().values, ...partial };
    set({ values, errors: {}, testState: 'idle', testMessage: '', submitError: null });
    writeDraft(values);
  },

  reset: () => {
    if (typeof window !== 'undefined') {
      try {
        window.sessionStorage.removeItem(DRAFT_KEY);
      } catch {
        /* 忽略 */
      }
    }
    set({
      values: { ...emptyFormValues },
      errors: {},
      step: 1,
      testState: 'idle',
      testMessage: '',
      testLatencyMs: 0,
      submitting: false,
      submitError: null,
    });
  },

  submit: async (id) => {
    const { values, setFieldError, clearErrors } = get();
    clearErrors();
    set({ submitting: true, submitError: null, testState: 'testing', testMessage: '正在校验只读连接…' });

    const payload: DataSourceInput = {
      name: values.name.trim(),
      dialect: values.dialect,
      host: values.host.trim(),
      port: Number(values.port),
      serviceName: values.serviceName.trim(),
      username: values.username.trim(),
      password: values.password,
      includeSchemas: values.includeSchemas,
      excludeTables: values.excludeTables
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter(Boolean),
      excludeColumns: values.excludeColumns
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter(Boolean),
    };

    try {
      if (id) {
        // 编辑：password 留空表示不修改（不传该字段）
        const updatePayload: Partial<DataSourceInput> = { ...payload };
        if (!values.password) delete updatePayload.password;
        await updateDataSource(id, updatePayload);
        set({ submitting: false, testState: 'success', testMessage: '连接成功，只读账号校验通过，配置已保存' });
        return id;
      }
      const result = await createDataSource(payload);
      set({ submitting: false, testState: 'success', testMessage: '连接成功，只读账号校验通过，配置已保存' });
      // 保存成功后清掉含凭据的草稿
      if (typeof window !== 'undefined') {
        try {
          window.sessionStorage.removeItem(DRAFT_KEY);
        } catch {
          /* 忽略 */
        }
      }
      return result.id;
    } catch (error) {
      const apiError = error instanceof ApiError ? error : null;
      const message = apiError?.message ?? '保存失败，请稍后重试';
      set({ submitting: false, testState: 'fail', testMessage: message, submitError: message });

      // 1001 参数校验失败：映射到具体字段（错误近字段回显，§8.4）
      if (apiError?.code === 1001) {
        const field = guessFieldFromMessage(message);
        if (field) setFieldError(field, message);
      }
      return null;
    }
  },
}));

/** 从后端中文校验消息推断出错字段（仅用于 1001 的就近回显） */
function guessFieldFromMessage(message: string): string | null {
  if (/名称|标识|实例名|重复/.test(message)) return 'name';
  if (/主机|地址|IP/.test(message)) return 'host';
  if (/端口/.test(message)) return 'port';
  if (/服务名|库名/.test(message)) return 'serviceName';
  if (/账号|用户名/.test(message)) return 'username';
  if (/密码|凭据/.test(message)) return 'password';
  if (/方言/.test(message)) return 'dialect';
  if (/模式/.test(message)) return 'includeSchemas';
  return null;
}
