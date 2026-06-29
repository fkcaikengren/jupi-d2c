import { AlertCircle, CheckCircle2, FolderOpen, FolderTree, HardDrive, Files, RefreshCw, Settings, Trash2, UploadCloud } from 'lucide-react'
import { type ComponentType, type ReactNode, useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { cleanupFiles, getFiles, type FilesResponse } from '@/api'
import { FileTree } from '@/components/FileTree'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { cn, formatBytes } from '@/lib/utils'

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
  const [data, setData] = useState<FilesResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [cleaning, setCleaning] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setData(await getFiles())
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  // 清理 1 小时前（修改时间早于 now-1h）的旧文件，成功后刷新列表。
  const cleanup = useCallback(async () => {
    if (!window.confirm('确定清理 1 小时前的文件？此操作不可恢复。')) return
    setCleaning(true)
    setError(null)
    setNotice(null)
    try {
      const res = await cleanupFiles(1)
      setNotice(`已清理 ${res.deleted} 个文件，释放 ${formatBytes(res.freedBytes)}`)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '清理失败')
    } finally {
      setCleaning(false)
    }
  }, [load])

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
              <Button
                variant="outline"
                size="sm"
                onClick={() => void cleanup()}
                disabled={cleaning || loading}
                title="删除修改时间早于 1 小时前的文件"
              >
                <Trash2 className={cn('size-4', cleaning && 'animate-pulse')} />
                清理 1 小时前的文件
              </Button>
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
    </div>
  )
}
