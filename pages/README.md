# LianT · 联T（Next.js 重写版）

原 Vue 3 + Vite 下载站的 Next.js + React + TypeScript 重写版，UI 与功能保持一致。

## 技术栈

- Next.js 16（App Router）
- React 19 + TypeScript

## 本地运行

```bash
npm install          # 安装依赖
npm run dev          # 启动开发服务器（已内置 --webpack）
```

打开 **http://localhost:3000/** 即可预览。生产模式：`npm run build && npm start`。

> dev / build 已配置为使用 **Webpack** 编译（`next dev --webpack` / `next build --webpack`），因为 Next.js 16 默认的 Turbopack 不支持 Android/Termux 平台。在普通电脑上运行也完全正常，只是编译方式不同。

## 页面

| 路由 | 说明 |
| --- | --- |
| `/` | 主页（Hero + 使用步骤） |
| `/manager` | 下载 Manager（Telegram Bot 管理器客户端） |
| `/service` | 下载 Service（服务端组件） |

## Release 数据

下载页的版本号与下载链接来自 `src/data/releases.generated.json`（构建期静态化，运行时不再调用 GitHub API）。

更新 Release 数据（发布新版本后执行）：

```bash
npm run fetch:releases
```

可选环境变量：`GITHUB_REPO`（默认 `Bailensn/LianT`）、`GITHUB_TOKEN`（未认证时每小时 60 次限额）。

## 自动化（GitHub Actions + Vercel Deploy Hook）

`.github/workflows/deploy-on-release.yml` 实现了发布即自动部署：当 GitHub 发布新 Release 时，自动执行 `fetch-releases.mjs` 刷新 `releases.generated.json` 并提交，然后通过 Vercel Deploy Hook 触发重新部署。

使用前需在仓库 Settings → Secrets 配置：

| Secret | 说明 |
| --- | --- |
| `GITHUB_TOKEN` | 仓库自带，用于拉取 Release 数据与提交（默认已存在） |
| `VERCEL_DEPLOY_HOOK` | Vercel 项目生成的 Deploy Hook URL（Settings → Deploy Hooks） |

> 本目录位于 LianT 主仓库的 `pages` 子目录，workflow 内已使用 `pages/` 前缀与 `working-directory: pages`。若拆分为独立仓库，需去掉这些前缀（文件顶部有注释说明）。

## 项目结构

```
src/
├── app/
│   ├── layout.tsx            # 根布局（导航 + 页面切换动画）
│   ├── page.tsx              # 主页
│   ├── manager/page.tsx      # 下载 Manager
│   ├── service/page.tsx      # 下载 Service
│   ├── icon.tsx              # 站点 favicon（由品牌 Logo 生成）
│   └── globals.css           # 全局样式
├── components/
│   ├── Nav.tsx               # 顶部导航（含移动端汉堡菜单）
│   ├── LogoIcon.tsx          # 品牌 Logo SVG
│   ├── SystemIcon.tsx        # 系统图标（Windows/macOS/Android/Linux）
│   ├── DownloadSystem.tsx    # 下载页容器（标题 + 版本徽标 + 卡片网格）
│   ├── DownloadArch.tsx      # 单个系统/架构下载卡片
│   └── PageTransition.tsx    # 页面切换动画
├── data/
│   ├── downloads.ts          # Manager/Service 构建列表 + 文件名拼接
│   ├── github.ts             # 读取静态 Release 数据
│   └── releases.generated.json
└── lib/
    └── logo.ts               # 品牌 Logo 路径数据
scripts/
└── fetch-releases.mjs        # 构建期抓取 GitHub Release 的脚本
```

## 部署（推荐 Vercel）

Vercel 原生支持 Next.js：推送到 GitHub 后导入 Vercel，框架自动识别 Next.js，无需额外配置。若 Release 数据需要更新，在 Vercel 构建命令中可先执行 `npm run fetch:releases && npm run build`。
