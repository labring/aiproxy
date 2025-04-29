重点信息如下
项目采用的框架是 react + vite + tailwindcss + typescript
路由使用的是 "react-router": "^7.5.1"
包管理器使用的是 pnpm
ui 使用的是 shadcn
主要分为左右布局，左边是 sidebar 负责主导航，右边是内容区。
项目 icon 使用 lucide-react

---

项目要求

1. 结构完整，功能完整，代码规范，可维护性强
2. 目录组装，文件命名符合规范，关键代码需要有友好注释
3. 使用 vite 进行开发，使用 vite 进行打包
4. 使用 tailwindcss 进行样式开发，tailwindcss 使用的是 v4 版本
5. 使用 shadcn 进行 ui 开发
6. 使用 react-router "react-router": "^7.5.1", 进行路由开发
7. 使用 tanstack query @tanstack/react-query@5.74.4 进行数据请求
8. 使用 tanstack table 进行表格开发
9. 使用 zustand 进行状态管理
10. 使用 react-hook-form 进行表单开发
11. 使用 zod 进行数据验证
12. 使用 react-i18next 进行国际化
13. 支持中英文切换，默认使用中文
14. 使用 i18next-http-backend 加载翻译文件
15. 使用 i18next-browser-languagedetector 检测用户语言
16. 项目 icon 使用 lucide-react

---

项目主要目录结构介绍
静态资源
public
public/locales 是国际化文件夹，负责项目的国际化配置
public/locales/en/translation.json 是英文翻译文件
public/locales/zh/translation.json 是中文翻译文件
src/assets 是项目静态资源文件夹，负责项目的静态资源管理

入口与配置
index.html 单应用挂载点，入口文件
src/main.tsx 是项目入口文件，负责项目的初始化
src/App.tsx 是项目主组件，负责项目的根组件
src/i18n.ts 是国际化 i18n 配置文件，负责项目的国际化配置
eslint.config.js 是 eslint 的配置文件，负责项目的 eslint 配置
package.json 包文件
vite.config.ts 是 vite 的配置文件，负责项目的 vite 配置
components.json 是 shadcn 的配置文件，负责项目的 shadcn 配置

样式
src/index.css 是项目样式文件，负责项目的样式，包括 shadcn 的样式，tailwind css 的导入

API 请求封装
src/api/index.ts: 使用 axios 创建了基础 API 客户端，包含请求拦截器和响应拦截器
src/api/auth.ts: 实现了认证相关的 API 服务，包括登录
src/api/services.ts: 统一导出 API 服务

项目内容
路由配置
src/routes 下是路由配置文件，负责路由的配置和面包屑的配置，
src/routes/config.tsx 是路由配置文件，负责路由的配置和面包屑的配置
src/routes/constants.ts 定义路由路径常量，例如 ROUTES

布局
src/components/layout 下是布局文件，负责项目的布局管理
src/components/layout/RootLayOut.tsx 是根布局文件，负责项目的根布局管理
src/components/layout/Sidebar.tsx 是侧边栏文件，负责项目的侧边栏管理

路由端点 页面
src/pages 下是页面文件，负责项目的页面管理
src/pages/auth 下是认证页面文件，负责项目的认证页面管理
src/pages/auth/login.tsx: 登录页面组件实现

src/lib 公共库
src/lib 下是项目公共库文件，负责项目的公共库管理

src/hooks hook
src/hooks 下是项目 hooks 文件，负责项目的 hooks 管理

src/types 类型
src/types 下是项目 types 文件，负责项目的 types 管理
src/types/i18next.d.ts 是 i18next 的类型文件，它扩展了 i18next 模块的类型定义，目的是提供更好的类型检查和代码补全功能。

src/components 项目组件
src/components/common 下是项目公共组件文件，负责项目的公共组件管理
src/components/layout 下是项目布局组件文件，负责项目的布局组件管理
src/components/table 下是项目表格组件文件，负责项目的表格组件管理，将 tanstack table 进行封装
src/components/ui shadcn ui 基础组件，由 shadcn 命令行生成

src/store 状态管理
src/store 下是项目状态管理文件，负责项目的状态管理
src/store/auth.ts 是认证状态管理文件，负责项目的认证状态管理

src/validation 表单验证
src/validation 下是项目表单验证文件，负责项目的表单验证管理
src/validation/auth.ts: 包含登录表单的 zod 验证 schema

src/feature 功能模块 一些页面的功能，组件可以单独抽离出来
src/feature/auth 下是认证功能模块文件，负责项目的认证功能模块管理
src/feature/auth/hook 下是认证功能模块的 tanstack query 数据管理封装
src/feature/auth/hooks.ts: 包含登录相关的 tanstack query hooks
src/feature/auth/components 下是认证功能模块组件文件，负责项目的认证功能模块组件管理
src/feature/auth/components/ProtectedRoute.tsx 是路由保护组件文件，负责项目的路由保护组件管理

src/utils 工具函数
src/utils 下是项目 utils 文件，负责项目的 utils 管理

全局状态管理:

- 认证状态: token、是否已认证等
- 状态持久化: 使用 Zustand persist 中间件实现

## 渠道功能模块

src/types/channel.ts - 定义渠道相关的类型
src/api/channel.ts - 封装渠道相关的 API 调用
src/validation/channel.ts - 渠道表单验证逻辑
src/feature/channel/hooks.ts - 渠道相关的数据请求和状态管理 hooks
src/feature/channel/components/ - 渠道相关的组件

- ChannelTable.tsx - 渠道列表表格组件，支持无限滚动
- ChannelDialog.tsx - 渠道创建/编辑对话框组件
- ChannelForm.tsx - 渠道表单组件
- DeleteChannelDialog.tsx - 渠道删除确认对话框组件
  src/pages/channel/page.tsx - 渠道页面组件
