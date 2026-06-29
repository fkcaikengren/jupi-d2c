package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validCfg 返回一份可通过 Validate 的配置，UploadDir 用绝对路径避免被解析改写。
func validCfg(t *testing.T) AppConfig {
	t.Helper()
	dir := t.TempDir()
	return AppConfig{
		Port:        8080,
		Token:       "rt-token",
		UploadDir:   dir,
		DBPath:      filepath.Join(dir, "test.db"),
		MaxFileSize: 5 * 1024 * 1024,
		WorkerCount: 3,
		QueueSize:   32,
	}
}

func TestLoadFromPath_MissingErrsWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	_, err := LoadFromPath(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
	assert.NoFileExists(t, path, "读取缺失配置绝不应创建文件")
}

func TestLoadFromPath_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	want := validCfg(t)
	require.NoError(t, Save(path, want))

	got, err := LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadFromPath_RejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	raw := "port: 3000\ntoken: secret\nupload_dir: /tmp/up\n" +
		"max_file_size: 1024\n" +
		"worker_count: 1\nqueue_size: 1\nbogus_key: 1\n"
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	_, err := LoadFromPath(path)
	require.Error(t, err, "拼错/多余的 key 应被 KnownFields 拒绝")
}

func TestLoadFromPath_ResolvesRelativeUploadDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	raw := "port: 3000\ntoken: secret\nupload_dir: ./uploads\n" +
		"max_file_size: 1024\n" +
		"worker_count: 1\nqueue_size: 1\n"
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	got, err := LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "uploads"), got.UploadDir,
		"相对 upload_dir 应锚定到配置文件所在目录")
}

func TestBootstrap_GeneratesTokenWithSecurePerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, Bootstrap(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "token 文件应仅属主可读写")

	cfg, err := LoadFromPath(path)
	require.NoError(t, err)
	assert.Len(t, cfg.Token, 64, "随机 token 应为 64 位十六进制")
}

func TestBootstrap_TokensAreUnique(t *testing.T) {
	a := filepath.Join(t.TempDir(), "config.yml")
	b := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, Bootstrap(a))
	require.NoError(t, Bootstrap(b))

	ca, err := LoadFromPath(a)
	require.NoError(t, err)
	cb, err := LoadFromPath(b)
	require.NoError(t, err)
	assert.NotEqual(t, ca.Token, cb.Token)
}

func TestEnsureConfig_CreatesWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	created, err := EnsureConfig(path)
	require.NoError(t, err)
	assert.True(t, created)
	assert.FileExists(t, path)
}

func TestEnsureConfig_NoOpWhenExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, Save(path, validCfg(t)))
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	created, err := EnsureConfig(path)
	require.NoError(t, err)
	assert.False(t, created, "已存在的配置不应被覆盖")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after, "EnsureConfig 不应改动既有配置")
}

func TestResolvePath_Precedence(t *testing.T) {
	t.Run("flag 最优先", func(t *testing.T) {
		assert.Equal(t, "/from/flag.yml", ResolvePath("/from/flag.yml"))
	})

	t.Run("无 flag 时回退到 home 默认", func(t *testing.T) {
		// 测试 CWD 在 internal/config，无 ./config.yml，应落到 ConfigDir。
		assert.Equal(t, filepath.Join(ConfigDir(), "config.yml"), ResolvePath(""))
	})
}

func TestValidate_Errors(t *testing.T) {
	base := AppConfig{
		Port: 3000, Token: "secret",
		UploadDir:   "/tmp/up",
		DBPath:      "/tmp/up/jupi-d2c.db",
		MaxFileSize: 1024, WorkerCount: 1, QueueSize: 1,
	}
	require.NoError(t, Validate(base))

	cases := map[string]func(c *AppConfig){
		"空 token":     func(c *AppConfig) { c.Token = "" },
		"port 越界":     func(c *AppConfig) { c.Port = 0 },
		"maxFileSize": func(c *AppConfig) { c.MaxFileSize = 0 },
		"workerCount": func(c *AppConfig) { c.WorkerCount = 0 },
		"queueSize":   func(c *AppConfig) { c.QueueSize = 0 },
		"空 uploadDir": func(c *AppConfig) { c.UploadDir = "" },
		"空 dbPath":    func(c *AppConfig) { c.DBPath = "" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			c := base
			mut(&c)
			assert.Error(t, Validate(c))
		})
	}
}

func TestSave_NoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	require.NoError(t, Save(path, validCfg(t)))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "应只剩 config.yml，无残留 .tmp")
}

func TestSave_RejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	err := Save(path, AppConfig{Port: 0}) // 非法
	require.Error(t, err)
	assert.NoFileExists(t, path, "校验失败不应写文件")
}
