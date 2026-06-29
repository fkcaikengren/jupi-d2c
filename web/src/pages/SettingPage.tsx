import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { AlertCircle, AlertTriangle, CheckCircle2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

import {
  clearToken,
  getConfig,
  putConfig,
  type AppConfig,
  type ConfigUpdate,
} from '@/api'
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

type FormState = {
  port: string
  maxFileSize: string
  workerCount: string
  queueSize: string
  uploadDir: string
  token: string
}

function toForm(c: AppConfig): FormState {
  return {
    port: String(c.port),
    maxFileSize: String(c.maxFileSize),
    workerCount: String(c.workerCount),
    queueSize: String(c.queueSize),
    uploadDir: c.uploadDir,
    token: '',
  }
}

function formatMB(bytes: string): string {
  const n = Number(bytes)
  if (!Number.isFinite(n) || n <= 0) return '—'
  return `${(n / (1024 * 1024)).toFixed(2)} MB`
}

// 配置页：加载 / 保存 config.yml。业务逻辑沿用原 Panel，仅替换为 shadcn 组件。
export default function SettingPage() {
  const navigate = useNavigate()
  const [config, setConfig] = useState<AppConfig | null>(null)
  const [form, setForm] = useState<FormState | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [restartRequired, setRestartRequired] = useState(false)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const res = await getConfig()
      setConfig(res.config)
      setForm(toForm(res.config))
      setRestartRequired(res.restartRequired)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  function update<K extends keyof FormState>(key: K, value: string) {
    setForm((f) => (f ? { ...f, [key]: value } : f))
    setNotice(null)
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!form) return
    setSaving(true)
    setError(null)
    setNotice(null)
    try {
      const payload: ConfigUpdate = {
        port: Number(form.port),
        maxFileSize: Number(form.maxFileSize),
        workerCount: Number(form.workerCount),
        queueSize: Number(form.queueSize),
        uploadDir: form.uploadDir,
      }
      if (form.token.trim() !== '') payload.token = form.token.trim()

      const res = await putConfig(payload)
      setConfig(res.config)
      setForm(toForm(res.config))
      setRestartRequired(res.restartRequired)
      setNotice('配置已保存到 config.yml')
    } catch (e) {
      const msg = (e as Error).message
      // 401 通常是 token 被改了或失效——清掉并回到鉴权页。
      if (msg.includes('401')) {
        clearToken()
        navigate('/auth', { replace: true })
        return
      }
      setError(msg)
    } finally {
      setSaving(false)
    }
  }

  const dirty = useMemo(() => {
    if (!config || !form) return false
    return (
      Number(form.port) !== config.port ||
      Number(form.maxFileSize) !== config.maxFileSize ||
      Number(form.workerCount) !== config.workerCount ||
      Number(form.queueSize) !== config.queueSize ||
      form.uploadDir !== config.uploadDir ||
      form.token.trim() !== ''
    )
  }, [config, form])

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">配置</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          配置保存到{' '}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">config.yml</code>，
          部分项重启后生效。
        </p>
      </div>

      {restartRequired && (
        <Alert variant="warning">
          <AlertTriangle />
          <AlertDescription>
            已保存的配置与正在运行的实例不一致，需<strong>重启 daemon</strong> 才能生效。
          </AlertDescription>
        </Alert>
      )}
      {error && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {notice && (
        <Alert variant="success">
          <CheckCircle2 />
          <AlertDescription>{notice}</AlertDescription>
        </Alert>
      )}

      {loading || !form || !config ? (
        <p className="text-sm text-muted-foreground">加载中…</p>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>服务配置</CardTitle>
            <CardDescription>修改后点击「保存配置」写入磁盘。</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={onSubmit} className="space-y-5">
              <Field
                htmlFor="port"
                label="端口 (PORT)"
                hint="服务监听端口（上传 API / 配置面板 / 文件访问共用）；修改后需重启。"
              >
                <Input
                  id="port"
                  type="number"
                  min={1}
                  max={65535}
                  value={form.port}
                  onChange={(e) => update('port', e.target.value)}
                />
              </Field>

              <Field
                htmlFor="maxFileSize"
                label="最大文件大小 (MAX_FILE_SIZE)"
                hint={`字节；当前 ≈ ${formatMB(form.maxFileSize)}`}
              >
                <Input
                  id="maxFileSize"
                  type="number"
                  min={1}
                  value={form.maxFileSize}
                  onChange={(e) => update('maxFileSize', e.target.value)}
                />
              </Field>

              <Field
                htmlFor="uploadDir"
                label="上传目录 (UPLOAD_DIR)"
                help="相对路径相对于 ~/.jupi-d2c/ 目录解析；也可填写绝对路径。"
                hint="变更后旧文件仍留在原目录，其 URL 可能失效。"
              >
                <Input
                  id="uploadDir"
                  type="text"
                  value={form.uploadDir}
                  onChange={(e) => update('uploadDir', e.target.value)}
                />
              </Field>

              <div className="grid grid-cols-2 gap-4">
                <Field htmlFor="workerCount" label="Worker 数 (WORKER_COUNT)">
                  <Input
                    id="workerCount"
                    type="number"
                    min={1}
                    value={form.workerCount}
                    onChange={(e) => update('workerCount', e.target.value)}
                  />
                </Field>
                <Field htmlFor="queueSize" label="队列长度 (QUEUE_SIZE)">
                  <Input
                    id="queueSize"
                    type="number"
                    min={1}
                    value={form.queueSize}
                    onChange={(e) => update('queueSize', e.target.value)}
                  />
                </Field>
              </div>

              <Field
                htmlFor="newToken"
                label="访问令牌 (TOKEN)"
                hint={
                  config.tokenSet
                    ? '已设置。留空表示保留当前值，填写则更新。'
                    : '尚未设置，请填写。'
                }
              >
                <Input
                  id="newToken"
                  type="password"
                  value={form.token}
                  placeholder={config.tokenSet ? '••••••••（保留现有）' : '请输入令牌'}
                  onChange={(e) => update('token', e.target.value)}
                />
              </Field>

              <div className="flex items-center gap-3 pt-2">
                <Button type="submit" disabled={saving || !dirty}>
                  {saving ? '保存中…' : '保存配置'}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void load()}
                  disabled={saving}
                >
                  重置
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function Field({
  htmlFor,
  label,
  hint,
  help,
  children,
}: {
  htmlFor: string
  label: string
  hint?: string
  help?: string
  children: ReactNode
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={htmlFor}>
        {label}
        {help && (
          <span
            title={help}
            aria-label={help}
            className="inline-flex size-4 cursor-help items-center justify-center rounded-full border text-[10px] font-normal leading-none text-muted-foreground transition hover:text-foreground"
          >
            ?
          </span>
        )}
      </Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}
