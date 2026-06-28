package main

import (
	"fmt"
	"net/http"
	"time"

	"jupi-d2c/internal/config"
	"jupi-d2c/internal/daemon"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "查看服务状态",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			pid, err := daemon.RunningPID(pidFile)
			if err != nil {
				return fmt.Errorf("检查运行状态失败: %w", err)
			}
			if pid == 0 {
				fmt.Fprintln(out, "状态: 已停止")
				return nil
			}

			fmt.Fprintf(out, "状态: 运行中 (pid %d)\n", pid)

			// 进程存活不代表 HTTP 已就绪——额外探测 /health 给出健康度。
			// 这里只读不生成：配置缺失时报“未知”，绝不因一次状态查询而落盘。
			cfg, err := config.LoadFromPath(config.ResolvePath(configFile))
			if err != nil {
				fmt.Fprintf(out, "健康: 未知（加载配置失败: %v）\n", err)
				return nil
			}

			url := fmt.Sprintf("http://localhost:%d/health", cfg.Port)
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				fmt.Fprintf(out, "健康: 不可达 (%s)\n", url)
				return nil
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Fprintf(out, "健康: OK (%s)\n", url)
			} else {
				fmt.Fprintf(out, "健康: 异常 HTTP %d (%s)\n", resp.StatusCode, url)
			}
			return nil
		},
	}
}
