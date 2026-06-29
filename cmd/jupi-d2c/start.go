package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"jupi-d2c/internal/config"
	"jupi-d2c/internal/daemon"

	"github.com/spf13/cobra"
)

var logFile string // --log-file

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "后台启动服务",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// 未显式指定 --log-file 时，把日志落点对齐到配置目录：
			// 开发（./config.yml）落工作目录，生产落 ~/.jupi-d2c。
			if !cmd.Flags().Changed("log-file") {
				logFile = filepath.Join(config.RuntimeDir(configFile), "jupi-d2c.log")
			}

			// 已在运行则拒绝重复启动。
			if pid, err := daemon.RunningPID(pidFile); err != nil {
				return fmt.Errorf("检查运行状态失败: %w", err)
			} else if pid > 0 {
				return fmt.Errorf("服务已在运行 (pid %d)", pid)
			}

			// 在父进程里确保配置就绪：首次运行生成的 token 打到本终端，
			// 而不是被重定向进日志文件后让用户找不到。
			cfgPath := config.ResolvePath(configFile)
			if _, err := config.EnsureConfig(cfgPath); err != nil {
				return fmt.Errorf("准备配置失败: %w", err)
			}

			// 读出端口与 token，供启动成功后提示访问地址（首次生成的 token 也已由
			// Bootstrap 打到 stderr，这里再次显示便于直接复制使用）。
			cfg, err := config.LoadFromPath(cfgPath)
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}

			// 把后台进程的输出重定向到日志文件。
			out, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return fmt.Errorf("打开日志文件失败: %w", err)
			}
			defer out.Close()

			// 后台拉起裸命令（即前台运行模式）。
			self, err := os.Executable()
			if err != nil {
				return fmt.Errorf("定位可执行文件失败: %w", err)
			}

			// 显式 --config 不会随 CWD/环境继承，必须透传给后台子进程，
			// 否则它会按默认搜索路径解析出另一份配置。
			var childArgs []string
			if configFile != "" {
				childArgs = append(childArgs, "--config", cfgPath)
			}

			child := exec.Command(self, childArgs...)
			child.Stdout = out
			child.Stderr = out
			// Setsid 让子进程脱离当前终端会话，成为独立的后台进程。
			child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

			if err := child.Start(); err != nil {
				return fmt.Errorf("启动后台进程失败: %w", err)
			}

			pid := child.Process.Pid
			if err := daemon.WritePIDFile(pidFile, pid); err != nil {
				// 写 PID 失败：杀掉刚拉起的进程，避免出现“运行中但无法管理”的孤儿。
				_ = child.Process.Kill()
				return fmt.Errorf("写入 PID 文件失败: %w", err)
			}
			// 与子进程解除父子关系，避免其成为僵尸。
			_ = child.Process.Release()

			// 短暂等待，确认进程没有立刻因配置错误等原因退出。
			time.Sleep(300 * time.Millisecond)
			if !daemon.ProcessAlive(pid) {
				_ = daemon.RemovePIDFile(pidFile)
				return fmt.Errorf("服务启动后立即退出，详见日志 %s", logFile)
			}

			out2 := cmd.OutOrStdout()
			fmt.Fprintf(out2, "已启动 (pid %d)\n", pid)
			fmt.Fprintf(out2, "  访问地址: http://localhost:%d\n", cfg.Port)
			fmt.Fprintf(out2, "  访问 token: %s\n", cfg.Token)
			fmt.Fprintf(out2, "  日志文件: %s\n", logFile)
			return nil
		},
	}
	// 默认留空，真正的默认在 RunE 里按配置目录解析（见上）。
	cmd.Flags().StringVar(&logFile, "log-file", "", "后台进程日志文件路径（默认与配置同目录：./jupi-d2c.log 或 ~/.jupi-d2c/jupi-d2c.log）")
	return cmd
}
