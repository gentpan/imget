# imget

[English](README.md) · **简体中文**

一个免费图片服务。一条 URL 给你随机图、壁纸、或指定尺寸的占位图。可自部署,MIT 协议。

线上 demo:**https://img.et**

## 能干什么

- **按分类随机出图** — `https://img.et/1920/1080?type=landscape` 每次访问返回不同的风景图,自动缩放 + 转格式。
- **壁纸模式** — `https://img.et/landscape` 直接返回一张 4K-8K 原图(302 到 CDN),适合壁纸 app 接入。
- **20 个分类** — banner、landscape、beauty、anime、city、nature、car、game、food、animal、travel、space、tech、business、sports、architecture、wedding、kids、abstract、concert。
- **自定义搜索** — `?keyword=红色法拉利` 覆盖分类默认词,直接搜 Pexels / Pixabay。
- **固定一张图** — `?r=42` 让同一 URL 始终返回同一张,适合文章头图、壁纸主题、强缓存。
- **每日自动补图** — 内置定时任务,每天给每个分类拉若干新图,池子持续更新。
- **WebP / AVIF 输出** — 默认现代格式,`?format=webp|avif` 切换。

## 安装(Docker 三条命令)

```bash
git clone https://github.com/gentpan/imget.git && cd imget
cp .env.example .env       # 然后编辑,见下面"配置"
docker compose up -d
```

服务跑在 `http://localhost:8080`。数据持久化在 `./data/`。

改了 `.env` 或拉了新代码,重建:
```bash
docker compose up -d --build
```

## 配置

编辑 `.env`,最少要填这些才能拿到真照片:

```ini
# 你的公网域名(OG 标签、首页示例代码用到)
SITE_BASE_URL=https://img.example.com

# Pexels — 主源(4K-8K 原图)。免费,注册:
#   https://www.pexels.com/api/
PEXELS_API_KEY=xxxxxxxxxxxxxxxxxxxx

# Pixabay — 兜底源。免费,注册:
#   https://pixabay.com/api/docs/
PIXABAY_API_KEY=xxxxxxxx-xxxxxxxxxxxxxxx

# S3 兼容存储(Cloudflare R2 / MinIO / SeaweedFS / AWS S3 / 自建均可)
# 全部留空 = 纯本地磁盘模式。
R2_ENDPOINT=https://your-s3-host
R2_ACCESS_KEY_ID=xxx
R2_SECRET_ACCESS_KEY=xxx
R2_BUCKET=imget
R2_CDN_BASE_URL=https://cdn.example.com
```

其他常用参数(都有合理默认值):

| 字段 | 默认 | 作用 |
|---|---|---|
| `SITE_NAME` | `imget` | 站名,显示在 title 和页脚。 |
| `DEFAULT_FORMAT` | `webp` | `webp` 或 `avif`。 |
| `MAX_DIM` | `4000` | 用户请求宽高上限。 |
| `INITIAL_PREFETCH_COUNT` | `5` | 新的 (尺寸, 分类) 组合首次访问时先抓几张种子图。 |
| `TOPUP_TYPES_MIN_PER_TYPE` / `MAX_PER_TYPE` | `5` / `10` | 每天自动补图的每分类随机数范围。设 `30` / `50` 一次能补 ~500-800 张。 |
| `PEXELS_MIN_WIDTH` | `1920` | 丢弃 Pexels 上宽度小于此值的图。设 `2560` 偏好 2K+,`3840` 严格 4K。 |

完整字段见 `.env.example`。**所有凭证只走 `.env`,没有单独的 secrets 文件,也没有管理后台。**

## URL 用法

```
https://img.et/1920/1080                          # 随机图,1920×1080,WebP
https://img.et/1920/1080?type=landscape           # 指定分类
https://img.et/800/600?type=car&format=avif       # 指定格式
https://img.et/1920/1080?r=42                     # 固定:同一 URL 始终返回同一张
https://img.et/1920/1080?r=42&s=2                 # 同一篇文章的第二张
https://img.et/800/600?keyword=red+ferrari        # 自定义搜索词

https://img.et/landscape                          # 壁纸模式:4K-8K 原图
https://img.et/landscape?r=42                     # 固定壁纸
https://img.et/type/landscape                     # 等价的命名空间形式
```

首页每个分类卡片都是链接,点进去就是该分类的壁纸 URL。

## 每日自动补图

内置一个子命令一次性给所有 20 个分类拉新图:

```bash
# 容器内或宿主机原生跑的话:
imget topup-types
```

用 cron 或 systemd timer 定时跑。下面是每天 04:00 UTC 自动跑的 timer:

```ini
# /etc/systemd/system/imget-topup.service
[Service]
Type=oneshot
WorkingDirectory=/opt/imget
ExecStart=/opt/imget/imget topup-types

# /etc/systemd/system/imget-topup.timer
[Timer]
OnCalendar=*-*-* 04:00:00 UTC
Persistent=true

[Install]
WantedBy=timers.target
```

```bash
systemctl enable --now imget-topup.timer
```

## 命令一览

```bash
imget serve              # HTTP 服务(默认,docker-compose 跑的就是它)
imget topup-types        # 每日补图,每分类随机 N 张
imget cleanup-cache      # 清掉过期的渲染缓存(原图永远保留)
imget r2-sync            # 把磁盘上还没传的文件批量上传 S3
imget r2-prune-orphans   # 删 S3 里 DB 不认的对象(--yes 实际执行)
imget import-r2          # 一次性:把已有 S3 bucket 索引进数据库
```

## License

MIT。
