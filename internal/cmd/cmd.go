package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	internalCache "github.com/Malowking/kbgo/internal/cache"
	"github.com/Malowking/kbgo/internal/controller/kbgo"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// 设置信号监听，用于优雅关闭
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			// 启动后台协程监听关闭信号
			go func() {
				sig := <-sigChan
				g.Log().Infof(ctx, "收到关闭信号: %v，开始优雅关闭...", sig)

				// 关闭缓存层，刷盘所有剩余数据
				g.Log().Info(ctx, "正在刷盘缓存数据...")
				if messageCache := internalCache.GetMessageCache(); messageCache != nil {
					messageCache.Close()
					g.Log().Info(ctx, "✓ 消息缓存层已关闭")
				}
				if mcpLogCache := internalCache.GetMCPCallLogCache(); mcpLogCache != nil {
					mcpLogCache.Close()
					g.Log().Info(ctx, "✓ MCP日志缓存层已关闭")
				}

				g.Log().Info(ctx, "✓ 所有缓存数据已刷盘完成")

				// 关闭HTTP服务器
				if err := g.Server().Shutdown(); err != nil {
					g.Log().Errorf(ctx, "HTTP服务器关闭失败: %v", err)
				}

				os.Exit(0)
			}()

			s := g.Server()

			// 配置静态文件服务
			s.SetServerRoot(".")
			s.AddStaticPath("/", ".")

			s.Group("/api", func(group *ghttp.RouterGroup) {
				group.Middleware(MiddlewareMultipartMaxMemory, MiddlewareHandlerResponse, ghttp.MiddlewareCORS)
				group.Bind(
					kbgo.NewV1(),
				)
			})
			s.Run()
			return nil
		},
	}
)
