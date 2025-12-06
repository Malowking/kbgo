package common

import (
	"os"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
)

// TestMain 在所有测试运行前执行，用于设置测试环境
// 禁用日志文件输出，只输出到 stdout
func TestMain(m *testing.M) {
	// 获取全局日志对象并配置为仅输出到控制台，禁用文件输出
	ctx := gctx.GetInitCtx()
	logger := g.Log()

	// 配置日志：只输出到 stdout，不写入文件
	logger.SetConfig(glog.Config{
		Flags:       glog.F_TIME_STD,
		Level:       glog.LEVEL_ALL,
		StdoutPrint: true,
		Path:        "", // 明确设置为空，不生成日志文件
		File:        "", // 明确设置为空，不生成日志文件
	})

	logger.Debug(ctx, "Test environment initialized - logging to stdout only, no log files will be created")

	// 运行测试
	code := m.Run()

	// 退出
	os.Exit(code)
}
