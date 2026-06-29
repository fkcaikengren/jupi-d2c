import { AlertCircle, CheckCircle2, ChevronDown, FolderOpen, FolderTree, HardDrive, Files, RefreshCw, Settings, Trash2, UploadCloud } from 'lucide-react'
import { type ComponentType, type ReactNode, useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import { AuthError, cleanupFiles, getFiles, type FilesResponse } from '@/api'
import { FileTree } from '@/components/FileTree'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn, formatBytes } from '@/lib/utils'

// 清理选项：删除修改时间早于 N 天前的文件（API 以小时为单位，故 days * 24）。
const CLEANUP_OPTIONS = [
  { days: 3, label: '清理 3 天前' },
  { days: 7, label: '清理 7 天前' },
  { days: 30, label: '清理 30 天前' },
] as const

// 信息标签：圆角胶囊 + 前置图标，用于展示目录 / 文件数 / 总大小等摘要信息。
function Badge({
  icon: Icon,
  title,
  children,
}: {
  icon: ComponentType<{ className?: string }>
  title?: string
  children: ReactNode
}) {
  return (
    <span
      title={title}
      className="inline-flex max-w-full items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground"
    >
      <Icon className="size-3 shrink-0" />
      <span className="truncate">{children}</span>
    </span>
  )
}

// 首页：可视化展示上传目录与内容（手风琴文件树）。
export default function HomePage() {
  const navigate = useNavigate()
  const [data, setData] = useState<FilesResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [cleaning, setCleaning] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  // 待确认的清理天数；非 null 时弹出二次确认对话框。
  const [pendingDays, setPendingDays] = useState<number | null>(null)

  // 鉴权失效：跳回 /auth 提示重新输入 token；其余错误交给调用方展示。返回是否已处理。
  const handleAuthError = useCallback(
    (e: unknown): boolean => {
      if (e instanceof AuthError) {
        navigate('/auth', { replace: true, state: { reason: 'expired' } })
        return true
      }
      return false
    },
    [navigate],
  )

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setData(await getFiles())
    } catch (e) {
      if (handleAuthError(e)) return
      setError(e instanceof Error ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }, [handleAuthError])

  // 清理修改时间早于 N 天前的旧文件，成功后刷新列表。由二次确认对话框触发。
  const cleanup = useCallback(
    async (days: number) => {
      setCleaning(true)
      setError(null)
      setNotice(null)
      try {
        const res = await cleanupFiles(days * 24)
        setNotice(`已清理 ${res.deleted} 个文件，释放 ${formatBytes(res.freedBytes)}`)
        await load()
      } catch (e) {
        if (handleAuthError(e)) return
        setError(e instanceof Error ? e.message : '清理失败')
      } finally {
        setCleaning(false)
      }
    },
    [load, handleAuthError],
  )

  // 确认对话框点击「确认」：取出待确认天数、关闭对话框并执行清理。
  function confirmCleanup() {
    if (pendingDays == null) return
    const days = pendingDays
    setPendingDays(null)
    void cleanup(days)
  }

  useEffect(() => {
    void load()
  }, [load])

  const empty = !!data && data.totalFiles === 0

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <FolderTree className="size-5 text-muted-foreground" />
            上传文件
          </CardTitle>
          {data ? (
            <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
              <Badge icon={FolderOpen} title={data.uploadDir}>
                {data.uploadDir}
              </Badge>
              <Badge icon={Files}>{data.totalFiles} 个文件</Badge>
              <Badge icon={HardDrive}>{formatBytes(data.totalSize)}</Badge>
            </div>
          ) : (
            <CardDescription>可视化浏览上传目录下的全部文件，目录可逐层展开。</CardDescription>
          )}
          <CardAction>
            <div className="flex items-center gap-2">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={cleaning || loading}
                    title="按时间清理旧文件"
                  >
                    <Trash2 className={cn('size-4', cleaning && 'animate-pulse')} />
                    清理旧文件
                    <ChevronDown className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {CLEANUP_OPTIONS.map((o) => (
                    <DropdownMenuItem
                      key={o.days}
                      onSelect={() => setPendingDays(o.days)}
                    >
                      {o.label}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              <Button
                variant="outline"
                size="icon"
                onClick={() => void load()}
                disabled={loading}
                title="刷新"
                aria-label="刷新"
              >
                <RefreshCw className={cn('size-4', loading && 'animate-spin')} />
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
              <AlertTitle>加载文件列表失败</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : loading && !data ? (
            <p className="py-8 text-center text-sm text-muted-foreground">加载中…</p>
          ) : empty ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <UploadCloud className="size-10 text-muted-foreground/60" />
              <p className="text-sm font-medium">暂无上传文件</p>
              <p className="max-w-sm text-xs text-muted-foreground">
                通过 <code className="rounded bg-muted px-1 py-0.5">POST /api/upload</code>{' '}
                上传后，文件将按目录在此展示。
              </p>
            </div>
          ) : (
            data && <FileTree root={data.root} />
          )}
        </CardContent>
      </Card>

      <div className="flex justify-center">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/setting">
            <Settings />
            前往配置
          </Link>
        </Button>
      </div>

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
              将删除修改时间早于 {pendingDays} 天前的全部文件，此操作不可恢复。是否继续？
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
