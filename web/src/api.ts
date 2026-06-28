// 与 Go UI API (internal/api/ui) 的契约保持一致。

export interface AppConfig {
  port: number
  adminPort: number
  uploadDir: string
  publicBaseURL: string
  maxFileSize: number
  workerCount: number
  queueSize: number
  tokenSet: boolean
}

export interface ConfigResponse {
  config: AppConfig
  restartRequired: boolean
}

// token 为空/省略表示"保留现有值"；adminPort 只读，不发送。
export interface ConfigUpdate {
  port?: number
  uploadDir?: string
  publicBaseURL?: string
  maxFileSize?: number
  workerCount?: number
  queueSize?: number
  token?: string
}

const TOKEN_KEY = 'd2c_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(t: string): void {
  localStorage.setItem(TOKEN_KEY, t)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

function authHeaders(): HeadersInit {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export async function getConfig(): Promise<ConfigResponse> {
  const res = await fetch('/api/config', { headers: authHeaders() })
  if (!res.ok) throw new Error(`加载配置失败 (${res.status})`)
  return res.json() as Promise<ConfigResponse>
}

export async function putConfig(update: ConfigUpdate): Promise<ConfigResponse> {
  const res = await fetch('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(update),
  })
  const body = (await res.json().catch(() => ({}))) as Partial<ConfigResponse> & { error?: string }
  if (!res.ok) throw new Error(body.error ?? `保存失败 (${res.status})`)
  return body as ConfigResponse
}