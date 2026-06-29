package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"jupi-d2c/internal/config"
	"jupi-d2c/internal/daemon"

	"github.com/spf13/cobra"
)

// 全局 flag，被各子命令共享。
var (
	pidFile    string // --pid-file，daemon 进程的 PID 落盘位置
	configFile string // --config，显式指定配置文件路径（优先于默认搜索）
)

// 版本信息，发布时由 GoReleaser 通过 -ldflags -X 注入；
// 源码构建（go build / go run）时保持下列默认占位值。
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// newRootCmd 组装根命令与子命令。
// 不带子命令直接运行根命令即前台阻塞运行服务（stdout 实时展示 HTTP 活动）；
// start/stop/status 则以守护进程方式控制后台实例。
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "jupi-d2c",
		Short:         "D2C 上传管理服务",
		Long:          "D2C 上传管理服务。\n\n直接运行（无子命令）将在前台启动服务并打印 HTTP 活动；\nstart/stop/status 用于以守护进程方式控制后台实例。",
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		Args:          cobra.NoArgs,
		SilenceUsage:  true, // 运行期错误不再打印 usage，避免噪音
		SilenceErrors: true, // 错误由 main 统一打印，避免重复
		// 未显式指定 --pid-file 时，把默认 PID 落点对齐到配置目录：
		// 开发（./config.yml）落工作目录，生产落 ~/.jupi-d2c。configFile 此时已完成解析。
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("pid-file") {
				pidFile = filepath.Join(config.RuntimeDir(configFile), "jupi-d2c.pid")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runForeground()
		},
	}
	root.SetVersionTemplate("jupi-d2c {{.Version}}\n")

	// 默认留空，真正的默认在 PersistentPreRunE 里按配置目录解析（见上）。
	root.PersistentFlags().StringVar(&pidFile, "pid-file", "",
		"PID 文件路径（默认与配置同目录：./jupi-d2c.pid 或 ~/.jupi-d2c/jupi-d2c.pid）")
	root.PersistentFlags().StringVar(&configFile, "config", "",
		"配置文件路径（默认 ./config.yml，否则 ~/.jupi-d2c/config.yml）")

	root.AddCommand(newStartCmd(), newStopCmd(), newStatusCmd())
	return root
}

// runForeground 在前台阻塞运行 HTTP 服务，直到收到 SIGINT/SIGTERM 后优雅关闭。
// 这是裸命令的行为，同时也是 start 在后台拉起的目标。
func runForeground() error {
	path := config.ResolvePath(configFile)
	if _, err := config.EnsureConfig(path); err != nil {
		return fmt.Errorf("准备配置失败: %w", err)
	}
	cfg, err := config.LoadFromPath(path)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d, err := daemon.New(cfg, path)
	if err != nil {
		return fmt.Errorf("初始化失败: %w", err)
	}
	if err := d.Run(ctx); err != nil {
		return fmt.Errorf("运行失败: %w", err)
	}
	return nil
}
