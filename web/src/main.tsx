import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App.tsx'

// 渲染前先按已保存值 / 系统偏好应用一次主题，避免暗色模式下的首屏闪烁。
{
  const stored = localStorage.getItem('d2c_theme')
  const dark =
    stored === 'dark' ||
    (stored === null && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
