import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// 合并 className：clsx 处理条件类，tailwind-merge 消除冲突的 Tailwind 工具类。
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
