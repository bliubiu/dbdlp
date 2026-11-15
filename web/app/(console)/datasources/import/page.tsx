'use client';

/**
 * dbdlp 控制台 · P04 批量导入页（04 文档 §2.3、§3.4）
 *
 * 单列纵向四阶段流程：准备 → 提交门禁 → 结果回显 → 失败处置；顶部提供模板下载。
 */

import * as React from 'react';
import { Download, FileDown, FileSpreadsheet, FileBraces } from 'lucide-react';
import { useImportStore } from '@/stores/useImportStore';
import { useUIStore } from '@/stores/useUIStore';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/primitives';
import { downloadImportTemplate } from '@/services/datasources';
import {
  ImportDropzone,
  ImportErrorPanel,
  ImportPhaseNotice,
  ImportProgressPanel,
  ImportResultGrid,
  ImportResultSummary,
  SkippedHint,
} from '@/components/business/import';
import { PageDensity } from '@/components/shell/PageDensity';

export default function DataSourceImportPage() {
  const store = useImportStore();
  const pushToast = useUIStore((s) => s.pushToast);
  const [templateBusy, setTemplateBusy] = React.useState<string | null>(null);
  const [templateError, setTemplateError] = React.useState<string | null>(null);

  const handleTemplate = async (format: 'excel' | 'yaml') => {
    setTemplateBusy(format);
    setTemplateError(null);
    try {
      const blob = await downloadImportTemplate(format);
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = format === 'excel' ? '数据源导入模板.xlsx' : '数据源导入模板.yaml';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      pushToast('模板已开始下载，填写后再上传导入', 'success');
    } catch (error) {
      const message = error instanceof Error ? error.message : '模板下载失败，请稍后重试';
      setTemplateError(message);
    } finally {
      setTemplateBusy(null);
    }
  };

  const busy = store.phase === 'uploading' || store.phase === 'processing';
  const hasFile = store.fileName.length > 0;

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <PageDensity density="compact" />
      {/* 页头 */}
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h1>批量导入</h1>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">
            通过 Excel 或 YAML 批量接入数据源；逐条连接测试，通过的行入库，失败行不入库
          </p>
        </div>
        <div className="flex items-center gap-[var(--space-2)]">
          <Button
            variant="secondary"
            size="sm"
            loading={templateBusy === 'excel'}
            disabled={templateBusy !== null}
            onClick={() => void handleTemplate('excel')}
          >
            <FileSpreadsheet size={16} aria-hidden="true" />
            下载 Excel 模板
          </Button>
          <Button
            variant="secondary"
            size="sm"
            loading={templateBusy === 'yaml'}
            disabled={templateBusy !== null}
            onClick={() => void handleTemplate('yaml')}
          >
            <FileBraces size={16} aria-hidden="true" />
            下载 YAML 模板
          </Button>
        </div>
      </div>

      {templateError ? (
        <Alert tone="danger" title="模板下载失败">
          {templateError}
        </Alert>
      ) : null}

      <ImportPhaseNotice
        phase={store.phase}
        failed={store.failed}
        allFailed={store.allFailed}
        total={store.total}
        serverMessage={store.serverMessage}
      />

      {/* 阶段一：准备文件 */}
      <ImportDropzone
        fileName={store.fileName}
        fileSize={store.fileSize}
        error={store.error}
        disabled={busy}
        onSelect={(file) => store.setFile(file)}
        onClear={() => store.clearFile()}
      />

      {/* 阶段二：提交门禁 */}
      <ImportProgressPanel phase={store.phase} progress={store.progress} fileName={store.fileName} />

      {hasFile && !busy && store.phase !== 'done' ? (
        <div className="flex flex-wrap items-center gap-[var(--space-2)]">
          <Button size="md" onClick={() => void store.submit()}>
            <Download size={16} aria-hidden="true" />
            提交导入
          </Button>
          <Button variant="ghost" size="md" onClick={() => store.reset()}>
            重新选择文件
          </Button>
          <span className="text-[var(--text-xs)] text-[var(--muted)]">
            提交后服务端逐行做连接测试，行数较多时耗时较长
          </span>
        </div>
      ) : null}

      {/* 阶段三：结果回显。code=2001 整批失败时同样给出总数/成功/失败三数 */}
      {store.phase === 'done' || store.allFailed ? (
        <>
          <ImportResultSummary
            total={store.total}
            succeeded={store.succeeded}
            failed={store.failed}
          />
          <ImportResultGrid total={store.total} succeeded={store.succeeded} failed={store.failed} />
        </>
      ) : null}

      {/* 阶段四：失败处置。code=0 部分成功与 code=2001 整批失败都展示逐行明细 */}
      {(store.phase === 'done' || store.allFailed) && store.errors.length > 0 ? (
        <ImportErrorPanel errors={store.errors} />
      ) : null}

      {store.phase === 'done' || store.allFailed ? (
        <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
          <SkippedHint />
          <div className="flex items-center gap-[var(--space-2)]">
            <Button variant="secondary" size="sm" onClick={() => store.reset()}>
              <FileDown size={16} aria-hidden="true" />
              再导入一批
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
