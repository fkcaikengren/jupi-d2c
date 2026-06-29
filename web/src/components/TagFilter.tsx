import { Check, ChevronDown, Tag, X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

// 可搜索的 tag 多选筛选器：按钮展开下拉，输入框搜索，勾选多个 tag。
// 现有 ui/ 未引入 popover/command，这里用相对定位 + 外部点击关闭自实现。
export function TagFilter({
  allTags,
  selected,
  onChange,
}: {
  allTags: string[]
  selected: string[]
  onChange: (next: string[]) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  // 点击组件外部时关闭下拉。
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return q ? allTags.filter((t) => t.toLowerCase().includes(q)) : allTags
  }, [allTags, query])

  function toggle(tag: string) {
    onChange(selected.includes(tag) ? selected.filter((t) => t !== tag) : [...selected, tag])
  }

  return (
    <div ref={ref} className="relative">
      <Button
        variant="outline"
        size="sm"
        onClick={() => setOpen((v) => !v)}
        title="按 tag 筛选"
      >
        <Tag className="size-4" />
        Tag 筛选
        {selected.length > 0 && (
          <span className="ml-1 rounded bg-primary px-1.5 text-xs text-primary-foreground">
            {selected.length}
          </span>
        )}
        <ChevronDown className="size-4" />
      </Button>

      {open && (
        <div className="absolute left-0 z-20 mt-1 w-64 rounded-md border bg-popover p-2 shadow-md">
          <Input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索 tag…"
            className="h-8"
          />

          {selected.length > 0 && (
            <button
              type="button"
              onClick={() => onChange([])}
              className="mt-2 flex w-full items-center justify-center gap-1 rounded px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            >
              <X className="size-3" />
              清除已选（{selected.length}）
            </button>
          )}

          <div className="mt-2 max-h-64 overflow-auto">
            {filtered.length === 0 ? (
              <p className="px-2 py-3 text-center text-xs text-muted-foreground">无匹配 tag</p>
            ) : (
              filtered.map((tag) => {
                const active = selected.includes(tag)
                return (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => toggle(tag)}
                    className={cn(
                      'flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground',
                      active && 'font-medium',
                    )}
                    title={tag}
                  >
                    <span
                      className={cn(
                        'flex size-4 shrink-0 items-center justify-center rounded border',
                        active ? 'border-primary bg-primary text-primary-foreground' : 'border-input',
                      )}
                    >
                      {active && <Check className="size-3" />}
                    </span>
                    <span className="truncate">{tag}</span>
                  </button>
                )
              })
            )}
          </div>
        </div>
      )}
    </div>
  )
}
