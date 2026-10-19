# 反向代理配置（网页管理端）

管理端默认只监听 **127.0.0.1:28742**，页面在 `http://127.0.0.1:28742/`。
要让手机/电脑通过域名访问，用 Nginx / Caddy 反代即可，两种写法都支持。

## 方案 A：反代剥掉前缀（推荐）

```nginx
location /LianT/ {
    proxy_pass http://127.0.0.1:28742/;      # 注意结尾的 / —— 会把 /LianT/ 前缀剥掉
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Prefix /LianT;
}
```

```caddy
handle_path /LianT/* {                 # handle_path 自动剥前缀
    reverse_proxy 127.0.0.1:28742
}
```

访问：`https://你的域名/LianT/`。token 用 `LIANT_ADMIN_TOKEN`，或 `./LianT admin` 打印的那串。

## 方案 B：让程序自己认前缀

反代不剥前缀时，给任务进程设 `LIANT_ADMIN_PREFIX=/LianT`：

```nginx
location /LianT/ {
    proxy_pass http://127.0.0.1:28742;        # 保留前缀，由程序处理
    proxy_set_header X-Forwarded-Prefix /LianT;
}
```

## 鉴权与安全

- token 顺序：`LIANT_ADMIN_TOKEN` 环境变量 > `.env` 里的同名项 > 数据库里自动生成（`./LianT admin` 可查看）。
- 带 token 的三种方式：`Authorization: Bearer <token>`、`?token=<token>`、或 cookie（用 `?token=` 打开页面会写 cookie，之后刷新不用再带）。
- **绑非回环地址却没配 token 时管理端拒绝启动**；把 `LIANT_ADMIN_ADDR` 改成 `0.0.0.0` 之前请务必先配 token 并加 HTTPS。
- 所有写操作都会进 `audit` 表（网页「操作日志」页可见）。

## 相关环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `LIANT_ADMIN_ADDR` | `127.0.0.1:28742` | 管理端监听地址 |
| `LIANT_ADMIN_TOKEN` | 自动生成 | 访问 token |
| `LIANT_ADMIN_PREFIX` | 空 | 页面挂载前缀，如 `/LianT` |
| `LIANT_ADMIN_HTML` | 内置 | 自定义页面文件（同目录放 `admin.css` / `admin.js` 可热更新） |
| `LIANT_ADMIN_ENABLE` | `true` | 设 `false` 完全关掉管理端 |
| `LIANT_ADMIN_TLS_CERT` / `LIANT_ADMIN_TLS_KEY` | 空 | 直接给管理端上 TLS（也可以交给反代） |

## Telegram Webhook（可选）

Webhook 现在按 Bot 配置（SQLite `bots` 表），在网页后台「机器人 → 编辑」里填写：

- `Webhook URL`：公网 HTTPS 地址
- `Webhook secret`：随机字符串
- `Webhook 本地监听`：如 `127.0.0.1:28743`
- `Webhook 路径`：如 `/telegram/webhook`

多个 webhook Bot 必须使用不同的本地监听端口（或由反代按路径分发到不同端口）。

```nginx
location /telegram/webhook {
    proxy_pass http://127.0.0.1:28743/telegram/webhook;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
}
```

程序启动/保存时会自动 `setWebhook`，并校验
`X-Telegram-Bot-Api-Secret-Token`。想切回长轮询：把该 Bot 的模式改成「长轮询」并保存，
或在网页里清空 Webhook URL。

## 静态资源

后台页面拆成了三个文件，都由程序自己输出（编译时 `go:embed` 打进二进制）：

| 路径 | 内容 |
| --- | --- |
| `/` | `admin.html` 页面骨架 |
| `/admin.css` | 样式 |
| `/admin.js` | 前端逻辑（`__BASE_PATH__` 按请求替换） |

- 三者都带 `Cache-Control: no-store`，改完刷新即生效；
- 优先读磁盘：程序会在命中的 `admin.html` 同目录找 `admin.css` / `admin.js`，找不到才用内置版本；
  想改样式不必重编译，用 `LIANT_ADMIN_HTML` 指到你的目录即可；
- 反代要把 `/`、`/admin.css`、`/admin.js`、`/api/*` 一起转发，前缀规则与页面一致。

## 排错

| 现象 | 原因 |
| --- | --- |
| 页面能开，但没样式、按钮没反应 | 反代只转发了 `/`，`/admin.css`、`/admin.js` 返回 404 |
| 刷新后 404 | 前缀没配：剥前缀的反代要设 `X-Forwarded-Prefix`，或给程序设 `LIANT_ADMIN_PREFIX` |
| 401 | token 没带或不对；用 `?token=` 打开一次会写 cookie |
| Webhook 收不到 | 本地监听端口没被转发，或 `X-Telegram-Bot-Api-Secret-Token` 不匹配 |
| 日志出现 `Webhook 模式没有设置 secret_token` | 没配 Webhook secret，任何能访问该路径的人都能伪造更新 |

