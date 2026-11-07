# Docker 部署

镜像里已经包含 Service 与网页管理端，不需要额外装 Node 或 Go。

## 一键部署

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | bash
```

脚本会检查 Docker、生成 compose 文件、拉镜像、启动，并打印配对命令。
带域名：

```bash
LIANT_DOMAIN=liant.example.com curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | bash
```

会自动起 Caddy 申请证书，之后不再有自签证书警告。

## 直接跑

```bash
docker compose up -d
```

默认拉取 `ghcr.io/bailensn/liant:latest`；没有镜像或想从源码构建时：

```bash
docker compose build && docker compose up -d
```

如果 GHCR 上的包还是私有的，先登录一次：

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u <你的用户名> --password-stdin
```

打开 `https://<服务器地址>:8443/`，首次会有自签证书警告，信任一次即可。

数据全在命名卷 `liant_data` 里，包含配置、节点密钥和 SQLite：
升级镜像后数据仍在，删掉卷等于重新开始。

## 账号密码

暴露到公网时建议加一层 HTTP 基本认证，浏览器会弹出登录框：

```yaml
environment:
  LIANT_BASIC_AUTH: "admin:换成你的密码"
```

一键脚本会自动生成随机密码并打印出来。认证挡在网页、`/v1/ws` 与媒体下载前面，
同一来源每分钟失败 10 次后会被限流；失败会写进安全日志。

不用 Docker 时等价命令：

```bash
export LIANT_BASIC_AUTH='admin:密码'
./LianT serve
# 或写进配置
./LianT config set auth.user admin
./LianT config set auth.pass 密码
```

## 首次配对

`pair` 需要终端交互，用一次性容器接进同一个卷：

```bash
docker compose run --rm liant pair --host 192.168.1.50
```

`--host` 填手机能访问到的地址。终端会打印配对码、二维码和配对链接，
手机扫二维码即可；步骤结束后 `pair` 进程退出，服务继续由 `docker compose up -d` 的那个容器提供。

如果服务和手机不在同一网络，用 `--public-url https://你的域名` 生成正确的配对链接。

## 用域名和正式证书

`docker-compose.proxy.yml` 会额外起一个 Caddy，自动申请 Let's Encrypt 证书，
LianT 只在内部提供明文，浏览器看到的是 Caddy 的证书——不用再点「继续访问」。

```bash
LIANT_DOMAIN=liant.example.com \
  docker compose -f docker-compose.yml -f docker-compose.proxy.yml up -d
```

配对时同样用域名：

```bash
LIANT_DOMAIN=liant.example.com \
docker compose -f docker-compose.yml -f docker-compose.proxy.yml run --rm \
  -e LIANT_NO_TLS=1 liant pair --public-url https://liant.example.com
```

`LIANT_NO_TLS=1` 让 LianT 关闭内置 TLS；`--public-url` 让二维码指向对外域名。
若用 Nginx 等其它反代，等价条件是：

- 反代到容器 `liant:8443` 的明文 HTTP；
- 转发 WebSocket 升级头 `Upgrade` / `Connection`；
- 容器环境变量 `LIANT_NO_TLS=1`。

## 常用命令

```bash
docker compose logs -f liant          # 日志
docker compose run --rm liant version # 版本
docker compose down                   # 停止
docker compose build --no-cache       # 重新构建
```

## 注意

- 端口固定 8443，反代对外可以用任意端口。
- 网页端与 WSS 同源，反代必须同时转发 `/` 与 `/v1/ws`，不要只转发其中一个。
- 换节点密钥等于换身份：卷里的 `data/node` 删掉后，所有已配对设备都要重新配对。
- 镜像同时发布 `linux/amd64` 与 `linux/arm64`，树莓派和 x86 服务器都能直接跑。

## 已有 Nginx / Caddy 时

用覆盖文件把服务收到回环地址，交给宿主机反代：

```bash
LIANT_BASIC_AUTH='admin:换成你的密码' \
  docker compose -f docker-compose.yml -f docker-compose.external-proxy.yml up -d
```

Nginx：

```nginx
server {
    listen 443 ssl http2;
    server_name lt.lensn.xyz;

    ssl_certificate     /etc/letsencrypt/live/lt.lensn.xyz/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/lt.lensn.xyz/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 1h;
    }
}
```

Caddy：

```caddyfile
lt.lensn.xyz {
    reverse_proxy 127.0.0.1:8443
}
```

两处必须都在：`/` 与 `/v1/ws` 走同一个 server 块，WebSocket 的 `Upgrade` 头不能丢。

配了 `LIANT_BASIC_AUTH` 之后，浏览器登录完直接点「直接进入」即可，
不需要配对码——服务端把「通过 Basic 认证」当作授权，设备自动登记并出现在设置页。
