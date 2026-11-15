'use client';

/**
 * dbdlp 控制台 · 批量导入切片（04 文档 §2.5、§3.4，P04）
 *
 * 职责：导入阶段、上传进度、结果、错误行。持久化：否。
 * 口径：逐条门禁 + 部分成功——通过的行入库、失败行不入库并逐行中文回显。
 */

import { create } from 'zustand';
import { ImportAllFailedError, importDataSources } from '@/services/datasources';
import { ApiError } from '@/services/http';
import type { ImportErrorRow } from '@/types/api';

/** 导入状态机（§3.4）：idle → selected → uploading → processing → done | failed */
export type ImportPhase = 'idle' | 'selected' | 'uploading' | 'processing' | 'done' | 'failed';

export type { ImportErrorRow } from '@/types/api';

interface ImportState {
  phase: ImportPhase;
  /** 已选文件元信息（不保留文件明文内容，不落地前端） */
  fileName: string;
  fileSize: number;
  fileRows: number;
  /** 上传进度 0~100 */
  progress: number;
  total: number;
  succeeded: number;
  failed: number;
  errors: ImportErrorRow[];
  /** 整批全部失败（后端 code=2001）：零写入，但逐行 errors 仍须回显 */
  allFailed: boolean;
  /** 后端中文 message（2001 分支用于顶部回显，逐行明细另见 errors） */
  serverMessage: string | null;
  /** 中文阶段失败原因 */
  error: string | null;
  setFile: (file: File) => void;
  clearFile: () => void;
  setProgress: (progress: number) => void;
  submit: () => Promise<void>;
  reset: () => void;
}

/** 文件级预检（§3.4 阶段 1）：扩展名、大小 ≤10MB */
const MAX_FILE_SIZE = 10 * 1024 * 1024;
const ALLOWED_EXT = ['.xlsx', '.xls', '.yaml', '.yml'];

export const useImportStore = create<ImportState>((set, get) => ({
  phase: 'idle',
  fileName: '',
  fileSize: 0,
  fileRows: 0,
  progress: 0,
  total: 0,
  succeeded: 0,
  failed: 0,
  errors: [],
  allFailed: false,
  serverMessage: null,
  error: null,

  setFile: (file) => {
    const lower = file.name.toLowerCase();
    const extOk = ALLOWED_EXT.some((ext) => lower.endsWith(ext));
    if (!extOk) {
      forgetSelectedFile();
      set({
        phase: 'failed',
        error: '文件格式不支持，请选择 .xlsx、.xls、.yaml 或 .yml 文件',
        fileName: '',
        fileSize: 0,
        fileRows: 0,
        errors: [],
        allFailed: false,
        serverMessage: null,
      });
      return;
    }
    if (file.size > MAX_FILE_SIZE) {
      forgetSelectedFile();
      set({
        phase: 'failed',
        error: '文件大小超过 10MB 上限，请拆分后重新导入',
        fileName: '',
        fileSize: 0,
        fileRows: 0,
        errors: [],
        allFailed: false,
        serverMessage: null,
      });
      return;
    }
    rememberSelectedFile(file);
    set({
      phase: 'selected',
      fileName: file.name,
      fileSize: file.size,
      fileRows: 0,
      error: null,
      errors: [],
      allFailed: false,
      serverMessage: null,
      total: 0,
      succeeded: 0,
      failed: 0,
      progress: 0,
    });
  },

  clearFile: () => {
    forgetSelectedFile();
    set({
      phase: 'idle',
      fileName: '',
      fileSize: 0,
      fileRows: 0,
      progress: 0,
      error: null,
      errors: [],
      allFailed: false,
      serverMessage: null,
    });
  },

  setProgress: (progress) => set({ progress }),

  submit: async () => {
    const { fileName } = get();
    if (!fileName) {
      set({ phase: 'failed', error: '尚未选择导入文件' });
      return;
    }
    set({
      phase: 'uploading',
      progress: 0,
      error: null,
      allFailed: false,
      serverMessage: null,
      errors: [],
      total: 0,
      succeeded: 0,
      failed: 0,
    });

    // 重新取文件对象（文件不落地前端，仅在选择时存在于内存引用）
    const file = lastSelectedFile;
    if (!file) {
      set({ phase: 'failed', error: '导入文件已失效，请重新选择文件' });
      return;
    }

    try {
      set({ phase: 'processing', progress: 60 });
      const result = await importDataSources(file);
      // code=0：全部成功或部分成功。部分成功时 errors 含失败行，必须逐行回显。
      set({
        phase: 'done',
        progress: 100,
        total: result.total ?? 0,
        succeeded: result.succeeded ?? 0,
        failed: result.failed ?? 0,
        errors: result.errors ?? [],
        allFailed: false,
        serverMessage: null,
        error: null,
      });
    } catch (error) {
      if (error instanceof ImportAllFailedError) {
        // code=2001 / HTTP 422：整批零写入，但后端 data 仍带逐行 errors。
        // 这里不能只留一句 message，必须把明细透出到失败行清单（P04 阶段四）。
        const result = error.result;
        set({
          phase: 'failed',
          progress: 100,
          total: result.total,
          succeeded: result.succeeded,
          failed: result.failed,
          errors: result.errors,
          allFailed: true,
          serverMessage: error.message,
          error: error.message,
        });
        return;
      }
      const message = error instanceof ApiError ? error.message : '导入失败，请稍后重试';
      set({ phase: 'failed', error: message, progress: 0, allFailed: false, serverMessage: null });
    }
  },

  reset: () =>
    set({
      phase: 'idle',
      fileName: '',
      fileSize: 0,
      fileRows: 0,
      progress: 0,
      total: 0,
      succeeded: 0,
      failed: 0,
      errors: [],
      allFailed: false,
      serverMessage: null,
      error: null,
    }),
}));

/**
 * 当前选中的 File 引用（模块级持有，不进 store 序列化，不写持久化层）。
 * 仅用于提交时取用文件内容；文件明文不落地前端。
 */
let lastSelectedFile: File | null = null;

/** 记录选中文件（由 ImportDropzone 调用） */
export function rememberSelectedFile(file: File): void {
  lastSelectedFile = file;
}

/** 清除选中文件引用 */
export function forgetSelectedFile(): void {
  lastSelectedFile = null;
}
