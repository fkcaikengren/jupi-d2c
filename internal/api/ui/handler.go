// Package ui 是本地控制面板：配置 REST API + 内嵌单页应用。
// 监听 0.0.0.0:AdminPort，全部接口可访问；
// /api/config 因为能改写配置（含 token 与端口）需要 Bearer token，
// UI 页面与 /api/health 公开。
package ui

import (
	"net/http"
	"strings"

	"d2c-manager/internal/config"

	"github.com/gin-gonic/gin"
)

// Handler 持有 daemon 正在运行的配置快照（只读）与 config.yml 路径。
// 它从不修改运行中的配置或触碰 server/pool —— 变更仅落盘，下次重启生效。
type Handler struct {
	cfg        config.AppConfig // 运行中的配置（不可变快照）
	configPath string
}

// configDTO 是回给前端的 JSON 形状。Token 只写不读：永不序列化，仅用 tokenSet 表示是否已配置。
type configDTO struct {
	Port          int    `json:"port"`
	AdminPort     int    `json:"adminPort"` // 前端只读
	UploadDir     string `json:"uploadDir"`
	PublicBaseURL string `json:"publicBaseURL"`
	MaxFileSize   int64  `json:"maxFileSize"`
	WorkerCount   int    `json:"workerCount"`
	QueueSize     int    `json:"queueSize"`
	TokenSet      bool   `json:"tokenSet"`
}

// configInput 是 PUT 的入参，全部可选（指针）。token 为 nil/空表示保留现有值。
// AdminPort 故意不接受输入（只读，避免把自己锁在外面）。
type configInput struct {
	Port          *int    `json:"port"`
	UploadDir     *string `json:"uploadDir"`
	PublicBaseURL *string `json:"publicBaseURL"`
	MaxFileSize   *int64  `json:"maxFileSize"`
	WorkerCount   *int    `json:"workerCount"`
	QueueSize     *int    `json:"queueSize"`
	Token         *string `json:"token"`
}

func toDTO(c config.AppConfig) configDTO {
	return configDTO{
		Port:          c.Port,
		AdminPort:     c.AdminPort,
		UploadDir:     c.UploadDir,
		PublicBaseURL: c.PublicBaseURL,
		MaxFileSize:   c.MaxFileSize,
		WorkerCount:   c.WorkerCount,
		QueueSize:     c.QueueSize,
		TokenSet:      c.Token != "",
	}
}

// restartRequired 比较影响运行时的字段，判断是否需要重启才能生效。
func restartRequired(a, b config.AppConfig) bool {
	return a.Port != b.Port ||
		a.AdminPort != b.AdminPort ||
		a.UploadDir != b.UploadDir ||
		a.PublicBaseURL != b.PublicBaseURL ||
		a.MaxFileSize != b.MaxFileSize ||
		a.WorkerCount != b.WorkerCount ||
		a.QueueSize != b.QueueSize ||
		a.Token != b.Token
}

// effectiveConfig 读取磁盘上的有效配置；读取失败时回退到运行中的配置。
func (h *Handler) effectiveConfig() config.AppConfig {
	if c, err := config.LoadFromPath(h.configPath); err == nil {
		return c
	}
	return h.cfg
}

// GetConfig 返回磁盘上的有效配置（不含 token），并标注是否与运行中的配置不一致。
func (h *Handler) GetConfig(c *gin.Context) {
	onDisk := h.effectiveConfig()
	c.JSON(http.StatusOK, gin.H{
		"config":          toDTO(onDisk),
		"restartRequired": restartRequired(onDisk, h.cfg),
	})
}

// PutConfig 合并入参到磁盘配置、校验、原子写回，并返回是否需要重启。
func (h *Handler) PutConfig(c *gin.Context) {
	var in configInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON", "message": err.Error()})
		return
	}

	merged := h.effectiveConfig()
	if in.Port != nil {
		merged.Port = *in.Port
	}
	if in.UploadDir != nil {
		merged.UploadDir = *in.UploadDir
	}
	if in.PublicBaseURL != nil {
		merged.PublicBaseURL = strings.TrimRight(*in.PublicBaseURL, "/")
	}
	if in.MaxFileSize != nil {
		merged.MaxFileSize = *in.MaxFileSize
	}
	if in.WorkerCount != nil {
		merged.WorkerCount = *in.WorkerCount
	}
	if in.QueueSize != nil {
		merged.QueueSize = *in.QueueSize
	}
	// token 为 nil/空 => 保留现有；非空 => 更新。
	if in.Token != nil && *in.Token != "" {
		merged.Token = *in.Token
	}

	if err := config.Validate(merged); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.Save(h.configPath, merged); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入配置失败", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":          toDTO(merged),
		"restartRequired": restartRequired(merged, h.cfg),
	})
}

// Health 给 UI 一个轻量探活端点（公开）。
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}