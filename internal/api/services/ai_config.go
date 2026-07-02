package services

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrAIConfigNotFound 表示 AI 配置不存在或未配置完整。
var ErrAIConfigNotFound = errors.New("AI 配置未设置或未配置完整")

// AIConfig 是全局 AI 配置（OpenAI 兼容 API 的接入信息）。
type AIConfig struct {
	URL       string `json:"url"`
	Key       string `json:"key"`
	Model     string `json:"model"`
	UpdatedAt int64  `json:"updatedAt"` // unix 毫秒
}

// AIConfigService 负责 AI 配置的持久化与查询，使用单行表 ai_config(id=1)。
type AIConfigService struct {
	db *sql.DB
}

// NewAIConfigService 绑定已打开的数据库连接。
func NewAIConfigService(db *sql.DB) *AIConfigService {
	return &AIConfigService{db: db}
}

// Get 返回当前 AI 配置；不存在时返回 ErrAIConfigNotFound。
func (s *AIConfigService) Get() (AIConfig, error) {
	var cfg AIConfig
	err := s.db.QueryRow(
		`SELECT url, key, model, updated_at FROM ai_config WHERE id = 1`,
	).Scan(&cfg.URL, &cfg.Key, &cfg.Model, &cfg.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AIConfig{}, ErrAIConfigNotFound
	}
	if err != nil {
		return AIConfig{}, fmt.Errorf("查询 AI 配置失败: %w", err)
	}
	return cfg, nil
}

// GetConfigured 返回 url/key/model 三者都非空的配置；任一项为空时返回 ErrAIConfigNotFound。
// 供 handler 判断"是否可调用 AI"时使用。
func (s *AIConfigService) GetConfigured() (AIConfig, error) {
	cfg, err := s.Get()
	if err != nil {
		return AIConfig{}, err
	}
	if cfg.URL == "" || cfg.Key == "" || cfg.Model == "" {
		return AIConfig{}, ErrAIConfigNotFound
	}
	return cfg, nil
}

// IsConfigured 返回 url/key/model 是否都已配置。
func (s *AIConfigService) IsConfigured() (bool, error) {
	cfg, err := s.Get()
	if err != nil {
		if errors.Is(err, ErrAIConfigNotFound) {
			return false, nil
		}
		return false, err
	}
	return cfg.URL != "" && cfg.Key != "" && cfg.Model != "", nil
}

// Update 写入 AI 配置（UPSERT id=1），nowMs 为 unix 毫秒。
func (s *AIConfigService) Update(url, key, model string, nowMs int64) error {
	_, err := s.db.Exec(
		`INSERT INTO ai_config (id, url, key, model, updated_at)
		 VALUES (1, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     url        = excluded.url,
		     key        = excluded.key,
		     model      = excluded.model,
		     updated_at = excluded.updated_at`,
		url, key, model, nowMs,
	)
	if err != nil {
		return fmt.Errorf("保存 AI 配置失败: %w", err)
	}
	return nil
}
