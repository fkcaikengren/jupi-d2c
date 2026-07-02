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

// 鉴权失效（401 未授权 / 403 禁止访问）。由页面捕获后跳回 /auth 重新输入 token。
export class AuthError extends Error {
  readonly status: number
  constructor(status: number) {
    super('登录已失效，请重新输入 token')
    this.name = 'AuthError'
    this.status = status
  }
}

// 统一请求入口：401/403 一律视为鉴权失效——清掉本地 token 并抛出 AuthError，
// 其余响应原样返回交给调用方处理。
async function request(input: RequestInfo, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401 || res.status === 403) {
    clearToken()
    throw new AuthError(res.status)
  }
  return res
}

export async function getConfig(): Promise<ConfigResponse> {
  const res = await request('/api/config', { headers: authHeaders() })
  if (!res.ok) throw new Error(`加载配置失败 (${res.status})`)
  return res.json() as Promise<ConfigResponse>
}

export async function getFiles(): Promise<FilesResponse> {
  const res = await request('/api/files', { headers: authHeaders() })
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
  const res = await request(`/api/files/cleanup?hours=${hours}`, {
    method: 'POST',
    headers: authHeaders(),
  })
  const body = (await res.json().catch(() => ({}))) as Partial<CleanupResponse> & { error?: string }
  if (!res.ok) throw new Error(body.error ?? `清理失败 (${res.status})`)
  return body as CleanupResponse
}

// ===== design 契约（与 internal/api/handlers/design.go 一致）=====

// 一条 design（一次生成的 AST 结果）的列表项元信息。
export interface DesignItem {
  id: string
  tag: string
  createdAt: number // unix 毫秒
  astUrl: string // 公开可访问的 AST JSON 地址（GET /api/ast/:id）
  referDomUrl: string // 公开可访问的 refer_dom 地址（GET /api/ast/:id/refer-dom）
}

export interface DesignListResponse {
  items: DesignItem[]
  total: number
  page: number
  pageSize: number
}

// 分页查询 design 列表（公开，按生成时间倒序）。tags 非空时按 tag 过滤。
export async function listDesigns(
  page: number,
  pageSize: number,
  tags: string[] = [],
): Promise<DesignListResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  for (const t of tags) params.append('tag', t)
  const res = await request(`/api/design?${params.toString()}`)
  const body = (await res.json().catch(() => ({}))) as Partial<DesignListResponse> & {
    error?: string
  }
  if (!res.ok) throw new Error(body.error ?? `加载 design 列表失败 (${res.status})`)
  return body as DesignListResponse
}

// 列出全部去重 tag（公开），供首页筛选下拉使用。
export async function listDesignTags(): Promise<string[]> {
  const res = await request('/api/design/tags')
  const body = (await res.json().catch(() => ({}))) as { tags?: string[]; error?: string }
  if (!res.ok) throw new Error(body.error ?? `加载 tag 列表失败 (${res.status})`)
  return body.tags ?? []
}

// 清理结果（与 internal/api/handlers/design.go CleanupDesigns 一致）。
export interface DesignCleanupResponse {
  deleted: number
  hours: number
}

// 清理 hours 小时前的 design（生成时间早于 now-hours），需鉴权。
export async function cleanupDesigns(hours: number): Promise<DesignCleanupResponse> {
  const res = await request(`/api/design/cleanup?hours=${hours}`, {
    method: 'POST',
    headers: authHeaders(),
  })
  const body = (await res.json().catch(() => ({}))) as Partial<DesignCleanupResponse> & {
    error?: string
  }
  if (!res.ok) throw new Error(body.error ?? `清理失败 (${res.status})`)
  return body as DesignCleanupResponse
}

// 拉取某个 design 的 AST JSON 原文（公开，URL 即凭据），返回格式化后的文本。
export async function getAstText(astUrl: string): Promise<string> {
  const res = await fetch(astUrl)
  if (!res.ok) throw new Error(`加载 AST 失败 (${res.status})`)
  const json = await res.json()
  return JSON.stringify(json, null, 2)
}

// ===== project scheme 契约（与 internal/api/handlers/project_scheme.go 一致）=====

export interface ReferDomResponse {
  referDom: string
  status: string
  errors: string
}

// 拉取某个 design 的 refer_dom 内容（公开，URL 即凭据），返回 refer_dom 正文及状态。
export async function getReferDomText(referDomUrl: string): Promise<ReferDomResponse> {
  const res = await fetch(referDomUrl)
  if (!res.ok) throw new Error(`加载 refer_dom 失败 (${res.status})`)
  return res.json() as Promise<ReferDomResponse>
}

// 一条项目适配方案的列表项元信息（不含 scheme markdown 大字段）。
export interface ProjectSchemeMeta {
  projectPath: string
  createdAt: number // unix 毫秒
  updatedAt: number // unix 毫秒
}

// 完整方案记录，含 scheme markdown 正文。
export interface ProjectScheme extends ProjectSchemeMeta {
  scheme: string
}

export interface ProjectSchemeListResponse {
  items: ProjectSchemeMeta[]
  total: number
  page: number
  pageSize: number
}

// 分页查询项目方案列表（公开，按更新时间倒序）。
export async function listProjectSchemes(
  page: number,
  pageSize: number,
): Promise<ProjectSchemeListResponse> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
  const res = await request(`/api/project-scheme?${params.toString()}`)
  const body = (await res.json().catch(() => ({}))) as Partial<ProjectSchemeListResponse> & {
    error?: string
  }
  if (!res.ok) throw new Error(body.error ?? `加载方案列表失败 (${res.status})`)
  return body as ProjectSchemeListResponse
}

// 按项目绝对路径拉取完整方案（含 scheme markdown）。
export async function getProjectScheme(projectPath: string): Promise<ProjectScheme> {
  const params = new URLSearchParams({ path: projectPath })
  const res = await request(`/api/project-scheme/detail?${params.toString()}`)
  const body = (await res.json().catch(() => ({}))) as Partial<ProjectScheme> & { error?: string }
  if (!res.ok) throw new Error(body.error ?? `加载方案失败 (${res.status})`)
  return body as ProjectScheme
}

export async function putConfig(update: ConfigUpdate): Promise<ConfigResponse> {
  const res = await request('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(update),
  })
  const body = (await res.json().catch(() => ({}))) as Partial<ConfigResponse> & { error?: string }
  if (!res.ok) throw new Error(body.error ?? `保存失败 (${res.status})`)
  return body as ConfigResponse
}

// ===== AI 配置契约（与 internal/api/handlers/ai_config.go 一致）=====

export interface AiConfig {
  url: string
  key: string // GET 返回时已 mask
  model: string
  updatedAt: number
}

export interface AiConfigDataResponse {
  data: AiConfig
}

// 获取服务端保存的 AI 配置（key 已 mask）。
export async function getAiConfig(): Promise<AiConfigDataResponse> {
  const res = await request('/api/ai/config', { headers: authHeaders() })
  const json = (await res.json().catch(() => ({}))) as Partial<AiConfigDataResponse> & { error?: string }
  if (!res.ok) throw new Error(json.error ?? `获取 AI 配置失败 (${res.status})`)
  return json as AiConfigDataResponse
}

export interface AiConfigInput {
  url: string
  key: string
  model: string
}

// 保存 AI 配置到服务端。
export async function updateAiConfig(body: AiConfigInput): Promise<AiConfigDataResponse> {
  const res = await request('/api/ai/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  })
  const json = (await res.json().catch(() => ({}))) as Partial<AiConfigDataResponse> & { error?: string }
  if (!res.ok) throw new Error(json.error ?? `保存 AI 配置失败 (${res.status})`)
  return json as AiConfigDataResponse
}

// ===== AI Chat 契约（与 internal/api/handlers/chat.go 一致）=====

export interface ChatMessage {
  role: string
  content: string
}

export interface ChatRequest {
  messages: ChatMessage[]
}

export interface Usage {
  promptTokens: number
  completionTokens: number
  totalTokens: number
}

export interface ChatResponse {
  message: ChatMessage
  usage?: Usage
}

export interface ChatDataResponse {
  data: ChatResponse
}

// 发送 AI 聊天请求（非流式，单次生成）。url/key/model 由服务端配置提供。
export async function aiChat(body: ChatRequest): Promise<ChatDataResponse> {
  const res = await request('/api/ai-chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  })
  const json = (await res.json().catch(() => ({}))) as Partial<ChatDataResponse> & {
    error?: string
  }
  if (!res.ok) throw new Error(json.error ?? `AI 对话失败 (${res.status})`)
  return json as ChatDataResponse
}

// ===== Refer DOM 契约（与 internal/api/handlers/design.go GenerateReferDom 一致）=====

export interface HTMLError {
  line: number
  col: number
  message: string
}

export interface ReferDomResult {
  id: string
  referDom: string
  warnings?: HTMLError[]
  code?: string
}

export interface ReferDomDataResponse {
  data: ReferDomResult
  code?: string
}

// 调用 AI 分析指定 design 的 AST/DSL 生成参考 HTML DOM 结构（需鉴权）。
// url/key/model 由服务端配置提供，无需前端传入。
export async function generateReferDom(
  designId: string,
): Promise<ReferDomDataResponse> {
  const res = await request(`/api/ast/${encodeURIComponent(designId)}/refer-dom`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
  })
  const json = (await res.json().catch(() => ({}))) as Partial<ReferDomDataResponse> & {
    error?: string
  }
  if (!res.ok) throw new Error(json.error ?? `生成参考 DOM 失败 (${res.status})`)
  return json as ReferDomDataResponse
}