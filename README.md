<div align="center">

# LotsACG

![LotsACG_banner](https://github.com/user-attachments/assets/1d2d7835-18c1-4a50-9cb9-c14ae69659be)

Collect, Download, Organize and Share your Favorite Anime Pictures.

**一个基于 [krau/ManyACG](https://github.com/krau/ManyACG) 的二次元插画收集与管理项目**

</div>

---

## ⭐ 求 Star ⭐

如果你觉得这个项目有用，欢迎给个 Star 支持一下！

[![GitHub stars](https://img.shields.io/github/stars/wwwangzilin/LotsACG?style=social)](https://github.com/wwwangzilin/LotsACG)

[→ 前往 GitHub 点 Star ←](https://github.com/wwwangzilin/LotsACG)

你的每一个 Star 都是作者继续维护的动力 💖

---

## 与上游 (krau/ManyACG) 相比的新功能

本 Fork 在保留上游全部功能的基础上，新增了以下内容：

### 🎯 智能推荐：完全交给 [Pixiv-XP-Pusher](https://github.com/bwwq/Pixiv-XP-Pusher)（原始 Python）

推荐逻辑**不在此仓库重复实现**，100% 由原始 Python 项目负责，逻辑与上游完全一致、互不干扰：

- `/xppusher` — 内置管理 XP-Pusher (Python) 进程：`start` / `stop` / `restart` / `status` / `key`
- 两者完全分离：XP-Pusher 用自己的 bot 与配置独立运行推荐
- **📤 推送到群**：XP-Pusher 推荐消息上新增「推送到群」按钮，点击时才调用 LotsACG 把作品发布到主频道
- `/xppusher key` 生成 API Key 填入 XP-Pusher 的 `config.yaml`（`lotsacg.url` / `lotsacg.api_key`）即可打通

```text
XP-Pusher (Python, 独立 bot) → 按你的 XP 个性化推荐
   └─ 推荐消息带「📤 推送到群」按钮
        └─ 点击 → 调 LotsACG REST /api/v1/bot/post_artwork → 发布到频道
```

### 🤖 AI API 接入 `[aiapi]`

- 支持任意 **OpenAI 兼容**接口（OpenAI / DeepSeek / Moonshot / 本地 Ollama 等）
- 用于 AI 自动打标、AI 重新生成描述等

### 🖼️ Pixiv 图片多代理下载

上游只有单个图床代理，本 Fork 支持**多级代理自动降级**：

```text
pximg.manyacg.top (主代理) → pixiv.cat → i.muxmus.com → 官方 i.pximg.net
```

- 配置 `source.pixiv.img_proxy` + `source.pixiv.img_proxies`
- 下载失败自动切换下一个代理，大幅提高图片下载成功率（解决上传 EOF 问题）

### 🔀 图片查重开关 `/dupcheck`

- 新增 **Telegram 指令**随时查看/切换图片查重：
  - `/dupcheck` 查看当前状态
  - `/dupcheck on` 开启查重
  - `/dupcheck off` 关闭查重
- 无需重启服务即可生效（状态存于 KV 存储）

### 📋 完整 `/help` 帮助

- 补全了所有支持的指令说明（普通用户 + 管理员指令）
- 显示版本号、构建日期、Git 提交号

### � 画师关注 `/follow`

- 私聊里用 `/follow <pixiv画师主页链接>` 关注画师，画师发布**新作品时自动推送**给你（图片+说明）
- `/unfollow` 取消关注，`/followlist` 查看已关注的画师
- 首次关注自动设置基线，**不会**把历史作品全部推给你
- 由内置监控器按 `[scheduler] watch_interval` 秒间隔定时检查

### 🏷️ 标签订阅 `/sub`

- `/sub <标签>` 订阅标签，有该标签的**新作品时自动推送**到私聊
- `/unsub` 取消订阅，`/sublist` 查看已订阅的标签
- 首次订阅自动设置基线，只推送之后的新作品

### 🌐 内置 Web 前端（ManyACG/web）

- 直接把配套的 [ManyACG/web](https://github.com/ManyACG/web) 前端缝进 REST 服务器，与 API **同源托管**在 `/`
- 启动时日志会打印 Web 访问链接（`[rest] public_url`）
- 前端路由（`/about`、`/tag/xxx`、`/artist/xxx` 等）自动 SPA 兜底；`/setu` `/sese` `/atom.xml` 自动重定向到对应 API
- 构建：`scripts/build_web.bat [API_BASE]`（或 `build_web.sh`），产物输出到 `web-dist/`，由 `[rest] web_dir` 指定

### ✨ AI 自动打标流水线

- 新作品入库时自动打标：图片 tagger（konatagger，`[tagging] tagnew`）+ **AI 标签**（`[aiapi]`/`[xpaiapi]` 的 `auto_tag = true`）
- AI 根据标题 + 已有标签自动补充 8-15 个 Pixiv 标签，大幅减少手动 `/autotag`

### 📤 批量发布队列 `/post`

- `/post` 支持作品链接、回复消息，以及**画师主页链接**（自动展开为该画师全部作品）批量排队发布
- 发布过程中实时显示进度 + **预计完成用时**（每 1 分钟刷新），完成后显示**总用时**与**平均下载速度**
- `/cancel` 取消队列（保留已发布内容），`/cd` 取消并删除已发布内容

### 🛠️ 其他改进

- **上传稳定性修复**：解决发送图片时 `io pipe closed` 错误
- **超大图自动降级**：Telegram 返回 `file is too big` 时自动加强压缩（缩小边长 + 降低画质）继续重试上传
- **R18 策略**：`/setu` 始终输出 R18；`/r18mode`（on/off/mixed）控制 `/random` 与推荐的 R18 过滤
- **自更新 `/update`**：从 GitHub Release 检测新版本，确认后自动下载替换并重启，失败自动回退旧版本
- **`/status`**：一键查看机器人状态（作品数 / 队列 / 版本 / 构建时间等）
- **构建/发布脚本**：`build_release.bat [版本] publish` 一键构建并发布 GitHub Release（配合 `/update` 自更新）
- **本地特征搜索 (imseek)**：内置 ORB 特征点以图搜图引擎（无需外部服务）

---

## 如何配置

### 1. 安装 FFmpeg（可选）

处理动图合成视频时需要 FFmpeg：

- **Ubuntu/Debian**: `sudo apt install ffmpeg -y`
- **Arch Linux**: `sudo pacman -S ffmpeg --noconfirm`
- **Windows**: 从 [gyan.dev](https://www.gyan.dev/ffmpeg/builds/) 下载 release-full 版，解压后将 `bin` 目录加入 `PATH`

### 2. 下载并准备

从 [Releases](https://github.com/wwwangzilin/LotsACG/releases) 下载对应系统/架构的二进制文件，解压后**在与二进制同目录**下创建 `config.toml`：

### 3. 最简配置（仅自动发图 Bot）

```toml
[telegram]
bot_token = "token"            # Bot Token
admins = [123456789]           # 你的 Telegram 用户 ID
# allowed_users 用户白名单(可选): 配置后仅这些用户可使用 bot 的命令,
# 未登记用户(游客)只能使用 /start /help /files(获取原图)。
allowed_users = [123456789]
username = "@yourchannel"      # 频道用户名（如有）
chat_id = -1001234567890       # 主频道 ID（与 username 二选一）

[source.pixiv]
# 建议配置 pixiv cookies，可提高爬取成功率
[[source.pixiv.cookies]]
name = "PHPSESSID"
value = ""

# 如不需要存储原图，以下配置可删除
[storage]
original_type = "telegram"
[storage.telegram]
enable = true
token = "用于发送原图的 Bot Token"  # 可与 telegram.bot_token 相同
chat_id = -1001234567890            # 用于存储原图的频道 ID
```

### 4. 运行

```bash
chmod +x lotsacg
./lotsacg
```

> Linux 下可用 systemd 托管，Windows 下可直接双击运行或注册为计划任务。

---

## 完整配置实例

```toml
[telegram]
bot_token = "123456:ABCDEF"
api_url = ""                      # 可选：自定义 Telegram API 地址
username = "@lotsacg_bot"
admins = [123456789]
# allowed_users 用户白名单(可选): 配置后仅这些用户可使用 bot 的命令,
# 未登记用户(游客)只能使用 /start /help /files(获取原图)。
allowed_users = [123456789]
caption_template = ""
chat_id = -1001111111111          # 主频道（普通作品）

# 额外目标频道：R18 作品会自动发到这里（第一个有效配置作为 R18 分流频道）
[[telegram.extra_target]]
title = "R18频道"
chat_id = -1002222222222

[database]
type = "sqlite"                   # sqlite / postgres / mysql
dsn = "lotsacg.db"

[kvdb]
type = "bbolt"                    # bbolt / redis
path = "data/kvdb.bbolt"

[search]
enable = false                    # 是否启用搜索（含查重/以图搜图依赖）
# 使用 MeiliSearch 时：
# engine = "meilisearch"
# [search.meilisearch]
# host = "http://127.0.0.1:7700"
# key = ""
# index = "lotsacg"
dup_check_enable = true           # 图片查重开关（可用 /dupcheck 指令实时切换）

# ── 本地特征点以图搜图（可选，无需外部服务）──
[imseek]
enable = false
data_dir = "./data/imseek"
distance = 64
count = 10
k = 3
nprobe = 3
nfeatures = 500
max_height = 1080
max_width = 768
auto_build = true
build_debounce_sec = 2
min_matches = 8
min_score = 25

# ── AI API：用于 /recommend 自动关联相似 tag（可选）──
# 支持任意 OpenAI 兼容接口
[aiapi]
enable = false
base_url = "https://api.openai.com/v1"   # 或 DeepSeek/Ollama 等
api_key = ""
model = "gpt-4o-mini"
recommend_tags = 12               # 每次推荐生成的关联 tag 数量
auto_tag = false                  # 是否在新作品入库时用 AI 自动补充标签

# ── XP 画像 AI API（可选，参考 Pixiv-XP-Pusher）──
# 与 [aiapi] 二选一；若 [aiapi] 未启用会自动使用此配置
[xpaiapi]
enabled = false
provider = "openai"               # openai / local
api_key = ""
base_url = "https://api.openai.com/v1"
model = "gpt-4o-mini"
scan_limit = 2000                 # 构建画像时扫描的作品数量上限
discovery_rate = 0.1              # 探索率 (0~1)：推荐中随机探索新风格的比例
auto_tag = false                  # 是否在新作品入库时用 AI 自动补充标签（[aiapi] 未启用时生效）

[xpaiapi.embedding]
model = "text-embedding-3-small"
dimensions = 1536

# 配置 Pixiv refresh_token + user_id 后, /recommend 会通过 OAuth 访问你的
# Pixiv 收藏夹, 用收藏作品的 tag 构建 XP 画像 (移植 XP-Pusher profiler)
[xpaiapi.pixiv]
refresh_token = ""
user_id = ""

[storage]
original_type = "telegram"        # telegram / local / webdav / alist
regular_length = 2560
regular_format = "webp"
thumb_length = 500
thumb_format = "avif"
cache_dir = "./imgcache"
cache_ttl = 14400

[storage.telegram]
enable = true
token = "123456:ABCDEF"
chat_id = -1003333333333

# [storage.local]
# enable = true
# path = "./data/storage"

# [storage.webdav]
# enable = true
# url = "https://dav.example.com"
# username = ""
# password = ""
# path = "/lotsacg"

[source.pixiv]
img_proxy = "pximg.manyacg.top"   # 主图片代理
img_proxies = ["pixiv.cat", "i.muxmus.com"]   # 备用代理，按顺序降级

# 单账号（兼容旧写法）
[[source.pixiv.cookies]]
name = "PHPSESSID"
value = ""
[[source.pixiv.cookies]]
name = "yuid_b"
value = ""

# 多账号轮询（可选，配置后按顺序轮换，请求失败自动跳到下一个）
[[source.pixiv.accounts]]
name = "账号A"
[[source.pixiv.accounts.cookies]]
name = "PHPSESSID"
value = ""
[[source.pixiv.accounts.cookies]]
name = "yuid_b"
value = ""

[[source.pixiv.accounts]]
name = "账号B"
[[source.pixiv.accounts.cookies]]
name = "PHPSESSID"
value = ""

[source.twitter]
# 可选：配置后可提高推特内容抓取成功率
# cookies = ""

[source.danbooru]
# 可选
# username = ""
# password = ""

[source.bilibili]
disable = false

[source.kemono]
disable = false

[source.yandere]
disable = false

[source.nhentai]
disable = false

[source]
proxy = ""                       # 可选：全局代理（http://user:pass@host:port）

[tagging]
enable = false                   # 是否启用 AI 标签生成
# engine = "konatagger"
# [tagging.konatagger]
# host = "http://127.0.0.1:8080"
# token = ""
# timeout = 30
tagnew = false                   # 是否自动为新作品打标签

[rest]
enable = false                   # 是否启用 Web 网站/API
addr = ":8080"
web_dir = "web-dist"              # 内置 Web 前端 (ManyACG/web 构建产物) 静态目录, 为空则不托管前端
public_url = "http://127.0.0.1:8080"  # Web 前端对外访问地址 (用于启动日志打印)
[rest.site]
title = "LotsACG - Kawaii is all you need"
desc = "ACG Image Collector and Gallery Server"
name = "LotsACG"
url = "https://example.com"
# [rest.limit]
# enable = true
# expiration = 60
# max = 100

# 画师关注 / 标签订阅 的监控检查间隔 (秒), 0=不启用监控
[scheduler]
watch_interval = 600

# XP-Pusher (Python) 进程管理: /xppusher start|stop|restart|status|key
[xppusher]
dir = "D:/projects/xp/Pixiv-XP-Pusher"   # XP-Pusher 项目目录
# python = ""                              # 可选: 指定解释器; 默认自动选可用的 venv/系统 python
# command = "main.py"
# args = ""                                # 可选附加参数, 如 "--once"

[log]
level = "info"
file_level = "info"
file = "logs/lotsacg.log"
```

---

## Telegram 指令速览

| 指令 | 说明 |
| --- | --- |
| `/start` | 开始使用 |
| `/help` | 显示完整帮助 |
| `/setu` | 随机图片（**始终 R18**） |
| `/random` | 随机全年龄图片（支持 `或\|与` 逻辑标签筛选） |
| `/search` | 文本/以图搜索 |
| `/info` | 发送作品图片和信息 |
| `/files` | 获取作品原图 |
| `/hybrid` | 混合搜索 |
| `/similar` | 搜索相似作品 |
| `/r18mode` | 设置 R18 过滤模式 (on/off/mixed) |
| `/groupsearch` | 在群组中搜索作品 |
| `/downloadzip` | 打包下载回复图片 (zip) |
| `/tagging` | 识别回复图片中的标签 |
| `/follow` `/unfollow` `/followlist` | 关注画师 / 取消关注 / 查看关注（新作自动推送） |
| `/sub` `/unsub` `/sublist` | 订阅标签 / 取消订阅 / 查看订阅（新作自动推送） |
| `/status` | 查看机器人状态（作品数 / 队列 / 版本等） |

**管理员指令**：`/addadmin` `/deladmin` `/delete` `/r18` `/title` `/tags` `/addtags` `/deltags` `/post`（批量发布，支持画师主页展开）`/cancel` `/cd` `/refresh` `/tagalias` `/autotag` `/dump` `/recaption` `/reindex` `/dupcheck` `/redescribe` `/update` `/xppusher`（管理 XP-Pusher 进程）

---

## 从 v0 迁移

若你之前使用 v0 版本，下载最新 v0.x release，在配置中添加：

```toml
[migrate]
target = "sqlite" # pgsql/mysql/sqlite
dsn = "file:lotsacg_migrate.db"
```

然后运行：

```bash
./lotsacg db migrate
```

---

<div align="center">

## ⭐ 支持项目 ⭐

如果你觉得 LotsACG 好用，请给仓库点个 **Star**！

[![GitHub stars](https://img.shields.io/github/stars/wwwangzilin/LotsACG?style=social)](https://github.com/wwwangzilin/LotsACG)

[→ Star LotsACG ←](https://github.com/wwwangzilin/LotsACG)

感谢你的支持！❤️

</div>
