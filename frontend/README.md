# KBGO Frontend

KBGO 知识库管理系统的现代化前端界面，使用 React + TypeScript + Vite 构建。

## 功能特性

### 知识库管理
- 创建、编辑、删除知识库
- 知识库分类和状态管理
- 搜索和筛选知识库

### 文档管理
- 文件上传和 URL 导入
- 文档索引和分块配置
- 批量操作（删除、重新索引）
- 文档状态跟踪

### 智能对话
- 实时聊天界面
- 支持流式和非流式输出
- 知识库检索集成
- 对话历史管理
- Markdown 渲染支持

### 模型管理
- 查看所有配置的 AI 模型
- 按类型筛选（LLM、Embedding、Rerank、多模态）
- 重新加载模型配置
- 模型状态监控

## 技术栈

- **框架**: React 18
- **语言**: TypeScript
- **构建工具**: Vite
- **路由**: React Router v6
- **状态管理**: Zustand
- **样式**: TailwindCSS
- **HTTP 客户端**: Axios
- **Markdown 渲染**: react-markdown
- **图标**: Lucide React

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 配置环境变量

创建 `.env` 文件（可选，默认使用代理）:

```env
# API 基础 URL（如果需要）
VITE_API_BASE_URL=http://localhost:8000
```

### 3. 启动开发服务器

```bash
npm run dev
```

前端将在 http://localhost:3000 启动。

### 4. 构建生产版本

```bash
npm run build
```

构建输出在 `dist/` 目录。

## 项目结构

```
src/
├── components/          # 共享组件
│   └── Layout/         # 布局组件（侧边栏、主布局）
├── pages/              # 页面组件
│   ├── KnowledgeBase/  # 知识库管理
│   ├── Documents/      # 文档管理
│   ├── Chat/           # 对话聊天
│   └── Models/         # 模型管理
├── services/           # API 服务
│   ├── api.ts          # API 客户端
│   └── index.ts        # API 接口
├── store/              # 状态管理
│   └── index.ts        # Zustand store
├── types/              # TypeScript 类型定义
│   └── index.ts        # 类型定义
├── lib/                # 工具函数
│   └── utils.ts        # 通用工具
├── App.tsx             # 应用根组件
├── main.tsx            # 应用入口
└── index.css           # 全局样式
```

## 开发指南

### API 代理配置

开发环境下，所有 `/v1/*` 请求会自动代理到后端服务器（默认 `http://localhost:8000`）。

在 `vite.config.ts` 中修改代理配置:

```typescript
server: {
  proxy: {
    '/v1': {
      target: 'http://your-backend-url:8000',
      changeOrigin: true,
    },
  },
}
```

### 添加新页面

1. 在 `src/pages/` 创建新页面目录
2. 在 `src/App.tsx` 添加路由
3. 在 `src/components/Layout/Sidebar.tsx` 添加导航项

### 自定义样式

全局样式在 `src/index.css` 中定义，使用 TailwindCSS 的 `@layer` 指令。

常用 CSS 类:
- `.btn` - 基础按钮
- `.btn-primary` - 主要按钮
- `.btn-secondary` - 次要按钮
- `.btn-danger` - 危险按钮
- `.input` - 输入框
- `.card` - 卡片容器

## API 接口

前端通过以下 API 与后端通信:

- **知识库**: `/v1/kb`
- **文档**: `/v1/upload`, `/v1/documents`, `/v1/index`
- **对话**: `/v1/chat`, `/v1/conversations`
- **模型**: `/v1/model/list`, `/v1/model/reload`

详细 API 文档请参考后端 README。

## 浏览器支持

- Chrome >= 90
- Firefox >= 88
- Safari >= 14
- Edge >= 90

## 故障排除

### 端口占用

如果 3000 端口被占用，修改 `vite.config.ts`:

```typescript
server: {
  port: 3001, // 使用其他端口
}
```

### API 请求失败

1. 确保后端服务已启动（默认 http://localhost:8000）
2. 检查浏览器控制台的网络请求
3. 验证 API 代理配置

### 构建错误

清除缓存并重新安装:

```bash
rm -rf node_modules package-lock.json
npm install
```

## License

MIT License