package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"d2c-manager/internal/config"
	"d2c-manager/internal/queue"
	"d2c-manager/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.AppConfig{
		Token:         "secret",
		UploadDir:     t.TempDir(),
		PublicBaseURL: "http://localhost:3000",
		MaxFileSize:   1024,
		WorkerCount:   2,
		QueueSize:     8,
	}
	pool := queue.NewPool(cfg.WorkerCount, cfg.QueueSize, storage.SaveBytes)
	pool.Start()
	t.Cleanup(func() { _ = pool.Shutdown(context.Background()) })
	return NewRouter(cfg, pool)
}

type uploadResp struct {
	Data storage.SavedFile `json:"data"`
}

func TestHealth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	newTestServer(t).ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
	assert.Contains(t, w.Body.String(), `"maxFileSize":1024`)
}

func TestUpload_RawBinary(t *testing.T) {
	r := newTestServer(t)
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
	assert.Contains(t, body.Data.URL, "http://localhost:3000/uploads/")
	assert.Regexp(t, `\.png$`, body.Data.Filename)
}

func TestUpload_Multipart(t *testing.T) {
	r := newTestServer(t)
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
	r := newTestServer(t)
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
	r := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "请求体为空")
}

func TestUpload_TooLarge(t *testing.T) {
	r := newTestServer(t)
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
	r := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", bytes.NewReader([]byte("x")))
	req.Header.Set("Content-Type", "image/png")
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestServeUpload_RoundTrip(t *testing.T) {
	r := newTestServer(t)
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
	r := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/uploads/does-not-exist.png", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestNotFound(t *testing.T) {
	r := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), `"not found"`)
}

func TestCORS_Preflight(t *testing.T) {
	r := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/upload", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	// 预检响应带方法/头/Max-Age（与 Hono cors() 一致）。
	assert.Equal(t, "GET,POST,OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type,Authorization,PRIVATE-TOKEN", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_NonPreflightHeaders(t *testing.T) {
	r := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://example.com")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	// 实际响应只带 Allow-Origin / Expose-Headers，不带预检专用头（与 Hono 一致）。
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Content-Length", w.Header().Get("Access-Control-Expose-Headers"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(t, w.Header().Get("Access-Control-Max-Age"))
}
