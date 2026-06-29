import { useState, type FormEvent } from 'react'
import { AlertCircle } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

import { setToken } from '@/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

// 鉴权页：填 token。提交后写入 localStorage 并跳转首页。
export default function AuthPage() {
  const navigate = useNavigate()
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  function submit(e: FormEvent) {
    e.preventDefault()
    const t = value.trim()
    if (!t) {
      setError('请输入 token')
      return
    }
    setToken(t)
    navigate('/', { replace: true })
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4 text-foreground">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl">Jupi D2C</CardTitle>
          <CardDescription>
            本地控制面板 · 首次访问请输入{' '}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">TOKEN</code>。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="token">访问令牌 (TOKEN)</Label>
              <Input
                id="token"
                type="password"
                autoFocus
                value={value}
                onChange={(e) => {
                  setValue(e.target.value)
                  setError(null)
                }}
                placeholder="请输入 token"
              />
              <p className="text-xs text-muted-foreground">
                token 仅保存在浏览器 localStorage 中，不会上传。
              </p>
            </div>

            {error && (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <Button type="submit" className="w-full">
              进入面板
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
