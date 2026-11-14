// Package cli 命令行入口：cobra 子命令编排应用层用例。
package cli

import (
	"context"
	"fmt"

	"dbdlp/internal/domain/port"
	"github.com/spf13/cobra"
)

// 本文件实现失败对象重试（retry）与断点续扫（resume）两个命令。

// newRetryCmd 重试失败对象。
func newRetryCmd(d *Deps) *cobra.Command {
	var taskID, format, out string
	cmd := &cobra.Command{
		Use:   "retry",
		Short: "重试扫描失败的对象（仅重扫失败表，已成功的表不重复扫描）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			res, err := d.ScanSvc.RetryFailed(ctx, taskID, format, out)
			if err != nil {
				return err
			}
			cmd.Printf("重试完成：任务 %s\n", res.TaskID)
			cmd.Printf("重扫表数：%d，新增台账：%d，剩余失败表：%d\n", res.Retried, res.NewFindings, res.Remaining)
			cmd.Printf("任务状态：%s\n", res.Status)
			d.log(ctx, port.LevelINFO, "命令行重试完成",
				port.Field{Key: "任务", Value: res.TaskID},
				port.Field{Key: "重扫表数", Value: fmt.Sprintf("%d", res.Retried)},
				port.Field{Key: "剩余失败表", Value: fmt.Sprintf("%d", res.Remaining)})
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "任务 ID（必填）")
	cmd.Flags().StringVar(&format, "report", "", "报告格式：excel|html|json（可选，支持逗号分隔多格式）")
	cmd.Flags().StringVar(&out, "out", d.reportOut(), "报告输出目录")
	cmd.MarkFlagRequired("task")
	return cmd
}

// newResumeCmd 断点续扫。
func newResumeCmd(d *Deps) *cobra.Command {
	var taskID, format, out string
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "从断点继续未完成的表（已完成的表不重复扫描）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			res, err := d.ScanSvc.Resume(ctx, taskID, format, out)
			if err != nil {
				return err
			}
			cmd.Printf("续扫完成：任务 %s\n", res.TaskID)
			cmd.Printf("续扫表数：%d，新增台账：%d，未完成表：%d\n", res.Retried, res.NewFindings, res.Remaining)
			cmd.Printf("任务状态：%s\n", res.Status)
			d.log(ctx, port.LevelINFO, "命令行续扫完成",
				port.Field{Key: "任务", Value: res.TaskID},
				port.Field{Key: "续扫表数", Value: fmt.Sprintf("%d", res.Retried)},
				port.Field{Key: "未完成表", Value: fmt.Sprintf("%d", res.Remaining)})
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "任务 ID（必填）")
	cmd.Flags().StringVar(&format, "report", "", "报告格式：excel|html|json（可选，支持逗号分隔多格式）")
	cmd.Flags().StringVar(&out, "out", d.reportOut(), "报告输出目录")
	cmd.MarkFlagRequired("task")
	return cmd
}
