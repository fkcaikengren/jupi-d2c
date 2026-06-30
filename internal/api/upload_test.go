package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"jupi-d2c/internal/config"
	"jupi-d2c/internal/infra/database"
	"jupi-d2c/internal/infra/queue"
	"jupi-d2c/internal/infra/storage"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer 起一个完整引擎（含 worker 池、临时 config.yml 与临时 SQLite），供上传/配置/design 测试共用。
func newTestServer(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	cfg := config.AppConfig{
		Port:        5678,
		Token:       "secret",
		UploadDir:   dir,
		DBPath:      filepath.Join(dir, "test.db"),
		MaxFileSize: 1024,
		WorkerCount: 2,
		QueueSize:   8,
	}
	pool := queue.NewPool(cfg.WorkerCount, cfg.QueueSize, storage.SaveBytes)
	pool.Start()
	t.Cleanup(func() { _ = pool.Shutdown(context.Background()) })

	db, err := database.Open(cfg.DBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, config.Save(path, cfg))
	return NewRouter(cfg, pool, path, db), path
}

type uploadResp struct {
	Data storage.SavedFile `json:"data"`
}

func TestHealth(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
	assert.Contains(t, w.Body.String(), `"maxFileSize":1024`)
}

func TestUpload_RawBinary(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader([]byte("PNGDATA")))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)
	var body uploadResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(7), body.Data.Size)
	assert.Equal(t, "image/png", body.Data.ContentType)
	assert.Contains(t, body.Data.URL, "http://localhost:5678/uploads/")
	assert.Regexp(t, `\.png$`, body.Data.Filename)
}

func TestUpload_Multipart(t *testing.T) {
	r, _ := newTestServer(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "avatar.png")
	require.NoError(t, err)
	_, _ = part.Write([]byte("IMG"))
	require.NoError(t, mw.Close())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)
	var body uploadResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(3), body.Data.Size)
	assert.Regexp(t, `\.png$`, body.Data.Filename)
}

func TestUpload_MultipartMissingField(t *testing.T) {
	r, _ := newTestServer(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("notfile", "x"))
	require.NoError(t, mw.Close())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "multipart 字段 file 缺失或不是文件")
}

func TestUpload_Empty(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "请求体为空")
}

func TestUpload_TooLarge(t *testing.T) {
	r, _ := newTestServer(t)
	big := make([]byte, 2048) // > MaxFileSize 1024
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(big))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, 413, w.Code)
	assert.Contains(t, w.Body.String(), "文件超过最大限制")
	assert.Contains(t, w.Body.String(), `"actual":2048`)
}

func TestUpload_NoAuth(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader([]byte("x")))
	req.Header.Set("Content-Type", "image/png")
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestServeUpload_RoundTrip(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader([]byte("DATA")))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	var body uploadResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/uploads/"+body.Data.Filename, nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
	assert.Equal(t, "DATA", w2.Body.String())
}

func TestServeUpload_Missing(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/uploads/does-not-exist.png", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

// TestSPA_FallbackAndAPI404 验证单服务下的兜底：未知非 API 路径回退到内嵌前端
// （已构建为 index.html，未构建为占位页，均 200 text/html）；未知 /api/* 返回 404 JSON。
func TestSPA_FallbackAndAPI404(t *testing.T) {
	r, _ := newTestServer(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/some/client/route", nil))
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	assert.Equal(t, 404, w2.Code)
	assert.Contains(t, w2.Body.String(), "not found")
}
