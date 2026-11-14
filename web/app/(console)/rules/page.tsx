'use client';

/**
 * dbdlp 控制台 · P11 规则管理页（04 文档 §2.3）
 *
 * 规则表格 + 右侧详情面板；顶部版本徽章与快照切换。
 * **只读查看**：不提供在线编辑规则的能力。
 */

import * as React from 'react';
import { Lock, RefreshCw } from 'lucide-react';
import { useRuleStore } from '@/stores/useRuleStore';
import { Button } from '@/components/ui/button';
import { Alert, Badge } from '@/components/ui/primitives';
import {
  RuleDetailPanel,
  RuleSnapshotList,
  RuleTable,
  RuleVersionBadge,
} from '@/components/business/rules';
import { PageDensity } from '@/components/shell/PageDensity';

export default function RulesPage() {
  const store = useRuleStore();

  React.useEffect(() => {
    void store.fetchRules();
    void store.fetchSnapshots();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const selected = store.rules.find((rule) => rule.id === store.selectedId) ?? null;

  return (
    <div className="flex flex-col gap-[var(--section-y-desktop)]">
      <PageDensity density="compact" />
      {/* 页头 */}
      <div className="flex flex-wrap items-center justify-between gap-[var(--space-3)]">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h1>规则管理</h1>
          <p className="text-[var(--text-sm)] text-[var(--muted)]">
            查看当前生效的敏感识别规则与历史版本快照
          </p>
          <p className="flex items-center gap-[var(--space-1)] text-[var(--text-xs)] text-[var(--muted)]">
            <Lock size={12} aria-hidden="true" />
            规则为只读视图，变更需在服务端配置并发布新版本
          </p>
        </div>
        <div className="flex items-center gap-[var(--space-2)]">
          {store.version ? <RuleVersionBadge version={store.version} /> : null}
          <Button
            variant="secondary"
            size="sm"
            loading={store.loading}
            onClick={() => {
              void store.fetchRules();
              void store.fetchSnapshots();
            }}
          >
            <RefreshCw size={16} aria-hidden="true" />
            刷新
          </Button>
        </div>
      </div>

      {store.error ? (
        <Alert tone="danger" title="规则清单加载失败">
          {store.error}
        </Alert>
      ) : null}

      <div className="grid grid-cols-1 gap-[var(--gutter-desktop)] lg:grid-cols-12">
        <div className="flex flex-col gap-[var(--gutter-desktop)] lg:col-span-8">
          <RuleTable
            rules={store.rules}
            loading={store.loading}
            selectedId={store.selectedId}
            onSelect={(id) => store.setSelected(id)}
          />
          <RuleSnapshotList
            snapshots={store.snapshots}
            currentVersion={store.version}
            onSelect={(version) => void store.fetchRules(version)}
          />
        </div>
        <div className="lg:col-span-4">
          <RuleDetailPanel rule={selected} />
        </div>
      </div>
    </div>
  );
}
