// 首页：暂为空白占位，后续用于查询其他信息。
export default function HomePage() {
  return (
    <div className="flex flex-col items-center gap-2 py-24 text-center">
      <p className="text-sm font-medium">Jupi D2C</p>
      <p className="max-w-sm text-xs text-muted-foreground">
        此处后续用于查询其他信息。从左上角菜单进入「文件」查看上传内容。
      </p>
    </div>
  )
}
