import {
  AlertCircle,
  Check,
  CheckCircle2,
  ChevronDown,
  Copy,
  ExternalLink,
  Eye,
  FileJson,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { cleanupDesigns, type DesignItem, getAstText, listDesignTags, listDesigns } from '@/api'
import { TagFilter } from '@/components/TagFilter'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Drawer } from '@/components/ui/drawer'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const PAGE_SIZE = 20

// 清理选项：删除生成时间早于 N 天前的 design（API 以小时为单位，故 days * 24）。
const CLEANUP_OPTIONS = [
  { days: 3, label: '清理 3 天前' },
  { days: 7, label: '清理 7 天前' },
  { days: 30, label: '清理 30 天前' },
] as const

// 把 unix 毫秒格式化为本地时间字符串。
function formatTime(ms: number): string {
  return new Date(ms).toLocaleString('zh-CN', { hour12: false })
}

// 小复制按钮：复制文本到剪贴板并短暂反馈。
function CopyButton({ text, title }: { text: string; title?: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <Button
      variant="ghost"
      size="icon"
      className="size-7"
      title={title ?? '复制'}
      onClick={() => {
        void navigator.clipboard.writeText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 1500)
      }}
    >
      {copied ? <Check className="size-3.5 text-green-600" /> : <Copy className="size-3.5" />}
    </Button>
  )
}

// 首页：分页查询保存的 design，支持 tag 筛选与按时间清理，表格按生成时间倒序，
// 点「查看 AST」打开右侧抽屉。
export default function HomePage() {
  const [data, setData] = useState<DesignItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  // tag 筛选：全部可选 tag 与当前已选。
  const [allTags, setAllTags] = useState<string[]>([])
  const [selectedTags, setSelectedTags] = useState<string[]>([])

  // 待确认的清理天数；非 null 时弹出二次确认对话框。
  const [pendingDays, setPendingDays] = useState<number | null>(null)
  const [cleaning, setCleaning] = useState(false)

  // 抽屉状态：当前查看的 design 及其 AST 文本。
  const [active, setActive] = useState<DesignItem | null>(null)
  const [astText, setAstText] = useState('')
  const [astLoading, setAstLoading] = useState(false)
  const [astError, setAstError] = useState<string | null>(null)

  // load 依赖 selectedTags：选中项变化时其身份变化，触发下方 effect 重新拉取第 1 页。
  const load = useCallback(
    async (p: number) => {
      setLoading(true)
      setError(null)
      try {
        const res = await listDesigns(p, PAGE_SIZE, selectedTags)
        setData(res.items)
        setTotal(res.total)
        setPage(res.page)
      } catch (e) {
        setError(e instanceof Error ? e.message : '加载失败')
      } finally {
        setLoading(false)
      }
    },
    [selectedTags],
  )

  const refreshTags = useCallback(async () => {
    try {
      setAllTags(await listDesignTags())
    } catch {
      // tag 列表加载失败不阻断主流程，筛选器留空即可。
    }
  }, [])

  useEffect(() => {
    void load(1)
  }, [load])

  useEffect(() => {
    void refreshTags()
  }, [refreshTags])

  // 打开抽屉并拉取 AST。
  const viewAst = useCallback(async (item: DesignItem) => {
    setActive(item)
    setAstText('')
    setAstError(null)
    setAstLoading(true)
    try {
      setAstText(await getAstText(item.astUrl))
    } catch (e) {
      setAstError(e instanceof Error ? e.message : '加载 AST 失败')
    } finally {
      setAstLoading(false)
    }
  }, [])

  // 确认清理：删除 N 天前的 design，成功后刷新列表与 tag 列表。
  async function confirmCleanup() {
    if (pendingDays == null) return
    const days = pendingDays
    setPendingDays(null)
    setCleaning(true)
    setError(null)
    setNotice(null)
    try {
      const res = await cleanupDesigns(days * 24)
      setNotice(`已清理 ${res.deleted} 条 design`)
      await Promise.all([load(1), refreshTags()])
    } catch (e) {
      setError(e instanceof Error ? e.message : '清理失败')
    } finally {
      setCleaning(false)
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const empty = !loading && data.length === 0

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <FileJson className="size-5 text-muted-foreground" />
            AST Design
          </CardTitle>
          <CardDescription>查询插件同步保存的 AST，共 {total} 条，按生成时间倒序。</CardDescription>
          <CardAction>
            <div className="flex items-center gap-2">
              <TagFilter allTags={allTags} selected={selectedTags} onChange={setSelectedTags} />
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={cleaning || loading}
                    title="按时间清理历史 design"
                  >
                    <Trash2 className={cleaning ? 'size-4 animate-pulse' : 'size-4'} />
                    清理历史数据
                    <ChevronDown className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {CLEANUP_OPTIONS.map((o) => (
                    <DropdownMenuItem key={o.days} onSelect={() => setPendingDays(o.days)}>
                      {o.label}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              <Button
                variant="outline"
                size="icon"
                onClick={() => void load(page)}
                disabled={loading}
                title="刷新"
                aria-label="刷新"
              >
                <RefreshCw className={loading ? 'size-4 animate-spin' : 'size-4'} />
              </Button>
            </div>
          </CardAction>
        </CardHeader>

        <CardContent>
          {notice && (
            <Alert variant="success" className="mb-4">
              <CheckCircle2 />
              <AlertTitle>清理完成</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          )}
          {error ? (
            <Alert variant="destructive">
              <AlertCircle />
              <AlertTitle>加载 design 列表失败</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : loading && data.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">加载中…</p>
          ) : empty ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <FileJson className="size-10 text-muted-foreground/60" />
              <p className="text-sm font-medium">
                {selectedTags.length > 0 ? '当前筛选无 design' : '暂无 design'}
              </p>
              <p className="max-w-sm text-xs text-muted-foreground">
                在插件设置中开启「同步 AST」并指向{' '}
                <code className="rounded bg-muted px-1 py-0.5">POST /api/design</code>，生成 AST 后将在此展示。
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="px-2 py-2 font-medium">访问该 AST 的 URL</th>
                    <th className="px-2 py-2 font-medium whitespace-nowrap">生成时间</th>
                    <th className="px-2 py-2 font-medium text-right">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {data.map((item) => (
                    <tr key={item.id} className="border-b last:border-0 hover:bg-muted/40">
                      <td className="px-2 py-2">
                        <div className="flex items-center gap-1">
                          <a
                            href={item.astUrl}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1 truncate font-mono text-xs text-primary hover:underline"
                            title={item.astUrl}
                          >
                            <span className="truncate">{item.astUrl}</span>
                            <ExternalLink className="size-3 shrink-0" />
                          </a>
                          <CopyButton text={item.astUrl} title="复制 URL" />
                        </div>
                        <div className="mt-0.5 truncate text-xs text-muted-foreground" title={item.tag}>
                          tag: {item.tag}
                        </div>
                      </td>
                      <td className="px-2 py-2 whitespace-nowrap text-muted-foreground">
                        {formatTime(item.createdAt)}
                      </td>
                      <td className="px-2 py-2 text-right">
                        <Button variant="outline" size="sm" onClick={() => void viewAst(item)}>
                          <Eye className="size-4" />
                          查看 AST
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              {/* 分页 */}
              <div className="mt-4 flex items-center justify-between text-sm">
                <span className="text-xs text-muted-foreground">
                  第 {page} / {totalPages} 页
                </span>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page <= 1 || loading}
                    onClick={() => void load(page - 1)}
                  >
                    上一页
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page >= totalPages || loading}
                    onClick={() => void load(page + 1)}
                  >
                    下一页
                  </Button>
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Drawer
        open={active !== null}
        onClose={() => setActive(null)}
        title={
          <span className="flex items-center gap-2">
            <FileJson className="size-4 text-muted-foreground" />
            AST · {active?.tag}
            {astText && <CopyButton text={astText} title="复制 AST" />}
          </span>
        }
      >
        {astLoading ? (
          <p className="py-8 text-center text-sm text-muted-foreground">加载中…</p>
        ) : astError ? (
          <Alert variant="destructive">
            <AlertCircle />
            <AlertTitle>加载 AST 失败</AlertTitle>
            <AlertDescription>{astError}</AlertDescription>
          </Alert>
        ) : (
          <pre className="rounded-md bg-muted p-3 text-xs leading-relaxed whitespace-pre-wrap break-all">
            {astText}
          </pre>
        )}
      </Drawer>

      <AlertDialog
        open={pendingDays !== null}
        onOpenChange={(open) => {
          if (!open) setPendingDays(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认清理</AlertDialogTitle>
            <AlertDialogDescription>
              将删除生成时间早于 {pendingDays} 天前的全部 design，此操作不可恢复。是否继续？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: 'destructive' })}
              onClick={confirmCleanup}
            >
              确认清理
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
