# 帮助手册

本软件的作用是 配合 「橘皮」 插件（master/figma插件）获取设计稿的信息，然后通过给 AI agent提供 MCP 服务，帮助研发完成界面自动化生成。

```
  ┌──────────────┐   上传图片 / 同步 AST   ┌──────────────┐   MCP（取 AST / 适配方案）   ┌──────────────┐
  │  橘皮 插件     │ ──────────────────────▶ │  jupi-d2c     │ ◀───────────────────────── │   AI agent   │
  │ master/figma │      HTTP（带 token）    │ 本地服务/存储  │      Streamable HTTP        │  (Claude 等) │
  └──────────────┘                         └──────────────┘                             └──────┬───────┘
       设计稿                                  图片 + AST + 方案                            生成前端代码
```

- **橘皮插件**：在设计稿侧解析出图片与 UI AST，推送给 jupi-d2c（见下「暴露给插件的接口」）。
- **jupi-d2c**：本地落盘图片、持久化 AST 与项目适配方案，并把这些能力暴露为 MCP 工具。
- **AI agent**：通过 MCP 取回 AST 与适配方案，还原成前端代码（见下「MCP 服务」）。

## 一、暴露给 「橘皮」 插件的接口

插件向本服务推送两类数据，全部走 HTTP（默认 `http://localhost:5678`）：

### 1. 上传图片 — `POST /api/upload`

把设计稿切图 / 图标落盘，返回可公开访问的 URL。

- 请求体：原始二进制，或 `multipart/form-data`（字段名 `file`）。
- 可选 `tag`（表单字段或 query 参数）：把文件归档到 `/uploads/<tag>/` 子目录。
- 响应：`{ "data": { "filename", "url", "size", "contentType" } }`，其中 `url` 即 `GET /uploads/...` 的公开地址。

### 2. 同步 AST — `POST /api/design`

把插件解析出的设计稿 UI AST（JSON 格式的 DSL）保存为一条 design，供后续 MCP 的 `query_ast` 取用。

- 请求体：`{ "tag": "<可选标识>", "ast": <任意 JSON，对象或字符串> }`。
- 响应：`{ "data": { "id", "tag", "createdAt", "astUrl" } }`。
- **`id` 是关键**：把它交给 AI（见下文使用范例），AI 凭 `id` 调用 `query_ast` 取回 AST 生成代码。`astUrl` 则是该 AST 的公开访问地址（URL 即凭据）。

### 3.鉴权

以上两个接口的请求头都需要携带 `~/.jupi-d2c/config.yml` 里的 `token`，三种写法任选其一（服务端用恒定时间比较）：

```
Authorization: Bearer <token>
Authorization: <token>
PRIVATE-TOKEN: <token>
```


## 二、MCP 服务

本服务把自身暴露为一个 MCP（Model Context Protocol）服务，走 **Streamable HTTP**，单端点挂在 `http://localhost:5678/mcp`（**公开、无需 token**——方案分析由 AI 端完成，本服务只做持久化）。AI agent 通过它取回 AST、复用/保存项目适配方案，从而把设计稿还原成前端代码。

### 1.在 Claude 中安装

确保 daemon 已启动（`jupi-d2c start`），然后用 Claude Code CLI 添加：

```bash
claude mcp add --transport http jupi-d2c http://localhost:5678/mcp
```

添加后用 `claude mcp list` 确认连接正常；在会话中即可调用下列工具。

> 其他支持 Streamable HTTP 的 MCP 客户端同理，把服务地址配为 `http://localhost:5678/mcp` 即可。

### 2.提供的工具

#### query_ast — 按 id 取设计稿 AST

按 `id` 返回某次设计稿的 UI AST（JSON 格式的 DSL）。典型用法：先 `get_project_scheme` 拿到适配方案，再用本工具取 AST，按方案中的「单位换算规则」把设计稿 px 转换成目标单位生成代码。AST 为绝对定位（多数 layout 为 none），生成代码时建议改写为 flex 布局，并兼顾 px 适配、样式方案与命名语义化。

| 字段 | | 说明 |
|---|---|---|
| 入参 | `id` (string) | 设计稿 AST 的 id，由用户提供。 |
| 响应 | `ast` (string) | 该 design 的 AST JSON 原文。 |

#### get_project_scheme — 查项目适配方案

生成界面前的**第一步**：按项目根目录绝对路径查询是否已保存过移动端 / PC 适配方案。命中（`exists=true`）就直接复用返回的 `scheme`，不要重新分析；未命中（`exists=false`）则需自行分析项目后调用 `save_project_scheme` 存回。未命中不是错误。

| 字段 | | 说明 |
|---|---|---|
| 入参 | `project_path` (string) | 项目根目录的绝对路径，作为方案唯一键。 |
| 响应 | `exists` (bool) | 是否已保存过方案；为 false 时应分析后保存。 |
| | `scheme` (string) | 已保存的适配方案 markdown；`exists=false` 时为空。 |
| | `updatedAt` (int64) | 上次更新时间（unix 毫秒）；`exists=false` 时为 0。 |

#### save_project_scheme — 保存项目适配方案

保存（或覆盖更新）某项目的适配方案。按 `project_path` 作唯一键，已存在则覆盖。方案分析由 AI 完成（阅读项目的 index.html / 构建配置 / 样式 / 依赖），写一份项目专属的 markdown，至少包含：① 适配方案（rem / vw / px / 响应式 / 移动端组件库 / 小程序）；② 设计稿基准宽度；③ 单位换算规则（最关键，给出公式 + 实例）；④ 构建/工具链；⑤ 响应式断点；⑥ 生成代码约定。目标：仅凭这份 markdown + 一份 AST 就能产出风格一致、单位正确的代码。

| 字段 | | 说明 |
|---|---|---|
| 入参 | `project_path` (string) | 项目根目录的绝对路径，作为方案唯一键。 |
| | `scheme` (string) | 适配方案 markdown（章节见上）。 |
| 响应 | `projectPath` (string) | 已保存的项目路径。 |
| | `updatedAt` (int64) | 本次更新时间（unix 毫秒）。 |

### 3.使用范例

在 AI 会话中，把插件同步 AST 后拿到的 `id` 交给 AI，并描述目标文件与特殊要求即可：

```
id=xxxxxx，请使用 jupi-d2c 帮我还原页面，写到 demo.vue 中，移动端、像素级还原、用 Tailwind 类名
```
```
id=yyyyyy，请使用 jupi-d2c 帮我还原页面，写到 components/Some.vue 中，其中的图片和svg图标下载下来放到 @/src/assets目录下使用，样式采用 sass.
```

AI 的典型执行流程：

1. `get_project_scheme(project_path)` 查当前项目适配方案；未命中则分析项目并 `save_project_scheme` 存回。
2. `query_ast(id)` 取回设计稿 AST。
3. 按方案的单位换算规则，把 AST 还原为 `demo.vue`。
