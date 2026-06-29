// Package mcp 把 jupi-d2c 暴露为一个 MCP（Model Context Protocol）服务，走 Streamable HTTP，
// 挂在主 gin 引擎的 /mcp 上。它只装配工具并把领域逻辑委托给 services，自身不含业务规则。
//
// 提供三个工具，配合 AI 生成界面的流程：
//
//	get_project_scheme(project_path)   先按绝对路径查是否已分析过适配方案；命中即复用。
//	save_project_scheme(project_path)  未命中时，AI 自行分析项目并把方案 markdown 存回。
//	query_ast(id)                      取某次设计稿的 AST 原文，结合方案换算单位生成代码。
//
// 适配方案的“分析”全部由 AI 端完成（它有项目文件访问与推理能力），本服务只负责持久化 markdown，
// 不扫描任何项目文件。
package mcp

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"jupi-d2c/internal/api/services"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 三个工具的输入/输出结构。字段的 jsonschema 标签会被 SDK 推断进 inputSchema，
// 作为给模型的参数说明。

type queryASTInput struct {
	ID string `json:"id" jsonschema:"设计稿 design AST 的 id，由用户提供，定位要取的 AST。"`
}

type queryASTOutput struct {
	AST string `json:"ast" jsonschema:"该 design 的 AST JSON 原文。"`
}

type getSchemeInput struct {
	ProjectPath string `json:"project_path" jsonschema:"项目根目录的绝对路径，作为方案的唯一键。"`
}

type getSchemeOutput struct {
	Exists    bool   `json:"exists" jsonschema:"该项目是否已保存过适配方案；为 false 时应分析后调用 save_project_scheme。"`
	Scheme    string `json:"scheme" jsonschema:"已保存的适配方案 markdown；exists 为 false 时为空。"`
	UpdatedAt int64  `json:"updatedAt" jsonschema:"上次更新时间（unix 毫秒）；exists 为 false 时为 0。"`
}

type saveSchemeInput struct {
	ProjectPath string `json:"project_path" jsonschema:"项目根目录的绝对路径，作为方案的唯一键。"`
	Scheme      string `json:"scheme" jsonschema:"适配方案 markdown，见工具描述里要求的章节。"`
}

type saveSchemeOutput struct {
	ProjectPath string `json:"projectPath"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// nowFunc 注入当前时间（unix 毫秒），便于测试；生产用 time.Now().UnixMilli()。
type deps struct {
	designs *services.DesignService
	schemes *services.ProjectSchemeService
	nowMs   func() int64
}

const queryASTDesc = `按 id 返回某次设计稿的 UI AST (也就是json格式的 DSL)。
典型用法：拿到适配方案（get_project_scheme）后，再用本工具取 AST，把 AST 里的设计稿 px 值按方案中的“单位换算规则”转换成目标单位生成界面代码。
其中 id 由用户提供。
UI AST目前定位是绝对定位的，所以很多layout都是 none，但希望你在生成代码时将布局做成 flex 布局（而不是纯 absolute 布局）。 
希望你在像素级还原页面的同时还要考虑到 px 适配、样式方案、命名语义化。`

const getSchemeDesc = `按项目绝对路径查询是否已保存过移动端/PC适配方案。
这应是生成界面前的第一步：命中（exists=true）就直接复用返回的 scheme，不要重新分析；
未命中（exists=false）则你需要自行分析该项目，再调用 save_project_scheme 存回。
未命中不是错误。`

const saveSchemeDesc = `保存（或覆盖更新）某项目的移动端/PC端适配方案。按 project_path 作唯一键，已存在则覆盖。
适配方案的分析由你（AI）完成：阅读项目的 index.html / 构建配置 / 样式文件 / 依赖后，写一份详细的、项目专属的 markdown。
要求 scheme markdown 至少包含以下章节：
1. 适配方案：rem / viewport(vw,vh) / px(PC 固定宽) / 响应式(media、container query) / 移动端组件库(vant、antd-mobile) / 小程序(rpx、Taro、uni-app) 之一。
2. 设计稿基准宽度：如 375 或 750（px）。
3. 单位换算规则（最关键，给出可操作的换算公式与一个实例）：
   - rem：如 postcss-pxtorem rootValue=37.5，设计稿 75px → 2rem；
   - vw：如 100vw=375px，设计稿 75px → 20vw（75/375*100）；
   - px：直接用设计稿 px；
   - rpx：如设计稿 750，1px → 1rpx。
4. 构建/工具链：postcss 插件及配置（rootValue / viewportWidth）、Tailwind 配置、UI 组件库。
5. 响应式断点（如适用）。
6. 生成代码约定：用 class 名还是内联样式、颜色/间距 token、字体约定、图片图标方案（默认优先采用 url 引用）等生成代码必须遵循的规则。
目标：仅凭这份 markdown + 一份 UI AST，就能产出风格一致、单位正确的UI前端代码。`

// NewHandler 用已打开的数据库装配 MCP 服务并返回其 Streamable HTTP http.Handler，
// 由 router 通过 gin.WrapH 挂到 /mcp。nowMs 注入当前时间（unix 毫秒），便于测试。
func NewHandler(db *sql.DB, nowMs func() int64) http.Handler {
	d := deps{
		designs: services.NewDesignService(db),
		schemes: services.NewProjectSchemeService(db),
		nowMs:   nowMs,
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "jupi-d2c", Version: "1"}, nil)

	mcp.AddTool(srv, &mcp.Tool{Name: "query_ast", Description: queryASTDesc},
		func(_ context.Context, _ *mcp.CallToolRequest, in queryASTInput) (*mcp.CallToolResult, queryASTOutput, error) {
			ast, err := d.designs.GetAST(in.ID)
			if err != nil {
				return nil, queryASTOutput{}, err
			}
			return nil, queryASTOutput{AST: ast}, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "get_project_scheme", Description: getSchemeDesc},
		func(_ context.Context, _ *mcp.CallToolRequest, in getSchemeInput) (*mcp.CallToolResult, getSchemeOutput, error) {
			ps, err := d.schemes.Get(in.ProjectPath)
			if errors.Is(err, services.ErrSchemeNotFound) {
				return nil, getSchemeOutput{Exists: false}, nil
			}
			if err != nil {
				return nil, getSchemeOutput{}, err
			}
			return nil, getSchemeOutput{Exists: true, Scheme: ps.Scheme, UpdatedAt: ps.UpdatedAt}, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "save_project_scheme", Description: saveSchemeDesc},
		func(_ context.Context, _ *mcp.CallToolRequest, in saveSchemeInput) (*mcp.CallToolResult, saveSchemeOutput, error) {
			ps, err := d.schemes.Upsert(in.ProjectPath, in.Scheme, d.nowMs())
			if err != nil {
				return nil, saveSchemeOutput{}, err
			}
			return nil, saveSchemeOutput{ProjectPath: ps.ProjectPath, UpdatedAt: ps.UpdatedAt}, nil
		})

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}
