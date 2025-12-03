package cmd

import (
	"context"

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
			s := g.Server()

			// 配置静态文件服务
			s.SetServerRoot(".")
			s.AddStaticPath("/", ".")

			s.Group("/api", func(group *ghttp.RouterGroup) {
				group.Middleware(MiddlewareHandlerResponse, ghttp.MiddlewareCORS)
				group.Bind(
					kbgo.NewV1(),
				)
			})
			s.Run()
			return nil
		},
	}
)
