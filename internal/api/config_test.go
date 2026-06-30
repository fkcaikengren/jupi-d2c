package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jupi-d2c/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configResp 镜像 /api/config 的 JSON 契约，供集成测试解码（实际 DTO 在 handlers 包内未导出）。
type configResp struct {
	Config struct {
		Port     int  `json:"port"`
		TokenSet bool `json:"tokenSet"`
	} `json:"config"`
	RestartRequired bool `json:"restartRequired"`
}

func TestGetConfig_RequiresAuth(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	assert.Equal(t, 401, w.Code)
}

func TestGetConfig_WrongToken(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer nope")
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func TestGetConfig_OK(t *testing.T) {
	r, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)

	var resp configResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 5678, resp.Config.Port)
	assert.True(t, resp.Config.TokenSet)
	assert.False(t, resp.RestartRequired) // 磁盘与运行快照一致
	// token 只写不回显：响应体里不应出现明文 token。
	assert.NotContains(t, w.Body.String(), "secret")
}

func TestPutConfig_PersistsAndFlagsRestart(t *testing.T) {
	r, path := newTestServer(t)

	body, _ := json.Marshal(map[string]any{"port": 4000})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)

	var resp configResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 4000, resp.Config.Port)
	assert.True(t, resp.RestartRequired) // port 已改，与运行快照不一致

	// 落盘校验：重新从磁盘读，port 已更新，token 保留。
	onDisk, err := config.LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, 4000, onDisk.Port)
	assert.Equal(t, "secret", onDisk.Token)
}

func TestPutConfig_TokenPreservedWhenEmpty(t *testing.T) {
	r, path := newTestServer(t)

	// 提交空 token：应保留磁盘上的现值。
	body, _ := json.Marshal(map[string]any{"port": 4001, "token": ""})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)

	onDisk, err := config.LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, "secret", onDisk.Token)
}

func TestPutConfig_InvalidRejected(t *testing.T) {
	r, path := newTestServer(t)

	body, _ := json.Marshal(map[string]any{"port": 0}) // 非法端口
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)

	// 非法配置绝不落盘：磁盘仍是初始 port。
	onDisk, err := config.LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, 5678, onDisk.Port)
}
