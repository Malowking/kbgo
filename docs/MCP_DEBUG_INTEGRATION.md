# MCP 功能集成到 debug.html 的说明

## 已完成的工作

1. **侧边栏菜单已添加** - 在 debug.html 第 388-401 行已经添加了 MCP 管理菜单
2. **HTML和JavaScript代码已准备好** - 所有 MCP 相关代码在 `debug_mcp_addon.html` 文件中

## 如何完成集成

### 方法：手动复制内容

1. 打开 `debug_mcp_addon.html` 文件
2. 复制从第一个 `<!-- MCP: 注册MCP服务 -->` 开始到最后一个 `</script>` 标签之前的所有内容
3. 打开 `debug.html`  文件
4. 找到第 1124 行的注释：`<!-- 此处插入 MCP 相关的所有 HTML 部分，请参考 debug_mcp_addon.html -->`
5. 将复制的内容粘贴到该注释下方（在 `</div>` 之前）
6. 找到第 1762 行 `</script>` 标签之前
7. 将 `debug_mcp_addon.html` 中的所有 JavaScript 函数（在 `<script>` 标签内的部分）复制粘贴到该位置

## 文件说明

- **debug.html**: 主调试页面，已包含侧边栏菜单，需要插入 MCP 内容
- **debug_mcp_addon.html**: 包含所有 MCP 相关的 HTML 和 JavaScript 代码

## MCP 功能列表

1. 注册MCP服务
2. MCP服务列表
3. 获取MCP服务
4. 更新MCP服务
5. 删除MCP服务
6. 测试MCP连接
7. 列出MCP工具
8. 调用MCP工具
9. MCP调用日志
10. MCP统计信息

## API 端点

所有 MCP 接口都已实现并可以通过 debug.html 调试：

- POST `/api/v1/mcp/registry` - 注册服务
- GET `/api/v1/mcp/registry` - 服务列表
- GET `/api/v1/mcp/registry/{id}` - 获取服务
- PUT `/api/v1/mcp/registry/{id}` - 更新服务
- DELETE `/api/v1/mcp/registry/{id}` - 删除服务
- POST `/api/v1/mcp/registry/{id}/test` - 测试连接
- GET `/api/v1/mcp/registry/{id}/tools` - 列出工具
- POST `/api/v1/mcp/call` - 调用工具
- GET `/api/v1/mcp/logs` - 查询日志
- GET `/api/v1/mcp/registry/{id}/stats` - 统计信息