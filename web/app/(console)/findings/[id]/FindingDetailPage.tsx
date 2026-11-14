'use client';

/**
 * dbdlp 控制台 · P08 字段明细与复核（客户端部分）
 *
 * 桌面端以右侧抽屉呈现（640px），保留可分享深链 /findings/{id}。
 * 移动端回退为全屏 ActionSheet 形态（单列布局）。
 */

import * as React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { ArrowLeft, Info, ListChecks, ClipboardCheck } from 'lucide-react';
import { useFindingDetailStore } from '@/stores/useFindingDetailStore';
import { Alert, Skeleton } from '@/components/ui/primitives';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/overlays';
import {
  FindingDetailHeader,
  FindingMetaGrid,
  FindingSourceHint,
  HitDetailTable,
  ReviewForm,
  ReviewHistory,
} from '@/components/business/finding-detail';

export function FindingDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = typeof params?.id === 'string' ? params.id : '';
  const store = useFindingDetailStore();

  React.useEffect(() => {
    if (!id) return;
    void store.load(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (!id) {
    return (
      <Alert tone="danger" title="字段编号缺失">
        地址不完整，请从结果台账重新进入。
      </Alert>
    );
  }

  if (store.loading && !store.finding) {
    return (
      <div className="flex flex-col gap-[var(--space-3)]">
        <Skeleton className="h-[110px] w-full" />
        <Skeleton className="h-[240px] w-full" />
      </div>
    );
  }

  if (store.error) {
    return (
      <Alert tone="danger" title="字段明细加载失败">
        {store.error}
      </Alert>
    );
  }

  if (!store.finding) {
    return (
      <Alert tone="warning" title="字段不存在">
        指定字段不存在或已过期，请返回结果台账重新选择。
      </Alert>
    );
  }

  const finding = store.finding;

  return (
    <div className="flex flex-col gap-[var(--space-4)] lg:ml-auto lg:w-[640px]">
      <div className="flex items-center justify-between gap-[var(--space-3)] no-print">
        <Button variant="ghost" size="sm" onClick={() => router.push('/findings')}>
          <ArrowLeft size={16} aria-hidden="true" />
          返回结果台账
        </Button>
        <FindingSourceHint />
      </div>

      <FindingDetailHeader finding={finding} />

      {store.notice ? (
        <Alert tone={store.noticeTone === 'error' ? 'danger' : 'success'}>
          {store.notice}
        </Alert>
      ) : null}

      <Tabs value={store.tab} onValueChange={(value) => store.setTab(value as typeof store.tab)}>
        <TabsList>
          <TabsTrigger value="basic">
            <Info size={16} aria-hidden="true" />
            基础信息
          </TabsTrigger>
          <TabsTrigger value="hits">
            <ListChecks size={16} aria-hidden="true" />
            命中明细（{store.totalHits}）
          </TabsTrigger>
          <TabsTrigger value="review">
            <ClipboardCheck size={16} aria-hidden="true" />
            复核（{store.reviews.length}）
          </TabsTrigger>
        </TabsList>

        <TabsContent value="basic">
          <FindingMetaGrid finding={finding} />
        </TabsContent>

        <TabsContent value="hits">
          <HitDetailTable hits={store.hits} />
        </TabsContent>

        <TabsContent value="review">
          <div className="flex flex-col gap-[var(--space-4)]">
            <ReviewForm
              conclusion={store.draft.conclusion}
              comment={store.draft.comment}
              submitting={store.submitting}
              error={store.noticeTone === 'error' ? store.notice : null}
              onConclusionChange={(value) => store.setDraft({ conclusion: value })}
              onCommentChange={(value) => store.setDraft({ comment: value })}
              onSubmit={() => void store.submit(id)}
            />
            <ReviewHistory reviews={store.reviews} />
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
