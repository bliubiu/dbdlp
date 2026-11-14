import type { NextConfig } from 'next';

/**
 * dbdlp 网页控制台构建配置
 *
 * 静态导出（output: 'export'）：产物输出到 web/out，由 Go 进程同端口托管（02 文档 §13.1、§19.1）。
 * 因此前端一律使用相对路径 /api/v1/* 调用后端，不存在独立 Node 服务、无跨域配置。
 *
 * trailingSlash: true —— 导出为「目录 + index.html」形态（out/datasources/index.html）。
 * 原因：Go 侧 handleStatic 对目录请求读取 `<dir>/index.html`，平铺的 `.html` 形态无法命中，
 * 会走 SPA 回落把根 index.html 返给所有子路由。目录形态与 Go 托管逻辑严格对齐。
 *
 * 说明：动态路由段（[id]）在静态导出下由 generateStaticParams 提供占位参数以产出静态文件，
 * 真实 id 由客户端在运行时从路径参数读取并向 /api/v1/* 取数（见 app/(console) 下各页）。
 */
const nextConfig: NextConfig = {
  output: 'export',
  reactStrictMode: true,
  trailingSlash: true,
  images: { unoptimized: true },
};

export default nextConfig;
