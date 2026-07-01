import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import helpMd from '@/content/help.md?raw'
import { Card, CardContent } from '@/components/ui/card'

// 帮助页：渲染 src/content/help.md 内容，复用 react-markdown + prose 渲染方案。
// 维护帮助文档只需编辑 help.md，无需改动本组件。
export default function HelpPage() {
  return (
    <div className="space-y-6">

      <Card>
        <CardContent className="py-6">
          <div className="prose prose-sm max-w-none dark:prose-invert prose-code:before:content-none prose-code:after:content-none">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{helpMd}</ReactMarkdown>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
