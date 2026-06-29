package services

import (
	"path/filepath"
	"testing"

	"jupi-d2c/internal/infra/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSchemeService(t *testing.T) *ProjectSchemeService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewProjectSchemeService(db)
}

// 不存在时 Get 返回 ErrSchemeNotFound。
func TestProjectScheme_GetNotFound(t *testing.T) {
	s := newSchemeService(t)
	_, err := s.Get("/no/such/project")
	assert.ErrorIs(t, err, ErrSchemeNotFound)
}

// Upsert 先插后查能拿回原文；再次 Upsert 覆盖 scheme、推进 updated_at、保留 created_at。
func TestProjectScheme_UpsertRoundTrip(t *testing.T) {
	s := newSchemeService(t)
	const path = "/Users/x/proj"

	first, err := s.Upsert(path, "# 方案 rem", 1000)
	require.NoError(t, err)
	assert.Equal(t, path, first.ProjectPath)
	assert.Equal(t, "# 方案 rem", first.Scheme)
	assert.Equal(t, int64(1000), first.CreatedAt)
	assert.Equal(t, int64(1000), first.UpdatedAt)

	got, err := s.Get(path)
	require.NoError(t, err)
	assert.Equal(t, first, got)

	// 同一路径再次保存：覆盖内容，created_at 不变，updated_at 前进。
	second, err := s.Upsert(path, "# 方案 viewport", 2000)
	require.NoError(t, err)
	assert.Equal(t, "# 方案 viewport", second.Scheme)
	assert.Equal(t, int64(1000), second.CreatedAt)
	assert.Equal(t, int64(2000), second.UpdatedAt)
}
