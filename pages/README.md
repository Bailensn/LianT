# LianT 官网 / 下载页

基于 React + Vite 的静态站点，用于介绍 LianT（联T）并提供 Manager / Service 各平台安装包的下载入口。

## 本地开发

```bash
npm install
npm run dev
```

## 构建

```bash
npm run build
```

构建产物在 `dist/` 目录，是纯静态文件，可以直接部署到任意静态托管平台。

## 部署到 Vercel

1. 把本项目文件夹放进你的仓库（例如放在一个叫 `pages` 的目录下，或作为独立仓库均可）。
2. 在 Vercel 新建项目并关联该仓库/目录。
3. Vercel 会自动识别为 Vite 项目：
   - Build Command: `npm run build`
   - Output Directory: `dist`
4. 点击部署即可，无需额外配置。

也可以用 Vercel CLI 直接从本地部署：

```bash
npm i -g vercel
vercel
```

## 修改下载链接

所有下载卡片的数据集中在 `src/data/downloads.js`：

- 目前所有按钮默认指向 `https://github.com/Bailensn/LianT/releases/latest`（GitHub Releases 页面），在你还没有发布正式安装包之前也不会出现死链接。
- 等你在 GitHub Releases 发布了具体文件后，可以把某一项的 `url` 换成固定直链格式：
  ```
  https://github.com/Bailensn/LianT/releases/latest/download/<发布时的文件名>
  ```
  这种链接会始终跟随最新版本，不需要每次发版都手动改地址。

## 目录结构

```
src/
  components/   页面各个模块（导航、Hero、架构说明、下载卡片、页脚）
  data/         下载链接数据
  styles/       全局样式（设计变量 / 基础样式 / 组件样式 / 版式）
public/         网站图标（取自 manager/resources 中的 LianT 图标）
```

## 关于设计

深色为默认主题（右上角可切换浅色），卡片的“毛玻璃”质感与悬浮时的青 / 紫双色描边，呼应了
`manager/desktop` 源码中 `GlassSheet`、`lens()`、`chromaticAberration` 等真实的界面效果参数；
圆角统一使用 32 / 36px，与 `LianTTheme` 中 `Shapes(large = 32.dp, extraLarge = 36.dp)` 保持一致。
