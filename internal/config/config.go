package config

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed defaults.yml
var defaultsYAML []byte

// ConfigDir 返回 CLI 工具的全局配置目录（~/.jupi-d2c）。
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// 极端回退：用当前目录
		return "."
	}
	return filepath.Join(home, ".jupi-d2c")
}

// ResolvePath 按优先级解析配置文件路径，显式优于约定：
//
//	显式 --config flag > ./config.yml（存在时）> ~/.jupi-d2c/config.yml
//
// flagPath 为空表示未提供该 flag。后两级是“开发用本地文件 / 生产用 home 文件”的默认约定。
func ResolvePath(flagPath string) string {
	if strings.TrimSpace(flagPath) != "" {
		return flagPath
	}
	const local = "./config.yml"
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return filepath.Join(ConfigDir(), "config.yml")
}

// RuntimeDir 返回运行时辅助文件（PID / 日志）的默认存放目录，与配置解析保持同一约定：
// 开发（存在 ./config.yml）落在进程工作目录，生产落在 ~/.jupi-d2c。
// 取「解析出的配置文件所在目录」，使这些文件始终与 config.yml 相邻，便于运维定位。
func RuntimeDir(flagConfigPath string) string {
	return filepath.Dir(ResolvePath(flagConfigPath))
}

// AppConfig 是启动期解析并校验后的配置。
// 配置的唯一来源是 config.yml，无环境变量 / .env 兜底。
type AppConfig struct {
	Port        int
	Token       string
	UploadDir   string
	MaxFileSize int64
	WorkerCount int
	QueueSize   int
}

// LoadFromPath 纯读取并校验 config.yml——文件不存在时直接返回错误，绝不写盘。
// 生成默认配置是 Bootstrap/EnsureConfig 的显式职责，读取类命令不应有副作用。
//
// 相对的 upload_dir 会被锚定到“配置文件所在目录”并归一化为绝对路径，
// 使上传目录始终落在配置旁边，而非进程恰好所在的工作目录。
func LoadFromPath(path string) (AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AppConfig{}, fmt.Errorf("配置文件不存在: %s", path)
		}
		return AppConfig{}, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var y yamlConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // 拼错的 key 直接报错，而非被静默忽略后用零值
	if err := dec.Decode(&y); err != nil {
		return AppConfig{}, fmt.Errorf("解析配置文件失败: %w", err)
	}

	cfg := AppConfig{
		Port:        y.Port,
		Token:       y.Token,
		UploadDir:   resolveUploadDir(y.UploadDir, path),
		MaxFileSize: y.MaxFileSize,
		WorkerCount: y.WorkerCount,
		QueueSize:   y.QueueSize,
	}
	if err := Validate(cfg); err != nil {
		return AppConfig{}, err
	}
	return cfg, nil
}

// resolveUploadDir 把相对的上传目录锚定到配置文件所在目录，并归一化为绝对路径。
// 空值原样返回，交给 Validate 报“不能为空”。
func resolveUploadDir(dir, configPath string) string {
	if dir == "" {
		return dir
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(configPath), dir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// EnsureConfig 在路径不存在时从嵌入模板生成配置，存在则原样保留。
// 返回是否为本次新建。仅应由“确实要启动服务”的命令调用。
func EnsureConfig(path string) (created bool, err error) {
	switch _, statErr := os.Stat(path); {
	case statErr == nil:
		return false, nil
	case errors.Is(statErr, os.ErrNotExist):
		if err := Bootstrap(path); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, fmt.Errorf("检查配置文件失败: %w", statErr)
	}
}

// tokenPlaceholder 是嵌入模板里待替换的 token 占位行。
const tokenPlaceholder = `token: ""`

// Bootstrap 把嵌入的 defaults.yml 逐字写出作为 config.yml（保留注释与排版），
// 仅将 token 占位替换为随机值，原子写入后把 token 打到 stderr 供运营直接复制。
// 刻意不走“反序列化→结构体→重序列化”，以免丢掉模板里的注释、字段顺序由结构体反客为主。
func Bootstrap(path string) error {
	tpl := string(defaultsYAML)
	if !strings.Contains(tpl, tokenPlaceholder) {
		return fmt.Errorf("默认模板缺少 token 占位 %q", tokenPlaceholder)
	}
	token := randomToken()
	rendered := strings.Replace(tpl, tokenPlaceholder, fmt.Sprintf("token: %q", token), 1)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := writeFileAtomic(path, []byte(rendered)); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "⚙ 已生成默认配置: %s\n", path)
	fmt.Fprintf(os.Stderr, "  访问 token: %s\n", token)
	return nil
}

// randomToken 生成 64 位十六进制随机 token。
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("生成随机 token 失败: %v", err))
	}
	return hex.EncodeToString(b)
}

// Validate 校验配置取值，供 Load 与面板 PUT 共用，确保永不写入非法配置。
func Validate(c AppConfig) error {
	if c.Token == "" {
		return errors.New("缺少 token 配置")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port 超出范围 (1-65535): %d", c.Port)
	}
	if c.MaxFileSize <= 0 {
		return fmt.Errorf("max_file_size 必须为正数: %d", c.MaxFileSize)
	}
	if c.WorkerCount < 1 {
		return fmt.Errorf("worker_count 必须 >= 1: %d", c.WorkerCount)
	}
	if c.QueueSize < 1 {
		return fmt.Errorf("queue_size 必须 >= 1: %d", c.QueueSize)
	}
	if c.UploadDir == "" {
		return errors.New("upload_dir 不能为空")
	}
	return nil
}

// yamlConfig 是 config.yml 的落盘形状（snake_case，与配置文件 key 一致）。
type yamlConfig struct {
	Port        int    `yaml:"port"`
	Token       string `yaml:"token"`
	UploadDir   string `yaml:"upload_dir"`
	MaxFileSize int64  `yaml:"max_file_size"`
	WorkerCount int    `yaml:"worker_count"`
	QueueSize   int    `yaml:"queue_size"`
}

// Save 校验并原子写入 config.yml。
func Save(path string, c AppConfig) error {
	if err := Validate(c); err != nil {
		return err
	}
	return writeYAMLAtomic(path, yamlConfig{
		Port:        c.Port,
		Token:       c.Token,
		UploadDir:   c.UploadDir,
		MaxFileSize: c.MaxFileSize,
		WorkerCount: c.WorkerCount,
		QueueSize:   c.QueueSize,
	})
}

// writeYAMLAtomic 序列化 yamlConfig 后原子写入，供 Save 使用。
func writeYAMLAtomic(path string, y yamlConfig) error {
	data, err := yaml.Marshal(y)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return writeFileAtomic(path, data)
}

// writeFileAtomic 原子写入（同目录临时文件 + rename），供 Save 与 Bootstrap 共用。
// 临时文件由 os.CreateTemp 以 0600 创建，token 落盘即受限于属主可读写。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.yml.tmp")
	if err != nil {
		return fmt.Errorf("创建临时配置文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后为 no-op

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("同步临时配置文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时配置文件失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}
