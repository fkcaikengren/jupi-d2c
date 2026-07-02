// Package handlers 是 HTTP 传输层：解析 gin 请求、调用 services、按契约回写 JSON。
// 它依赖 services（领域逻辑）与 config/storage（数据形状），自身不含业务规则。
package handlers

import (
	"database/sql"

	"jupi-d2c/internal/api/services"
	"jupi-d2c/internal/config"
	"jupi-d2c/internal/infra/queue"
)

// Handlers 持有处理请求所需的依赖：启动期配置快照与各 service。
// cfg 用于鉴权外的运行期参数（maxFileSize、UploadDir）；service 封装池、配置读写与 design 存储。
type Handlers struct {
	cfg       config.AppConfig
	uploads   *services.UploadService
	configs   *services.ConfigService
	designs   *services.DesignService
	schemes   *services.ProjectSchemeService
	aiConfigs *services.AIConfigService
	ai        *services.AIService
}

// New 用启动期快照、worker 池、config.yml 路径与数据库连接装配各 service。
func New(cfg config.AppConfig, pool *queue.Pool, configPath string, db *sql.DB) *Handlers {
	designs := services.NewDesignService(db)
	aiConfigs := services.NewAIConfigService(db)
	return &Handlers{
		cfg:       cfg,
		uploads:   services.NewUploadService(pool),
		configs:   services.NewConfigService(configPath, cfg),
		designs:   designs,
		schemes:   services.NewProjectSchemeService(db),
		aiConfigs: aiConfigs,
		ai:        services.NewAIService(designs, aiConfigs),
	}
}
