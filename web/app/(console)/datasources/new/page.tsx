import type { Metadata } from 'next';
import { DataSourceFormPage } from '../DataSourceFormPage';

export const metadata: Metadata = {
  title: '新建数据源 · 达梦敏感数据扫描发现平台',
};

export default function NewDataSourcePage() {
  return <DataSourceFormPage />;
}
