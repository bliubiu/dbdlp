package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// newServeCmd 构造 serve 子命令：启动 HTTP API 与前端静态资源托管。
func newServeCmd(d *Deps) *cobra.Command {
	var listen, staticDir string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 Web 控制台与 HTTP API",
		Long: "启动 HTTP API 并托管前端静态资源（web/out）。\n" +
			"首次启动需提供控制台管理员账号：设置环境变量 DBDLP_ADMIN_USER 与 DBDLP_ADMIN_PASSWORD，\n" +
			"账号不存在时自动创建（已存在则不覆盖口令）。",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if d.HTTPServer == nil {
				return fmt.Errorf("HTTP API 未装配，无法启动服务")
			}
			addr := listen
			if addr == "" {
				addr = d.Listen
			}
			if addr == "" {
				return fmt.Errorf("监听地址不能为空")
			}
			static := staticDir
			if static == "" {
				static = d.StaticDir
			}

			srv := &http.Server{
				Addr:              addr,
				Handler:           d.HTTPServer,
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       60 * time.Second,
				WriteTimeout:      120 * time.Second,
				IdleTimeout:       120 * time.Second,
			}

			// 优雅停机：收到 SIGINT/SIGTERM 后先停止收新请求，再等在途请求结束。
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				cmd.Printf("控制台已启动：http://%s\n", addr)
				if static != "" {
					cmd.Printf("静态资源目录：%s\n", static)
				}
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- err
					return
				}
				errCh <- nil
			}()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				cmd.Println("收到停机信号，开始优雅停机……")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := srv.Shutdown(shutdownCtx); err != nil {
					return fmt.Errorf("优雅停机失败：%w", err)
				}
				cmd.Println("服务已停止")
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&listen, "listen", "", "HTTP 监听地址（默认取 dbdlp.toml 的 [web].listen）")
	cmd.Flags().StringVar(&staticDir, "static-dir", "", "前端静态资源目录（默认取 dbdlp.toml 的 [web].static_dir）")
	return cmd
}
