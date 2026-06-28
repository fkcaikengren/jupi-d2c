package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"jupi-d2c/internal/config"
	"jupi-d2c/internal/daemon"

	"github.com/spf13/cobra"
)

// defaultLogFile 是后台进程 stdout/stderr 的落盘位置。
const defaultLogFile = "./jupi-d2c.log"

var logFile string // --log-file

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "后台启动服务",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			fmt.Fprintf(cmd.OutOrStdout(), "已启动 (pid %d)，日志: %s\n", pid, logFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&logFile, "log-file", defaultLogFile, "后台进程日志文件路径")
	return cmd
}
