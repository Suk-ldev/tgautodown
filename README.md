# tgautodown
一款用于下载TG上视频、音乐、图片、文档、磁力链的TG机器人；只需将所需的资源转发给机器人，机器人会自动完成下载，并且会在完成后回复消息通知你。

### 最新版本全新升级
- 采用全新架构，完全重写
- 不再依赖机器人
- 不再依赖bot-api-server
- 支持获取视频、文档、音乐原文件名
- 支持查看正在下载任务进度，并通过 UID 暂停、继续、删除下载任务

# TG截图
- 下载视频
![视频下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-video.png)

- 下载音乐
![音乐下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-audio.png)

- 下载图片
![图片下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-photos.png)

- 下载磁力链
![磁力链下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-magnet.png)

- 下载文档
![文档下载截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-docs.png)

- 笔记摘抄
![笔记摘抄截图](https://github.com/nasbump/tgautodown/blob/main/screenshots/download-note.png)

# 编译安装
项目纯go实现，直接拉代码编译：
```
git clone https://github.com/Suk-ldev/tgautodown.git
cd tgautodown
go build
```

# 启动
- 启动参数：
```
$ ./tgautodown -h
usage: ./build/tgautodown options
  -cfg     ## 配置文件，默认为: /app/data/config.json
  -proxy   ## socks5代理地址: 127.0.0.1:1080
  -f2a     ## TG账号开启了两步认证的话，这里需要输入密码
  -retrycnt 10  ## 失败时最大重试次数
  -names   ## 频道名，支持公开频道用户名和私有邀请链接hash，不支持直接填写频道ID
           ## 可以传多个频道，以,号隔; 如 -names abc,+def 这表示接收公开频道abc和私有邀请链接 https://t.me/+def 中的消息
           ## 建议使用自建的私有频道，减少噪音
```
- 配置文件示例
```
{
  "cfgdir":"/app/data", ## 配置文件保存目录
  "saveDir":"/app/download", ## 下载文件保存目录
  "gopeed":"/app/bin/gopeed", ## BT下载命令路径
  "httpaddr":":2020", ## web服务端口
}
```


### 启动示例：
```
./build/tgautodown \
  -proxy 192.168.1.7:7891 \
  -cfg /app/data/config.json \
  -names +AjbQIYhiKlhhNzMx  
```

### 频道配置说明
- 公开频道或公开群：填写用户名，例如 `my_public_channel`，不要带 `https://t.me/`。
- 私有频道或私有群：填写邀请链接中的 `+hash`，例如邀请链接是 `https://t.me/+AjbQIYhiKlhhNzMx`，则填写 `+AjbQIYhiKlhhNzMx`。
- 当前不支持直接填写频道 ID。
- 如果账号尚未加入私有频道或私有群，程序会尝试使用邀请链接自动加入；也可以先在 Telegram 官方客户端里手动加入，再启动程序。
- 如果日志出现 `FLOOD_WAIT (xxxx)`，表示 Telegram 已限制该账号继续请求，需要等待括号中的秒数后再试。频繁重启或反复检查邀请链接会延长等待体验。

### 首次启动需要登陆TG
- 浏览器打开： http://<IP>:2020
- 参考下图流程完成登陆，以后就可以不用再登陆了
![web登陆](https://github.com/nasbump/tgautodown/blob/main/screenshots/web_login.jpg)

### 下载保存路径：
视频、音乐、文档、图片、磁力链、笔记分别保存在`videos`,`music`,`documents`,`photos`,`bt`,`note`目录下。

### 下载控制
开始下载后，回复消息会包含一个临时 UID，例如 `100`。下载完成、失败或删除后 UID 会被释放。

可在监听频道中发送：
```
暂停 100
继续 100
删除 100
```

- 其他
1. appid和apphash获取：https://core.telegram.org/api/obtaining_api_id

# docker启动
先拉取项目：
```
git clone https://github.com/Suk-ldev/tgautodown.git
cd tgautodown
```

编辑 `docker-compose.yml`：
```
nano docker-compose.yml
```

需要重点修改这些字段：
- `TG_CHANNEL`：要监听的频道或群。公开频道填用户名，例如 `my_public_channel`；私有频道填邀请链接中的 `+hash`，例如 `+AjbQIYhiKlhhNzMx`。
- `TG_PROXY`：Telegram 访问代理。如果服务器可以直接访问 Telegram，可留空为 `""`；如果需要代理，填写 `socks5://IP:端口`。
- `TG_F2A`：Telegram 两步验证密码。没有开启两步验证就留空为 `""`。
- `TG_RETRYCNT`：下载失败时的重试次数，一般保持 `"10"`。
- `ports`：默认把容器的 `2020` 映射到宿主机 `2020`，如需改端口可写成 `"宿主机端口:2020"`。
- `volumes`：左边是宿主机目录，右边是容器目录。通常只改左边，例如把 `/mnt/sda1/download` 改成你的下载目录。

示例 `docker-compose.yml`：
```
services:
  tgautodown:
    image: tgautodown:${TARGETARCH:-arm64}
    build:
      context: .
      dockerfile: Dockerfile
      args:
        TARGETOS: linux
        TARGETARCH: ${TARGETARCH:-arm64}
    platform: linux/${TARGETARCH:-arm64}
    container_name: tgautodown
    restart: unless-stopped
    environment:
      TG_CHANNEL: "+AjbQIYhiKlhhNzMx"  # 频道名，私有频道的话一定要带上+号
      TG_PROXY: "socks5://192.168.31.2:7891" # 代理地址，目前只支持socks5代理
      TG_F2A: "f2apassword"  # TG账号开启了两步认证的话，这里需要输入密码
      TG_RETRYCNT: "10"    # 失败时最大重试次数
    ports:
      - 2020:2020
    volumes:
      - "/mnt/sda1/download:/app/download"
      - "/mnt/sda1/data:/app/data"
```

修改完后启动：
```
docker compose up -d --build
```

ARM64 设备可直接运行默认配置；如果要构建 amd64 镜像：
```
TARGETARCH=amd64 docker compose up -d --build
```

如需强制重新构建镜像：
```
docker compose down --rmi local
docker compose build --no-cache --pull
docker compose up -d --force-recreate
```

如果是 ARM64 环境：
```
TARGETARCH=arm64 docker compose build --no-cache --pull
TARGETARCH=arm64 docker compose up -d --force-recreate
```


# 感谢
- [基于MTProto协议的TG库](github.com/gotd/td/tg)
- [下载：gopeed](https://github.com/GopeedLab/gopeed)
