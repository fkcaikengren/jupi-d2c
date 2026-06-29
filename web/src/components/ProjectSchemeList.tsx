import { AlertCircle, FileText, FolderCog, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { getProjectScheme, listProjectSchemes, type ProjectSchemeMeta } from '@/api'
import { CopyButton } from '@/components/CopyButton'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Drawer } from '@/components/ui/drawer'
import { formatTime } from '@/lib/utils'

const PAGE_SIZE = 8

// 分页查询保存的项目适配方案，按更新时间倒序，点「查看方案」打开右侧抽屉渲染 markdown。
export function ProjectSchemeList() {
  const [data, setData] = useState<ProjectSchemeMeta[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // 抽屉状态：当前查看的方案路径及其 markdown 正文。
  const [active, setActive] = useState<ProjectSchemeMeta | null>(null)
  const [scheme, setScheme] = useState('')
  const [schemeLoading, setSchemeLoading] = useState(false)
  const [schemeError, setSchemeError] = useState<string | null>(null)

  const load = useCallback(async (p: number) => {
    setLoading(true)
    setError(null)
    try {
      const res = await listProjectSchemes(p, PAGE_SIZE)
      setData(res.items)
      setTotal(res.total)
      setPage(res.page)
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(1)
  }, [load])

  // 打开抽屉并拉取完整方案 markdown。
  const viewScheme = useCallback(async (item: ProjectSchemeMeta) => {
    setActive(item)
    setScheme('')
    setSchemeError(null)
    setSchemeLoading(true)
    try {
      const res = await getProjectScheme(item.projectPath)
      setScheme(res.scheme)
    } catch (e) {
      setSchemeError(e instanceof Error ? e.message : '加载方案失败')
    } finally {
      setSchemeLoading(false)
    }
  }, [])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const empty = !loading && data.length === 0

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <FolderCog className="size-5 text-muted-foreground" />
            Project Scheme 列表
          </CardTitle>
          <CardAction>
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
          </CardAction>
        </CardHeader>

        <CardContent>
          {error ? (
            <Alert variant="destructive">
              <AlertCircle />
              <AlertTitle>加载方案列表失败</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : loading && data.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">加载中…</p>
          ) : empty ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <FolderCog className="size-10 text-muted-foreground/60" />
              <p className="text-sm font-medium">暂无项目方案</p>
              <p className="max-w-sm text-xs text-muted-foreground">
                通过 MCP 工具{' '}
                <code className="rounded bg-muted px-1 py-0.5">save_project_scheme</code>{' '}
                保存项目适配方案后将在此展示。
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="px-2 py-2 font-medium">项目路径</th>
                    <th className="px-2 py-2 font-medium whitespace-nowrap">更新时间</th>
                    <th className="px-2 py-2 font-medium text-right">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {data.map((item) => (
                    <tr
                      key={item.projectPath}
                      className="border-b last:border-0 hover:bg-muted/40"
                    >
                      <td className="px-2 py-2">
                        <div className="flex items-center gap-1">
                          <span
                            className="truncate font-mono text-xs"
                            title={item.projectPath}
                          >
                            {item.projectPath}
                          </span>
                          <CopyButton text={item.projectPath} title="复制路径" />
                        </div>
                      </td>
                      <td className="px-2 py-2 whitespace-nowrap text-muted-foreground">
                        {formatTime(item.updatedAt)}
                      </td>
                      <td className="px-2 py-2 text-right">
                        <Button variant="outline" size="sm" onClick={() => void viewScheme(item)}>
                          <FileText className="size-4" />
                          查看方案
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              {/* 分页 */}
              <div className="mt-4 flex items-center justify-between text-sm">
                <span className="text-xs text-muted-foreground">
                  第 {page} / {totalPages} 页，共 {total} 条
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
            <FileText className="size-4 text-muted-foreground" />
            <span className="truncate font-mono text-xs" title={active?.projectPath}>
              {active?.projectPath}
            </span>
            {scheme && <CopyButton text={scheme} title="复制方案" />}
          </span>
        }
      >
        {schemeLoading ? (
          <p className="py-8 text-center text-sm text-muted-foreground">加载中…</p>
        ) : schemeError ? (
          <Alert variant="destructive">
            <AlertCircle />
            <AlertTitle>加载方案失败</AlertTitle>
            <AlertDescription>{schemeError}</AlertDescription>
          </Alert>
        ) : (
          <div className="prose prose-sm max-w-none dark:prose-invert">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{scheme}</ReactMarkdown>
          </div>
        )}
      </Drawer>
    </>
  )
}
