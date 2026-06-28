package handlers

import (
	"net/http"
	"strings"

	"d2c-manager/internal/config"

	"github.com/gin-gonic/gin"
)

// ===== 配置 API DTO（camelCase，与 web/src/api.ts 契约一致）=====

type configDTO struct {
	Port          int    `json:"port"`
	UploadDir     string `json:"uploadDir"`
	PublicBaseURL string `json:"publicBaseURL"`
	MaxFileSize   int64  `json:"maxFileSize"`
	WorkerCount   int    `json:"workerCount"`
	QueueSize     int    `json:"queueSize"`
	TokenSet      bool   `json:"tokenSet"` // token 只写不回显，仅暴露是否已设置
}

type configResponse struct {
	Config          configDTO `json:"config"`
	RestartRequired bool      `json:"restartRequired"`
}

// configUpdate 用指针区分"未提供"与"零值"。token 留空/省略=保留现值。
type configUpdate struct {
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
		UploadDir:     c.UploadDir,
		PublicBaseURL: c.PublicBaseURL,
		MaxFileSize:   c.MaxFileSize,
		WorkerCount:   c.WorkerCount,
		QueueSize:     c.QueueSize,
		TokenSet:      c.Token != "",
	}
}

// merge 以 base 为基底套用 upd 中已提供的字段，返回合并后的配置。
// token 留空表示保留现值；publicBaseURL 去掉尾部斜杠。
func merge(base config.AppConfig, upd configUpdate) config.AppConfig {
	next := base
	if upd.Port != nil {
		next.Port = *upd.Port
	}
	if upd.UploadDir != nil {
		next.UploadDir = *upd.UploadDir
	}
	if upd.PublicBaseURL != nil {
		next.PublicBaseURL = strings.TrimRight(*upd.PublicBaseURL, "/")
	}
	if upd.MaxFileSize != nil {
		next.MaxFileSize = *upd.MaxFileSize
	}
	if upd.WorkerCount != nil {
		next.WorkerCount = *upd.WorkerCount
	}
	if upd.QueueSize != nil {
		next.QueueSize = *upd.QueueSize
	}
	if upd.Token != nil && strings.TrimSpace(*upd.Token) != "" {
		next.Token = strings.TrimSpace(*upd.Token)
	}
	return next
}

// GetConfig 返回磁盘上的当前配置，并标记是否与运行实例不一致（需重启）。
func (h *Handlers) GetConfig(c *gin.Context) {
	disk := h.configs.Current()
	c.JSON(http.StatusOK, configResponse{
		Config:          toDTO(disk),
		RestartRequired: h.configs.RestartRequired(disk),
	})
}

// PutConfig 以磁盘配置为基底合并提交字段，校验后原子写回 config.yml。
func (h *Handlers) PutConfig(c *gin.Context) {
	var upd configUpdate
	if err := c.ShouldBindJSON(&upd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON", "message": err.Error()})
		return
	}

	next := merge(h.configs.Current(), upd)

	if err := h.configs.Save(next); err != nil {
		// Save 内含 Validate：非法配置在此被 400 拦下，绝不落盘。
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configResponse{
		Config:          toDTO(next),
		RestartRequired: h.configs.RestartRequired(next),
	})
}
