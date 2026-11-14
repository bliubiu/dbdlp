'use client';

/**
 * dbdlp 控制台 · P12 系统设置页（04 文档 §2.3、§9）
 *
 * Tab 分组：生效配置 / 审计日志 / 外观，每组 ≤4 字段。
 * 不提供深色主题切换。
 */

import * as React from 'react';
import { ClipboardList, Rows3, SlidersHorizontal } from 'lucide-react';
import { useConfigStore } from '@/stores/useConfigStore';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/primitives';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/overlays';
import {
  AppearancePanel,
  AuditFilterBar,
  AuditLogTable,
  ConfigReadOnlyPanel,
  SessionHint,
} from '@/components/business/config';

export default function ConfigPage() {
  const store = useConfigStore();

  React.useEffect(() => {
    void store.fetchConfig();
    void store.fetchAuditLogs();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const totalPages = Math.max(1, Math.ceil(store.totalAuditLogs / 50));

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      {/* 页头 */}
      <div className="flex flex-col gap-[var(--space-1)]">
        <h1>系统设置</h1>
        <p className="text-[var(--text-sm)] text-[var(--muted)]">
          生效配置只读回显、审计日志与纯前端外观偏好
        </p>
        <SessionHint />
      </div>

      {store.error ? (
        <Alert tone="danger" title="加载失败">
          {store.error}
        </Alert>
      ) : null}

      <Tabs defaultValue="config">
        <TabsList>
          <TabsTrigger value="config">
            <SlidersHorizontal size={16} aria-hidden="true" />
            生效配置
          </TabsTrigger>
          <TabsTrigger value="audit">
            <ClipboardList size={16} aria-hidden="true" />
            审计日志
          </TabsTrigger>
          <TabsTrigger value="appearance">
            <Rows3 size={16} aria-hidden="true" />
            外观
          </TabsTrigger>
        </TabsList>

        <TabsContent value="config">
          <ConfigReadOnlyPanel config={store.config} loading={store.loading} />
        </TabsContent>

        <TabsContent value="audit">
          <div className="flex flex-col gap-[var(--space-3)]">
            <AuditFilterBar
              actor={store.auditActor}
              action={store.auditAction}
              onActorChange={(value) => store.setAuditActor(value)}
              onActionChange={(value) => store.setAuditAction(value)}
            />
            <AuditLogTable logs={store.auditLogs} total={store.totalAuditLogs} />
            <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
              <span className="text-[var(--text-xs)] text-[var(--muted)]">
                共 {store.totalAuditLogs} 条，当前第 {store.auditPage} / {totalPages} 页
              </span>
              <div className="flex items-center gap-[var(--space-2)]">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={store.auditPage <= 1}
                  onClick={() => {
                    const next = store.auditPage - 1;
                    store.setAuditPage(next);
                    void store.fetchAuditLogs();
                  }}
                >
                  上一页
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={store.auditPage >= totalPages}
                  onClick={() => {
                    const next = store.auditPage + 1;
                    store.setAuditPage(next);
                    void store.fetchAuditLogs();
                  }}
                >
                  下一页
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="appearance">
          <AppearancePanel />
        </TabsContent>
      </Tabs>
    </div>
  );
}
