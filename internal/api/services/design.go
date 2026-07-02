package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrDesignNotFound 表示按 id 找不到对应的 design，handler 据此回 404。
var ErrDesignNotFound = errors.New("design not found")

// Design 是 design 列表项的元信息（不含 ast 大字段）。
type Design struct {
	ID        string `json:"id"`
	Tag       string `json:"tag"`
	CreatedAt int64  `json:"createdAt"` // unix 毫秒
}

// DesignService 负责 design（AST 结果）的持久化与查询。
type DesignService struct {
	db *sql.DB
}

// NewDesignService 绑定已打开的数据库连接。
func NewDesignService(db *sql.DB) *DesignService {
	return &DesignService{db: db}
}

// Save 插入一条 design，id 为随机不可猜测串、createdAt 为传入的 unix 毫秒。
func (s *DesignService) Save(tag, ast string, createdAt int64) (Design, error) {
	id, err := randomID()
	if err != nil {
		return Design{}, err
	}
	_, err = s.db.Exec(
		`INSERT INTO designs (id, tag, ast, created_at) VALUES (?, ?, ?, ?)`,
		id, tag, ast, createdAt,
	)
	if err != nil {
		return Design{}, fmt.Errorf("保存 design 失败: %w", err)
	}
	return Design{ID: id, Tag: tag, CreatedAt: createdAt}, nil
}

// List 按生成时间倒序分页返回 design 元信息（不含 ast），并返回总数。
// tags 非空时只返回 tag 命中其中任一值的 design。
func (s *DesignService) List(page, pageSize int, tags []string) ([]Design, int, error) {
	where, args := tagFilter(tags)

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM designs`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计 design 数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := s.db.Query(
		`SELECT id, tag, created_at FROM designs`+where+
			` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("查询 design 列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]Design, 0, pageSize)
	for rows.Next() {
		var d Design
		if err := rows.Scan(&d.ID, &d.Tag, &d.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("读取 design 行失败: %w", err)
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历 design 行失败: %w", err)
	}
	return items, total, nil
}

// tagFilter 由 tag 列表构造 `WHERE tag IN (?,...)` 子句与对应参数；空列表返回空子句。
func tagFilter(tags []string) (string, []any) {
	if len(tags) == 0 {
		return "", nil
	}
	placeholders := make([]string, len(tags))
	args := make([]any, len(tags))
	for i, t := range tags {
		placeholders[i] = "?"
		args[i] = t
	}
	return " WHERE tag IN (" + strings.Join(placeholders, ",") + ")", args
}

// Tags 返回所有去重后的 tag，按字母升序，供前端筛选下拉使用。
func (s *DesignService) Tags() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT tag FROM designs ORDER BY tag ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询 tag 列表失败: %w", err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("读取 tag 失败: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 tag 失败: %w", err)
	}
	return tags, nil
}

// DeleteOlderThan 删除 created_at 早于 cutoffMs（unix 毫秒）的 design，返回删除条数。
func (s *DesignService) DeleteOlderThan(cutoffMs int64) (int, error) {
	res, err := s.db.Exec(`DELETE FROM designs WHERE created_at < ?`, cutoffMs)
	if err != nil {
		return 0, fmt.Errorf("清理 design 失败: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取清理结果失败: %w", err)
	}
	return int(n), nil
}

// UpdateReferDom 更新指定 design 的 refer_dom。
// id 不存在时返回 ErrDesignNotFound。
func (s *DesignService) UpdateReferDom(id, referDom string) error {
	res, err := s.db.Exec(`UPDATE designs SET refer_dom = ? WHERE id = ?`, referDom, id)
	if err != nil {
		return fmt.Errorf("更新 refer_dom 失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDesignNotFound
	}
	return nil
}

// UpdateReferDomWithStatus 更新 refer_dom 同时保存状态和验证错误信息。
// id 不存在时返回 ErrDesignNotFound。
func (s *DesignService) UpdateReferDomWithStatus(id, referDom, status, errorsStr string) error {
	res, err := s.db.Exec(
		`UPDATE designs SET refer_dom = ?, refer_dom_status = ?, refer_dom_errors = ? WHERE id = ?`,
		referDom, status, errorsStr, id,
	)
	if err != nil {
		return fmt.Errorf("更新 refer_dom 及状态失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDesignNotFound
	}
	return nil
}

// GetReferDom 返回指定 design 的 refer_dom、状态和错误信息。
// 不存在返回 ErrDesignNotFound。
func (s *DesignService) GetReferDom(id string) (referDom, status, errorsStr string, err error) {
	err = s.db.QueryRow(
		`SELECT refer_dom, refer_dom_status, refer_dom_errors FROM designs WHERE id = ?`, id,
	).Scan(&referDom, &status, &errorsStr)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", ErrDesignNotFound
	}
	if err != nil {
		return "", "", "", fmt.Errorf("查询 refer_dom 失败: %w", err)
	}
	return
}

// GetAST 返回指定 design 的 AST JSON 原文；不存在返回 ErrDesignNotFound。
func (s *DesignService) GetAST(id string) (string, error) {
	var ast string
	err := s.db.QueryRow(`SELECT ast FROM designs WHERE id = ?`, id).Scan(&ast)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDesignNotFound
	}
	if err != nil {
		return "", fmt.Errorf("查询 design 失败: %w", err)
	}
	return ast, nil
}

// randomID 生成 16 字节十六进制随机 id，作为 /api/ast/:id 的不可猜测凭据。
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 design id 失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}
