/**
 * dbdlp 控制台 · 类名合并工具
 *
 * shadcn/ui 标准用法：clsx + tailwind-merge 合并条件类名。
 */

import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
