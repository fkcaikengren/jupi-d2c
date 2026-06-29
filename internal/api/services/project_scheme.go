package services

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrSchemeNotFound 表示按项目绝对路径找不到已保存的适配方案。
var ErrSchemeNotFound = errors.New("project scheme not found")

// ProjectScheme 是一个项目的移动端适配方案记录。
// Scheme 是 AI 分析后写入的 markdown，作为单一可信来源（含单位换算规则等）。
type ProjectScheme struct {
	ProjectPath string `json:"projectPath"`
	Scheme      string `json:"scheme"`
	CreatedAt   int64  `json:"createdAt"` // unix 毫秒
	UpdatedAt   int64  `json:"updatedAt"` // unix 毫秒
}

// ProjectSchemeService 负责项目适配方案的持久化与查询，以绝对路径为主键。
type ProjectSchemeService struct {
	db *sql.DB
}

// NewProjectSchemeService 绑定已打开的数据库连接。
func NewProjectSchemeService(db *sql.DB) *ProjectSchemeService {
	return &ProjectSchemeService{db: db}
}

// Get 按项目绝对路径返回适配方案；不存在返回 ErrSchemeNotFound。
func (s *ProjectSchemeService) Get(projectPath string) (ProjectScheme, error) {
	var ps ProjectScheme
	err := s.db.QueryRow(
		`SELECT project_path, scheme, created_at, updated_at FROM project_schemes WHERE project_path = ?`,
		projectPath,
	).Scan(&ps.ProjectPath, &ps.Scheme, &ps.CreatedAt, &ps.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectScheme{}, ErrSchemeNotFound
	}
	if err != nil {
		return ProjectScheme{}, fmt.Errorf("查询项目方案失败: %w", err)
	}
	return ps, nil
}

// Upsert 按绝对路径写入适配方案：不存在则插入，存在则覆盖 scheme 并刷新 updated_at；
// created_at 在更新时保持不变。nowMs 由调用方传入（unix 毫秒）。返回写入后的完整记录。
func (s *ProjectSchemeService) Upsert(projectPath, scheme string, nowMs int64) (ProjectScheme, error) {
	_, err := s.db.Exec(
		`INSERT INTO project_schemes (project_path, scheme, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_path) DO UPDATE SET
		     scheme     = excluded.scheme,
		     updated_at = excluded.updated_at`,
		projectPath, scheme, nowMs, nowMs,
	)
	if err != nil {
		return ProjectScheme{}, fmt.Errorf("保存项目方案失败: %w", err)
	}
	return s.Get(projectPath)
}
