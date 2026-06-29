import { LogOut, Moon, Settings, Sun } from 'lucide-react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'

import { clearToken } from '@/api'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/lib/theme'

// 带顶栏的主布局：应用名 + 右侧操作区（暗色切换 / 设置 / 退出），下方为路由出口。
export function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { theme, toggle } = useTheme()

  function logout() {
    clearToken()
    navigate('/auth', { replace: true })
  }

  const onSetting = location.pathname === '/setting'

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-10 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-14 max-w-3xl items-center justify-between px-4">
          <Link to="/" className="flex items-center gap-2 font-semibold">
            <img src="/jupi-logo.png" alt="Jupi D2C" className="h-6 w-6 rounded-full" />
            <span className="text-base tracking-tight">Jupi D2C</span>
          </Link>

          <nav className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              onClick={toggle}
              title={theme === 'dark' ? '切换到亮色' : '切换到暗色'}
              aria-label="切换主题"
            >
              {theme === 'dark' ? <Sun /> : <Moon />}
            </Button>
            <Button
              variant={onSetting ? 'secondary' : 'ghost'}
              size="icon"
              asChild
              title="配置"
              aria-label="配置"
            >
              <Link to="/setting">
                <Settings />
              </Link>
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={logout}
              title="退出登录"
              aria-label="退出登录"
            >
              <LogOut />
            </Button>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  )
}
