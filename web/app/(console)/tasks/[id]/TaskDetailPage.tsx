'use client';

/**
 * dbdlp 控制台 · P06 扫描任务详情（客户端部分）
 *
 * 头部摘要卡 → 进度区 → Tab（表进度 / 失败跳过对象 / 审计日志 / 参数快照）。
 * 真实 id 由客户端从路径参数读取并向 /api/v1/tasks/{id} 取数。
 */

import * as React from 'react';
import { useParams } from 'next/navigation';
import { ClipboardList, Activity, SlidersHorizontal, TriangleExclamationPoint } from 'lucide-react';
import { useTaskDetailStore } from '@/stores/useTaskDetailStore';
import { listAuditLogs } from '@/services/system';
import { Alert, Skeleton } from '@/components/ui/primitives';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/overlays';
import {
  AuditLogViewer,
  FailedObjectTable,
  ObjectProgressTable,
  ParamSnapshot,
  TaskActionDialog,
  TaskHeader,
  TaskProgressSegments,
} from '@/components/business/task-detail';
import type { AuditLog } from '@/types/api';
import { PageDensity } from '@/components/shell/PageDensity';

type ActionKind = 'pause' | 'resume' | 'retry' | null;

export function TaskDetailPage() {
  const params = useParams<{ id: string }>();
  const id = typeof params?.id === 'string' ? params.id : '';
  const store = useTaskDetailStore();
  const [auditLogs, setAuditLogs] = React.useState<AuditLog[]>([]);
  const [action, setAction] = React.useState<ActionKind>(null);
  const [pending, setPending] = React.useState(false);
  const [actionError, setActionError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    void store.load(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  // 任务运行中时按 3 秒轮询进度
  React.useEffect(() => {
    if (!id) return;
    const status = store.progress?.status ?? store.task?.status;
    if (status !== 'running') return;
    store.startPolling(id);
    return () => store.stopPolling();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, store.progress?.status, store.task?.status]);

  // 审计日志按任务目标过滤
  React.useEffect(() => {
    if (!id) return;
    let cancelled = false;
    void listAuditLogs({ page: 1, limit: 50 })
      .then((result) => {
        if (cancelled) return;
        const items = (result.items ?? []).filter(
          (log) => !log.target || log.target.includes(id),
        );
        setAuditLogs(items);
      })
      .catch(() => {
        if (!cancelled) setAuditLogs([]);
      });
    return () => {
      cancelled = true;
    };
  }, [id, store.task?.finishedAt]);

  const handleAction = async (kind: ActionKind, targetIds: string[] = []) => {
    if (!kind) return;
    setPending(true);
    setActionError(null);
    if (kind === 'pause') {
      await store.pause(id);
    } else if (kind === 'resume') {
      await store.resume(id);
    } else {
      await store.retry(id, targetIds);
    }
    setPending(false);
    setAction(null);
  };

  if (!id) {
    return (
      <Alert tone="danger" title="任务编号缺失">
        任务地址不完整，请从扫描任务列表重新进入。
      </Alert>
    );
  }

  if (store.loading && !store.task) {
    return (
      <div className="flex flex-col gap-[var(--space-3)]">
        <Skeleton className="h-[120px] w-full" />
        <Skeleton className="h-[200px] w-full" />
      </div>
    );
  }

  if (store.error) {
    return (
      <Alert tone="danger" title="任务加载失败">
        {store.error}
      </Alert>
    );
  }

  if (!store.task) {
    return (
      <Alert tone="warning" title="任务不存在">
        指定的任务不存在或已过期，请返回扫描任务列表重新选择。
      </Alert>
    );
  }

  const task = store.task;

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <PageDensity density="compact" />
      <TaskHeader
        task={task}
        busy={pending}
        onPause={() => {
          setActionError(null);
          setAction('pause');
        }}
        onResume={() => {
          setActionError(null);
          setAction('resume');
        }}
        onRetry={() => {
          setActionError(null);
          setAction('retry');
        }}
      />

      {store.notice ? <Alert tone="info">{store.notice}</Alert> : null}

      <TaskProgressSegments progress={store.progress} />

      <Tabs value={store.tab} onValueChange={(value) => store.setTab(value as typeof store.tab)}>
        <TabsList>
          <TabsTrigger value="progress">
            <Activity size={16} aria-hidden="true" />
            表进度
          </TabsTrigger>
          <TabsTrigger value="failed">
            <TriangleExclamationPoint size={16} aria-hidden="true" />
            失败跳过对象
          </TabsTrigger>
          <TabsTrigger value="audit">
            <ClipboardList size={16} aria-hidden="true" />
            审计日志
          </TabsTrigger>
          <TabsTrigger value="params">
            <SlidersHorizontal size={16} aria-hidden="true" />
            参数快照
          </TabsTrigger>
        </TabsList>

        <TabsContent value="progress">
          <ObjectProgressTable
            totalTables={task.summary?.totalTables ?? 0}
            doneTables={task.summary?.doneTables ?? 0}
            failedTables={task.summary?.failedTables ?? 0}
            skippedTables={task.summary?.skippedTables ?? 0}
          />
        </TabsContent>

        <TabsContent value="failed">
          <FailedObjectTable
            objects={task.summary?.failedObjects ?? []}
            onRetry={() => {
              setActionError(null);
              setAction('retry');
            }}
          />
        </TabsContent>

        <TabsContent value="audit">
          <AuditLogViewer logs={auditLogs} />
        </TabsContent>

        <TabsContent value="params">
          <ParamSnapshot task={task} />
        </TabsContent>
      </Tabs>

      <TaskActionDialog
        open={action !== null}
        onOpenChange={(open) => {
          if (!open) {
            setAction(null);
            setActionError(null);
          }
        }}
        title={
          action === 'pause'
            ? '确认暂停任务'
            : action === 'resume'
              ? '确认继续执行'
              : '确认重试失败对象'
        }
        description={
          action === 'pause'
            ? '暂停会在当前检查点落盘，已完成的表不会重扫，可随时继续执行。'
            : action === 'resume'
              ? '继续执行会从上次检查点恢复，不会重复扫描已完成的表。'
              : '重试会重新建立只读连接并重新扫描失败对象，已成功的表不受影响。'
        }
        confirmLabel={
          action === 'pause' ? '暂停任务' : action === 'resume' ? '继续执行' : '重试失败对象'
        }
        pending={pending}
        error={actionError}
        onConfirm={() => {
          if (action === 'retry') {
            void handleAction('retry', task.summary?.failedObjects?.map((o) => o.id) ?? []);
          } else {
            void handleAction(action);
          }
        }}
      />
    </div>
  );
}
