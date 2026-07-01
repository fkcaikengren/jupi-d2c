import JSZip from 'jszip'

/**
 * 从 AST 中递归扫描所有资源 URL（image / vector），
 * 下载资源并生成 ZIP 包。
 *
 * 压缩包结构：
 *  │- ast.json          （URL 已重写为相对路径的 AST）
 *  │- assets/
 *       │- xxx.svg
 *       │- yyy.png
 *
 * @param astUrl   AST JSON 的公网 URL（GET /api/ast/:id）
 * @param label    下载文件命名前缀（如 design id）
 */
export async function downloadAstPackage(astUrl: string, label: string): Promise<void> {
  // 1. 拉取 AST JSON
  const res = await fetch(astUrl)
  if (!res.ok) throw new Error(`加载 AST 失败 (${res.status})`)
  const ast: unknown = await res.json()

  // 2. 扫描所有资源 URL
  const urls = collectAssetUrls(ast)

  // 3. 下载每个资源，建立 filename → blob 映射
  const assetEntries: { filename: string; blob: Blob }[] = await Promise.all(
    urls.map(async (url) => {
      const filename = extractFilename(url)
      const imgRes = await fetch(url)
      if (!imgRes.ok) throw new Error(`下载资源失败: ${url} (${imgRes.status})`)
      const blob = await imgRes.blob()
      return { filename, blob }
    }),
  )

  // 4. 深克隆 AST 并重写 URL 为相对路径
  const modifiedAst = rewriteAssetUrls(structuredClone(ast), (url) => {
    const filename = extractFilename(url)
    return `./assets/${filename}`
  })

  // 5. 构建 ZIP
  const zip = new JSZip()
  zip.file('ast.json', JSON.stringify(modifiedAst, null, 2))

  const assetsFolder = zip.folder('assets')
  if (!assetsFolder) throw new Error('创建 assets 文件夹失败')

  for (const { filename, blob } of assetEntries) {
    assetsFolder.file(filename, blob)
  }

  // 6. 触发浏览器下载
  const blob = await zip.generateAsync({ type: 'blob' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `${label}-ast-package.zip`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  // 稍后释放 blob URL
  setTimeout(() => URL.revokeObjectURL(link.href), 60_000)
}

// ---- helpers ----

const UPLOADS_URL_PATTERN = /^https?:\/\/localhost:5678\/uploads\//

/**
 * 递归遍历 AST，收集所有匹配 /uploads/ 的 url 字段值（去重）。
 * 这些 url 可能出现在 vector.url 或 image.url 等位置。
 */
function collectAssetUrls(node: unknown): string[] {
  const found = new Set<string>()

  function walk(value: unknown): void {
    if (!value || typeof value !== 'object') return

    if (Array.isArray(value)) {
      for (const item of value) walk(item)
      return
    }

    const obj = value as Record<string, unknown>

    for (const [key, val] of Object.entries(obj)) {
      if (key === 'url' && typeof val === 'string' && UPLOADS_URL_PATTERN.test(val)) {
        found.add(val)
      } else {
        walk(val)
      }
    }
  }

  walk(node)
  return [...found]
}

/** 从 URL 中提取文件名（最后一个 / 之后的部分）。 */
function extractFilename(url: string): string {
  const idx = url.lastIndexOf('/')
  return idx >= 0 ? url.slice(idx + 1) : url
}

/**
 * 深遍历 AST，将所有匹配 /uploads/ 的 url 字符串替换为新值。
 * 保留原始结构，仅修改 url 的值。
 */
function rewriteAssetUrls(node: unknown, map: (url: string) => string): unknown {
  function walk(value: unknown): unknown {
    if (!value || typeof value !== 'object') return value

    if (Array.isArray(value)) {
      return value.map(walk)
    }

    const obj = value as Record<string, unknown>
    const result: Record<string, unknown> = {}

    for (const [key, val] of Object.entries(obj)) {
      if (key === 'url' && typeof val === 'string' && UPLOADS_URL_PATTERN.test(val)) {
        result[key] = map(val)
      } else {
        result[key] = walk(val)
      }
    }

    return result
  }

  return walk(node)
}
