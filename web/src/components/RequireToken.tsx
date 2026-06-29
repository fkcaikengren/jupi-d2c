import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

import { getToken } from '@/api'

// 路由守卫：无 token 时重定向到 /auth，并记下来源路径以便登录后跳回。
export function RequireToken({ children }: { children: ReactNode }) {
  const location = useLocation()
  if (!getToken()) {
    return <Navigate to="/auth" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
