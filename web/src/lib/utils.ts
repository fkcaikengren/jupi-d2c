import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// 合并 className：clsx 处理条件类，tailwind-merge 消除冲突的 Tailwind 工具类。
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// 把字节数格式化为人类可读字符串（B / KB / MB / GB）。
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / 1024 ** i
  // 整数不带小数，否则保留 1 位
  const text = i === 0 || value >= 100 ? Math.round(value).toString() : value.toFixed(1)
  return `${text} ${units[i]}`
}
