// 与 Go 配置 API (internal/api) 的契约保持一致。

export interface AppConfig {
  port: number
  uploadDir: string
  maxFileSize: number
  workerCount: number
  queueSize: number
  tokenSet: boolean
}

export interface ConfigResponse {
  config: AppConfig
  restartRequired: boolean
}

// ===== 文件树契约（与 internal/api/handlers/files.go 一致）=====

export interface FileNode {
  name: string
  type: 'dir' | 'file'
  path: string // 相对上传目录的路径，以 / 分隔；根为 ''
  children?: FileNode[]
  size?: number
  modTime?: string // RFC3339
  url?: string // /uploads/<path>
  contentType?: string
}

export interface FilesResponse {
  root: FileNode
  uploadDir: string // 解析后的绝对路径
  totalFiles: number
  totalSize: number
}

// token 为空/省略表示"保留现有值"。
export interface ConfigUpdate {
  port?: number
  uploadDir?: string
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

export async function getFiles(): Promise<FilesResponse> {
  const res = await fetch('/api/files', { headers: authHeaders() })
  if (!res.ok) throw new Error(`加载文件列表失败 (${res.status})`)
  return res.json() as Promise<FilesResponse>
}

// 清理结果（与 internal/api/handlers/files.go CleanupFiles 一致）。
export interface CleanupResponse {
  deleted: number
  freedBytes: number
  hours: number
}

// 清理 hours 小时前的旧文件（修改时间早于 now-hours）。
export async function cleanupFiles(hours: number): Promise<CleanupResponse> {
  const res = await fetch(`/api/files/cleanup?hours=${hours}`, {
    method: 'POST',
    headers: authHeaders(),
  })
  const body = (await res.json().catch(() => ({}))) as Partial<CleanupResponse> & { error?: string }
  if (!res.ok) throw new Error(body.error ?? `清理失败 (${res.status})`)
  return body as CleanupResponse
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