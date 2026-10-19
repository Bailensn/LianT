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
| `LIANT_ADMIN_HTML` | 内置 | 自定义页面文件 |
| `LIANT_ADMIN_ENABLE` | `true` | 设 `false` 完全关掉管理端 |
