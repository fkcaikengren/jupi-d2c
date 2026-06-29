import {
  ChevronRight,
  File,
  FileArchive,
  FileAudio,
  FileCode,
  FileText,
  FileVideo,
  Folder,
  FolderOpen,
  Image as ImageIcon,
} from 'lucide-react'
import { useState } from 'react'

import type { FileNode } from '@/api'
import { cn, formatBytes } from '@/lib/utils'

// 手风琴式上传目录树：目录可展开/折叠，子级容器带竖向引导线强化层次感，
// 图片文件显示缩略图，点击文件在新标签打开（URL 即凭据，公开可访）。
// 缩进不靠像素计算，而是靠子级容器逐层嵌套（每层一条引导线），层次自然累积。

function isImage(node: FileNode): boolean {
  return node.type === 'file' && !!node.contentType?.startsWith('image/')
}

// 按 contentType / 扩展名挑选文件图标。
function FileIcon({ node }: { node: FileNode }) {
  const ct = node.contentType ?? ''
  const ext = node.name.includes('.') ? node.name.split('.').pop()!.toLowerCase() : ''
  const cls = 'size-4 shrink-0 text-muted-foreground'

  if (ct.startsWith('image/')) return <ImageIcon className={cls} />
  if (ct.startsWith('video/')) return <FileVideo className={cls} />
  if (ct.startsWith('audio/')) return <FileAudio className={cls} />
  if (['zip', 'gz', 'tar', 'rar', '7z'].includes(ext)) return <FileArchive className={cls} />
  if (['js', 'ts', 'tsx', 'jsx', 'json', 'go', 'py', 'css', 'html', 'sh', 'yml', 'yaml'].includes(ext))
    return <FileCode className={cls} />
  if (ct.startsWith('text/') || ['txt', 'md'].includes(ext)) return <FileText className={cls} />
  return <File className={cls} />
}

function NodeRow({ node, expanded, onToggle }: {
  node: FileNode
  expanded: Set<string>
  onToggle: (path: string) => void
}) {
  if (node.type === 'dir') {
    const open = expanded.has(node.path)
    const count = node.children?.length ?? 0
    return (
      <div>
        <button
          type="button"
          onClick={() => onToggle(node.path)}
          className="flex w-full items-center gap-1.5 rounded-md py-1.5 pl-2 pr-2 text-left text-sm transition-colors hover:bg-accent"
          aria-expanded={open}
        >
          <ChevronRight
            className={cn('size-4 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')}
          />
          {open ? (
            <FolderOpen className="size-4 shrink-0 text-primary/80" />
          ) : (
            <Folder className="size-4 shrink-0 text-primary/70" />
          )}
          <span className="truncate font-medium">{node.name}</span>
          <span className="ml-auto shrink-0 text-xs text-muted-foreground">{count} 项</span>
        </button>

        {open && count > 0 && (
          // 子级整体右移并加左引导线：每嵌套一层就多一条线，形成树形层次。
          <div className="ml-4 animate-in fade-in slide-in-from-top-1 duration-200">
            <div className="border-l border-border/60 pl-1">
              {node.children!.map((child) => (
                <NodeRow key={child.path} node={child} expanded={expanded} onToggle={onToggle} />
              ))}
            </div>
          </div>
        )}
      </div>
    )
  }

  // 文件行：整行是新标签链接，左侧缩略图/图标，右侧文件大小。
  // 前导占位 span 对齐目录行的 chevron 列。
  return (
    <a
      href={node.url}
      target="_blank"
      rel="noreferrer"
      className="flex items-center gap-1.5 rounded-md py-1.5 pl-2 pr-2 text-sm transition-colors hover:bg-accent"
      title={node.name}
    >
      <span className="size-4 shrink-0" />
      {isImage(node) ? (
        <img
          src={node.url}
          alt={node.name}
          loading="lazy"
          className="size-5 shrink-0 rounded border border-border object-cover"
        />
      ) : (
        <FileIcon node={node} />
      )}
      <span className="truncate text-foreground/90">{node.name}</span>
      {typeof node.size === 'number' && (
        <span className="ml-auto shrink-0 text-xs tabular-nums text-muted-foreground">
          {formatBytes(node.size)}
        </span>
      )}
    </a>
  )
}

// FileTree 渲染根节点的所有子项（根目录本身不显示为一行）。默认全部折叠。
export function FileTree({ root }: { root: FileNode }) {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set<string>())

  function toggle(path: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  return (
    <div className="space-y-0.5">
      {(root.children ?? []).map((node) => (
        <NodeRow key={node.path} node={node} expanded={expanded} onToggle={toggle} />
      ))}
    </div>
  )
}
