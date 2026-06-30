import { useEffect, useRef, useState, type FormEvent } from 'react'
import { AlertCircle, Bot, Send, User } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

import {
  aiChat,
  AuthError,
  type ChatMessage,
  type ChatRequest,
} from '@/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

// 配置中存储的 key，用于 localStorage 记住用户的 API 设置。
const STORAGE_URL = 'd2c_ai_url'
const STORAGE_KEY = 'd2c_ai_key'
const STORAGE_MODEL = 'd2c_ai_model'

// 默认值
const DEFAULT_URL = 'https://api.openai.com/v1'
const DEFAULT_MODEL = 'gpt-4o-mini'

type ConfigState = {
  url: string
  key: string
  model: string
}

function loadConfig(): ConfigState {
  return {
    url: localStorage.getItem(STORAGE_URL) ?? DEFAULT_URL,
    key: localStorage.getItem(STORAGE_KEY) ?? '',
    model: localStorage.getItem(STORAGE_MODEL) ?? DEFAULT_MODEL,
  }
}

function saveConfig(c: ConfigState) {
  if (c.url) localStorage.setItem(STORAGE_URL, c.url)
  if (c.key) localStorage.setItem(STORAGE_KEY, c.key)
  if (c.model) localStorage.setItem(STORAGE_MODEL, c.model)
}

export default function AIChatPage() {
  const navigate = useNavigate()

  // 配置区
  const [config, setConfig] = useState<ConfigState>(loadConfig)
  const [showConfig, setShowConfig] = useState(true)

  // 对话区
  const [messages, setMessages] = useState<ChatMessage[]>([
    { role: 'assistant', content: '你好！我是 AI 助手。请在上方配置 API 信息，然后开始聊天。' },
  ])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const bottomRef = useRef<HTMLDivElement>(null)

  // 自动滚到底
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  function handleAuthError(e: unknown): boolean {
    if (e instanceof AuthError) {
      navigate('/auth', { replace: true, state: { reason: 'expired' } })
      return true
    }
    return false
  }

  function updateConfig<K extends keyof ConfigState>(key: K, value: string) {
    setConfig((c) => ({ ...c, [key]: value }))
  }

  function applyConfig() {
    saveConfig(config)
    setShowConfig(false)
    setError(null)
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    const text = input.trim()
    if (!text || loading) return

    const userMsg: ChatMessage = { role: 'user', content: text }
    setMessages((prev) => [...prev, userMsg])
    setInput('')
    setLoading(true)
    setError(null)

    // 补一条占位 assistant 消息
    setMessages((prev) => [...prev, { role: 'assistant', content: '……' }])

    const body: ChatRequest = {
      url: config.url,
      key: config.key,
      model: config.model,
      messages: [...messages, userMsg].map((m) => ({ role: m.role, content: m.content })),
    }

    try {
      const res = await aiChat(body)
      const reply = res.data.message

      // 替换最后一条占位消息
      setMessages((prev) => {
        const next = [...prev]
        next[next.length - 1] = reply
        return next
      })
    } catch (e) {
      // 移除占位
      setMessages((prev) => prev.slice(0, -1))
      if (handleAuthError(e)) return
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">AI 聊天</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          配置 OpenAI 兼容 API，测试接口可用性。
        </p>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* ---- 配置区 ---- */}
      <Card>
        <CardContent className="pt-6">
          {showConfig ? (
            <div className="space-y-4">
              <div className="grid grid-cols-[1fr_1fr_1fr] gap-3">
                <div className="space-y-2">
                  <Label htmlFor="url">API 地址 (URL)</Label>
                  <Input
                    id="url"
                    type="text"
                    placeholder={DEFAULT_URL}
                    value={config.url}
                    onChange={(e) => updateConfig('url', e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="key">API 密钥 (Key)</Label>
                  <Input
                    id="key"
                    type="password"
                    placeholder="sk-..."
                    value={config.key}
                    onChange={(e) => updateConfig('key', e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="model">模型名 (Model)</Label>
                  <Input
                    id="model"
                    type="text"
                    placeholder={DEFAULT_MODEL}
                    value={config.model}
                    onChange={(e) => updateConfig('model', e.target.value)}
                  />
                </div>
              </div>
              <div className="flex gap-2">
                <Button onClick={applyConfig}>应用配置</Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setConfig(loadConfig())
                  }}
                >
                  重置
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-between">
              <div className="space-y-0.5 text-sm">
                <span className="text-muted-foreground">API: </span>
                <code className="rounded bg-muted px-1 py-0.5 text-xs">{config.url}</code>
                <span className="ml-3 text-muted-foreground">Model: </span>
                <code className="rounded bg-muted px-1 py-0.5 text-xs">{config.model}</code>
              </div>
              <Button variant="outline" size="sm" onClick={() => setShowConfig(true)}>
                修改配置
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* ---- 聊天区 ---- */}
      <Card>
        <CardContent className="flex flex-col space-y-4 pt-6">
          <div className="flex max-h-[420px] min-h-[300px] flex-col gap-4 overflow-y-auto rounded-lg border bg-muted/30 p-4">
            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex gap-2 ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                {msg.role !== 'user' && (
                  <div className="mt-1 flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10">
                    <Bot className="size-4 text-primary" />
                  </div>
                )}
                <div
                  className={`max-w-[80%] whitespace-pre-wrap rounded-lg px-3 py-2 text-sm leading-relaxed ${
                    msg.role === 'user'
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-card text-card-foreground'
                  }`}
                >
                  {msg.content}
                </div>
                {msg.role === 'user' && (
                  <div className="mt-1 flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/20">
                    <User className="size-4 text-primary" />
                  </div>
                )}
              </div>
            ))}
            <div ref={bottomRef} />
          </div>

          <form onSubmit={onSubmit} className="flex gap-2">
            <Input
              placeholder="输入消息…"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              disabled={loading || showConfig}
            />
            <Button type="submit" disabled={loading || showConfig || !input.trim()}>
              <Send />
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
