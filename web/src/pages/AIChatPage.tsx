import { useEffect, useRef, useState, type FormEvent } from 'react'
import { AlertCircle, Bot, ChevronDown, ChevronRight, Send, User, Loader2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

import {
  aiChat,
  getAiConfig,
  updateAiConfig,
  generateReferDom,
  listDesigns,
  AuthError,
  type ChatMessage,
  type ChatRequest,
  type DesignItem,
  type ReferDomResult,
  type AiConfig,
} from '@/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

// 默认值
const DEFAULT_URL = 'https://api.openai.com/v1'
const DEFAULT_MODEL = 'gpt-4o-mini'

export default function AIChatPage() {
  const navigate = useNavigate()

  // 配置区（从服务端读取）
  const [config, setConfig] = useState<AiConfig>({
    url: '',
    key: '',
    model: '',
    updatedAt: 0,
  })
  // 编辑中的配置（未保存时用）
  const [editUrl, setEditUrl] = useState('')
  const [editKey, setEditKey] = useState('')
  const [editModel, setEditModel] = useState('')
  const [showConfig, setShowConfig] = useState(true)
  const [configLoading, setConfigLoading] = useState(false)
  const [savingConfig, setSavingConfig] = useState(false)
  const [configError, setConfigError] = useState<string | null>(null)

  // 对话区
  const [messages, setMessages] = useState<ChatMessage[]>([
    { role: 'assistant', content: '你好！我是 AI 助手。请先配置 AI 参数，然后开始聊天。' },
  ])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Refer DOM 区
  const [referDomEnabled, setReferDomEnabled] = useState(false)
  const [designs, setDesigns] = useState<DesignItem[]>([])
  const [selectedDesignId, setSelectedDesignId] = useState('')
  const [generatingDom, setGeneratingDom] = useState(false)
  const [generatedDom, setGeneratedDom] = useState<ReferDomResult | null>(null)
  const [domError, setDomError] = useState<string | null>(null)
  const [domWarningExpanded, setDomWarningExpanded] = useState(false)

  const bottomRef = useRef<HTMLDivElement>(null)

  // 自动滚到底
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // 加载时获取服务端 AI 配置
  useEffect(() => {
    let cancelled = false
    setConfigLoading(true)
    getAiConfig()
      .then((res) => {
        if (cancelled) return
        const c = res.data
        setConfig(c)
        setEditUrl(c.url || DEFAULT_URL)
        setEditKey('') // 服务端返回的 key 已 mask，不填充到输入框
        setEditModel(c.model || DEFAULT_MODEL)
      })
      .catch((e) => {
        if (!cancelled) setConfigError(e.message)
      })
      .finally(() => {
        if (!cancelled) setConfigLoading(false)
      })
    return () => { cancelled = true }
  }, [])

  // 当开关打开时加载 design 列表
  useEffect(() => {
    if (!referDomEnabled) return
    let cancelled = false
    listDesigns(1, 100)
      .then((res) => {
        if (!cancelled) setDesigns(res.items)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [referDomEnabled])

  function handleAuthError(e: unknown): boolean {
    if (e instanceof AuthError) {
      navigate('/auth', { replace: true, state: { reason: 'expired' } })
      return true
    }
    return false
  }

  async function applyConfig() {
    if (!editUrl.trim() || !editKey.trim() || !editModel.trim()) {
      setConfigError('请填写完整的 AI 配置信息')
      return
    }
    setSavingConfig(true)
    setConfigError(null)
    try {
      const res = await updateAiConfig({
        url: editUrl.trim(),
        key: editKey.trim(),
        model: editModel.trim(),
      })
      setConfig(res.data)
      setShowConfig(false)
      setError(null)
    } catch (e) {
      if (handleAuthError(e)) return
      setConfigError((e as Error).message)
    } finally {
      setSavingConfig(false)
    }
  }

  function handleReferDomToggle(checked: boolean) {
    setReferDomEnabled(checked)
    if (!checked) {
      setGeneratedDom(null)
      setDomError(null)
    }
  }

  async function handleGenerateDom() {
    if (!selectedDesignId) return
    setGeneratingDom(true)
    setDomError(null)
    setGeneratedDom(null)
    setDomWarningExpanded(false)

    try {
      const res = await generateReferDom(selectedDesignId)
      setGeneratedDom(res.data)
    } catch (e) {
      if (handleAuthError(e)) return
      setDomError((e as Error).message)
    } finally {
      setGeneratingDom(false)
    }
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

  // 是否有警告
  const hasWarning = generatedDom?.code === 'REFER_DOM_WARNING'

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">AI 聊天</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          配置 OpenAI 兼容 API，测试接口可用性。
        </p>
      </div>

      {configError && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{configError}</AlertDescription>
        </Alert>
      )}

      {error && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* ---- 配置区 ---- */}
      <Card>
        <CardContent className="pt-6">
          {configLoading ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              加载配置…
            </div>
          ) : showConfig ? (
            <div className="space-y-4">
              <div className="grid grid-cols-[1fr_1fr_1fr] gap-3">
                <div className="space-y-2">
                  <Label htmlFor="url">API 地址 (URL)</Label>
                  <Input
                    id="url"
                    type="text"
                    placeholder={DEFAULT_URL}
                    value={editUrl}
                    onChange={(e) => setEditUrl(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="key">API 密钥 (Key)</Label>
                  <Input
                    id="key"
                    type="password"
                    placeholder={config.key ? '已配置，重新输入覆盖' : 'sk-...'}
                    value={editKey}
                    onChange={(e) => setEditKey(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="model">模型名 (Model)</Label>
                  <Input
                    id="model"
                    type="text"
                    placeholder={DEFAULT_MODEL}
                    value={editModel}
                    onChange={(e) => setEditModel(e.target.value)}
                  />
                </div>
              </div>
              <div className="flex gap-2">
                <Button onClick={applyConfig} disabled={savingConfig}>
                  {savingConfig ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" />
                      保存中…
                    </>
                  ) : (
                    '应用配置'
                  )}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setEditUrl(config.url || DEFAULT_URL)
                    setEditKey('')
                    setEditModel(config.model || DEFAULT_MODEL)
                    setConfigError(null)
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
                <code className="rounded bg-muted px-1 py-0.5 text-xs">{config.url || DEFAULT_URL}</code>
                <span className="ml-3 text-muted-foreground">Model: </span>
                <code className="rounded bg-muted px-1 py-0.5 text-xs">{config.model || DEFAULT_MODEL}</code>
              </div>
              <Button variant="outline" size="sm" onClick={() => {
                setShowConfig(true)
                setEditUrl(config.url || DEFAULT_URL)
                setEditKey('')
                setEditModel(config.model || DEFAULT_MODEL)
              }}>
                修改配置
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* ---- Refer DOM 功能区 ---- */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <Label htmlFor="refer-dom-switch" className="cursor-pointer text-sm font-medium">
              AI 通过分析 Design AST/DSL 生成参考 DOM 结构
            </Label>
            <Switch
              id="refer-dom-switch"
              checked={referDomEnabled}
              onCheckedChange={handleReferDomToggle}
            />
          </div>

          {referDomEnabled && (
            <div className="mt-4 space-y-4 border-t pt-4">
              {/* Design 选择 */}
              <div className="space-y-2">
                <Label htmlFor="design-select">选择 Design AST</Label>
                <select
                  id="design-select"
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors placeholder:text-muted-foreground focus-visible:outline-hidden focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                  value={selectedDesignId}
                  onChange={(e) => {
                    setSelectedDesignId(e.target.value)
                    setGeneratedDom(null)
                    setDomError(null)
                  }}
                >
                  <option value="">-- 请选择 --</option>
                  {designs.map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.tag} ({d.id.slice(0, 8)}…)
                    </option>
                  ))}
                </select>
              </div>

              {/* 生成按钮 */}
              <Button
                onClick={handleGenerateDom}
                disabled={!selectedDesignId || generatingDom}
              >
                {generatingDom ? (
                  <>
                    <Loader2 className="mr-2 size-4 animate-spin" />
                    生成中…
                  </>
                ) : (
                  '生成参考 DOM'
                )}
              </Button>

              {/* 错误 */}
              {domError && (
                <Alert variant="destructive">
                  <AlertCircle />
                  <AlertDescription>{domError}</AlertDescription>
                </Alert>
              )}

              {/* 警告卡片（有 warning 时） */}
              {hasWarning && generatedDom && (
                <Alert variant="default" className="border-yellow-500 bg-yellow-50 dark:bg-yellow-950/20">
                  <div className="flex flex-col gap-2 w-full">
                    <button
                      type="button"
                      onClick={() => setDomWarningExpanded(!domWarningExpanded)}
                      className="flex items-center gap-2 text-sm font-medium text-yellow-800 dark:text-yellow-200 cursor-pointer"
                    >
                      {domWarningExpanded ? (
                        <ChevronDown className="size-4 shrink-0" />
                      ) : (
                        <ChevronRight className="size-4 shrink-0" />
                      )}
                      <AlertCircle className="size-4 shrink-0" />
                      DOM 生成完成，但存在以下问题
                    </button>

                    {/* 错误列表（始终可见） */}
                    <div className="text-xs text-yellow-700 dark:text-yellow-300 ml-6 space-y-0.5">
                      {generatedDom.warnings?.map((w, i) => (
                        <div key={i}>
                          行 {w.line} 列 {w.col}: {w.message}
                        </div>
                      ))}
                    </div>

                    {/* 展开后展示生成结果 */}
                    {domWarningExpanded && (
                      <div className="ml-6 mt-2 space-y-2">
                        <Label className="text-xs text-muted-foreground">生成结果（可复制）</Label>
                        <pre className="max-h-64 overflow-auto rounded-lg border bg-background p-3 text-xs leading-relaxed">
                          <code>{generatedDom.referDom}</code>
                        </pre>
                      </div>
                    )}
                  </div>
                </Alert>
              )}

              {/* 无警告时正常展示结果 */}
              {generatedDom && !hasWarning && (
                <div className="space-y-2">
                  <Label className="text-sm text-muted-foreground">生成结果（可复制）</Label>
                  <pre className="max-h-96 overflow-auto rounded-lg border bg-muted/30 p-4 text-xs leading-relaxed">
                    <code>{generatedDom.referDom}</code>
                  </pre>
                </div>
              )}
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
