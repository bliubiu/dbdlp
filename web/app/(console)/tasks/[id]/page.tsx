import type { Metadata } from 'next';
import { TaskDetailPage } from './TaskDetailPage';

export const metadata: Metadata = {
  title: '扫描任务详情 · 达梦敏感数据扫描发现平台',
};

/**
 * 静态导出下动态路由需要一个已知参数以产出静态文件；
 * 真实 id 由客户端在挂载后从路径参数读取并向 /api/v1/tasks/{id} 取数。
 */
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'sample' }];
}

export default function TaskDetailRoute() {
  return <TaskDetailPage />;
}
