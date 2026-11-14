import type { Metadata, Viewport } from 'next';
import './globals.css';
import { ToastContainer } from '@/components/shell/ToastContainer';
import { DensityProvider } from '@/components/shell/DensityProvider';
import { CommandPalette } from '@/components/shell/CommandPalette';

export const metadata: Metadata = {
  title: '达梦敏感数据扫描发现平台',
  description: '资产数据源管理、扫描任务、字段台账与报告中心',
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  // 允许 200% 缩放不丢内容（§8.5）
  maximumScale: 2,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" data-theme="light" data-density="default" suppressHydrationWarning>
      <body className="min-h-screen bg-[var(--bg)] text-[var(--fg)] antialiased">
        <DensityProvider>
          {children}
          <ToastContainer />
          <CommandPalette />
        </DensityProvider>
      </body>
    </html>
  );
}
