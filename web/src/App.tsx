import { Navigate, Route, Routes } from 'react-router-dom'

import { Layout } from '@/components/Layout'
import { RequireToken } from '@/components/RequireToken'
import AuthPage from '@/pages/AuthPage'
import AIChatPage from '@/pages/AIChatPage'
import FilesPage from '@/pages/FilesPage'
import HelpPage from '@/pages/HelpPage'
import HomePage from '@/pages/HomePage'
import SettingPage from '@/pages/SettingPage'

// 路由表：
//  /auth     —— 鉴权页（填 token），无顶栏布局
//  /         —— 首页（需 token，带顶栏），暂空白预留
//  /files    —— 文件页（需 token，带顶栏），浏览上传目录
//  /setting  —— 配置页（需 token，带顶栏）
//  /help     —— 帮助页（需 token，带顶栏），渲染 help.md
export default function App() {
  return (
    <Routes>
      <Route path="/auth" element={<AuthPage />} />
      <Route
        element={
          <RequireToken>
            <Layout />
          </RequireToken>
        }
      >
        <Route path="/" element={<HomePage />} />
        <Route path="/files" element={<FilesPage />} />
        <Route path="/ai" element={<AIChatPage />} />
        <Route path="/setting" element={<SettingPage />} />
        <Route path="/help" element={<HelpPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
