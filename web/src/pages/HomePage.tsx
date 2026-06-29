import { ArrowRight, Settings, UploadCloud } from 'lucide-react'
import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

// 首页 welcome：项目简介 + 进入配置入口。
export default function HomePage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <UploadCloud className="size-5 text-muted-foreground" />
            欢迎使用 Jupi D2C
          </CardTitle>
          <CardDescription>
            本地文件直传服务的控制面板。上传 / 下载 API 与本面板共用同一端口，
            配置持久化到 <code className="rounded bg-muted px-1 py-0.5 text-xs">config.yml</code>。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            在「配置」页可调整服务端口、最大文件大小、上传目录、Worker 数与队列长度，
            以及更新访问令牌。部分配置需重启 daemon 后生效。
          </p>
          <Button asChild>
            <Link to="/setting">
              <Settings />
              前往配置
              <ArrowRight />
            </Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
