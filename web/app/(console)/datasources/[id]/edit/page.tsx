import type { Metadata } from 'next';
import { DataSourceFormPage } from '../../DataSourceFormPage';

export const metadata: Metadata = {
  title: '编辑数据源 · 达梦敏感数据扫描发现平台',
};

/**
 * 静态导出下动态路由需要一个已知参数以产出静态文件；
 * 真实 id 由客户端在挂载后从路径参数读取并向 /api/v1/* 取数。
 */
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'sample' }];
}

export default async function EditDataSourcePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <DataSourceFormPage id={id} />;
}
