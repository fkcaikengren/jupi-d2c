import { useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  clearToken,
  getConfig,
  getToken,
  putConfig,
  setToken,
  type AppConfig,
  type ConfigUpdate,
} from './api'

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

export default function App() {
  // 首次渲染时从 localStorage 读 token；有就直接进主 UI，没有则显示输入框。
  const [token, setTokenState] = useState<string | null>(() => getToken())

  if (!token) {
    return <TokenGate onSubmit={(t) => { setToken(t); setTokenState(t) }} />
  }

  return <Panel token={token} onLogout={() => { clearToken(); setTokenState(null) }} />
}

// ===== Token 输入页 =====

function TokenGate({ onSubmit }: { onSubmit: (token: string) => void }) {
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  function submit(e: FormEvent) {
    e.preventDefault()
    const t = value.trim()
    if (!t) {
      setError('请输入 token')
      return
    }
    onSubmit(t)
  }

  return (
    <div className="min-h-screen bg-slate-50 text-slate-800">
      <div className="mx-auto max-w-md px-4 py-16">
        <header className="mb-8">
          <h1 className="text-2xl font-semibold text-slate-900">Jupi D2C</h1>
          <p className="mt-1 text-sm text-slate-500">
            本地控制面板 · 首次访问请输入 <code className="rounded bg-slate-200 px-1 py-0.5">TOKEN</code>。
          </p>
        </header>

        <form onSubmit={submit} className="space-y-4 rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">访问令牌 (TOKEN)</span>
            <input
              type="password"
              autoFocus
              value={value}
              onChange={(e) => { setValue(e.target.value); setError(null) }}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm shadow-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
              placeholder="请输入 token"
            />
            <span className="mt-1 block text-xs text-slate-400">
              token 仅保存在浏览器 localStorage 中，不会上传。
            </span>
          </label>

          {error && (
            <div className="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </div>
          )}

          <button
            type="submit"
            className="w-full rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-700"
          >
            进入面板
          </button>
        </form>
      </div>
    </div>
  )
}

// ===== 主面板 =====

function Panel({ token, onLogout }: { token: string; onLogout: () => void }) {
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
      // 401 通常是 token 被改了或失效——回到登录页。
      if (msg.includes('401')) onLogout()
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
    <div className="min-h-screen bg-slate-50 text-slate-800">
      <div className="mx-auto max-w-2xl px-4 py-10">
        <header className="mb-8 flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-slate-900">Jupi D2C</h1>
            <p className="mt-1 text-sm text-slate-500">
              本地控制面板 · 配置保存到 <code className="rounded bg-slate-200 px-1 py-0.5">config.yml</code>，重启后生效。
            </p>
          </div>
          <button
            type="button"
            onClick={onLogout}
            title={`已登录 token: ${token.slice(0, 4)}…`}
            className="shrink-0 rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:bg-slate-100"
          >
            更换 token
          </button>
        </header>

        {restartRequired && (
          <div className="mb-6 rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            ⚠️ 已保存的配置与正在运行的实例不一致，需<strong>重启 daemon</strong> 才能生效。
          </div>
        )}
        {error && (
          <div className="mb-6 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}
        {notice && (
          <div className="mb-6 rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {notice}
          </div>
        )}

        {loading || !form || !config ? (
          <p className="text-sm text-slate-500">加载中…</p>
        ) : (
          <form
            onSubmit={onSubmit}
            className="space-y-5 rounded-xl border border-slate-200 bg-white p-6 shadow-sm"
          >
            <Field label="端口 (PORT)" hint="服务监听端口（上传 API / 配置面板 / 文件访问共用）；修改后需重启。">
              <input
                type="number"
                min={1}
                max={65535}
                value={form.port}
                onChange={(e) => update('port', e.target.value)}
                className={inputClass}
              />
            </Field>

            <Field label="最大文件大小 (MAX_FILE_SIZE)" hint={`字节；当前 ≈ ${formatMB(form.maxFileSize)}`}>
              <input
                type="number"
                min={1}
                value={form.maxFileSize}
                onChange={(e) => update('maxFileSize', e.target.value)}
                className={inputClass}
              />
            </Field>

            <Field
              label="上传目录 (UPLOAD_DIR)"
              help="相对路径相对于 ~/.jupi-d2c/ 目录解析；也可填写绝对路径。"
              hint="变更后旧文件仍留在原目录，其 URL 可能失效。"
            >
              <input
                type="text"
                value={form.uploadDir}
                onChange={(e) => update('uploadDir', e.target.value)}
                className={inputClass}
              />
            </Field>

            <div className="grid grid-cols-2 gap-4">
              <Field label="Worker 数 (WORKER_COUNT)">
                <input
                  type="number"
                  min={1}
                  value={form.workerCount}
                  onChange={(e) => update('workerCount', e.target.value)}
                  className={inputClass}
                />
              </Field>
              <Field label="队列长度 (QUEUE_SIZE)">
                <input
                  type="number"
                  min={1}
                  value={form.queueSize}
                  onChange={(e) => update('queueSize', e.target.value)}
                  className={inputClass}
                />
              </Field>
            </div>

            <Field
              label="访问令牌 (TOKEN)"
              hint={config.tokenSet ? '已设置。留空表示保留当前值，填写则更新。' : '尚未设置，请填写。'}
            >
              <input
                type="password"
                value={form.token}
                placeholder={config.tokenSet ? '••••••••（保留现有）' : '请输入令牌'}
                onChange={(e) => update('token', e.target.value)}
                className={inputClass}
              />
            </Field>

            <div className="flex items-center gap-3 pt-2">
              <button
                type="submit"
                disabled={saving || !dirty}
                className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {saving ? '保存中…' : '保存配置'}
              </button>
              <button
                type="button"
                onClick={() => void load()}
                disabled={saving}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 transition hover:bg-slate-100 disabled:opacity-40"
              >
                重置
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

const inputClass =
  'w-full rounded-lg border border-slate-300 px-3 py-2 text-sm shadow-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200'

function Field({
  label,
  hint,
  help,
  children,
}: {
  label: string
  hint?: string
  help?: string
  children: React.ReactNode
}) {
  return (
    <label className="block">
      <span className="mb-1 flex items-center gap-1 text-sm font-medium text-slate-700">
        {label}
        {help && (
          <span
            title={help}
            aria-label={help}
            className="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-slate-300 text-[10px] font-normal leading-none text-slate-400 transition hover:border-slate-400 hover:text-slate-600"
          >
            ?
          </span>
        )}
      </span>
      {children}
      {hint && <span className="mt-1 block text-xs text-slate-400">{hint}</span>}
    </label>
  )
}