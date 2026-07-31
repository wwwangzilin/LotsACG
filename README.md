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

### 🎯 智能推荐系统 `/recommend`

在**私聊**中智能推荐作品，完整移植 [Pixiv-XP-Pusher](https://github.com/krau/Pixiv-XP-Pusher) 的推荐算法：

- **XP 画像构建**：TF-IDF + 时间衰减权重算法，结合你的点赞/点踩偏好 + 群历史记录
- **Tag 权重系统**：从画像中提取高权重标签 + 常用组合（co-occurrence），按权重从高到低推荐
- **组合搜索**：用 top tag 组合在 Pixiv 搜索全新作品（AND 语义），单 tag 兜底 + RSS 拉新
- **综合排序**：匹配度评分（移植 calculate_match_score）+ 收藏数归一化
- **AI 精排**：可接入任意 OpenAI 兼容 API，用 LLM 对候选二次精排（喜爱概率）
- **Pixiv 全新作品**：推荐始终来自 Pixiv 新图，排除已发布/已看过的内容
- **匹配度展示**：每条推荐直接显示匹配百分比
- **推送到群**：每个推荐作品都带「📤 推送到群」按钮，一键发布到你的频道

```text
/recommend → 构建 XP 画像 (TF-IDF + 时间衰减)
          → 组合搜索 + 单 tag 兜底 → Pixiv 新图
          → 匹配度 + 收藏数 + AI 精排
          → 展示最优推荐 (👍/👎/⏭️/📤 推送到群)
```

### 🤖 AI API 接入 `[aiapi]`

- 支持任意 **OpenAI 兼容**接口（OpenAI / DeepSeek / Moonshot / 本地 Ollama 等）
- 自动为推荐生成关联/相似的 Pixiv 搜索标签
- 未配置时推荐功能降级为仅使用偏好标签，不影响使用

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

### 🛠️ 其他改进

- **上传稳定性修复**：解决发送图片时 `io pipe closed` 错误
- **构建脚本**：提供 Windows `build.bat`，自动填写版本号与 Git 提交
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
[rest.site]
title = "LotsACG - Kawaii is all you need"
desc = "ACG Image Collector and Gallery Server"
name = "LotsACG"
url = "https://example.com"
# [rest.limit]
# enable = true
# expiration = 60
# max = 100

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
| `/random` 或 `/setu` | 随机图片（支持 `或\|与` 逻辑标签筛选） |
| `/search` | 以图搜图 |
| `/info` | 发送作品图片和信息 |
| `/files` | 获取作品原图 |
| `/hybrid` | 混合搜索 |
| `/similar` | 搜索相似作品 |
| `/recommend` | **私聊**智能推荐（XP 画像 + AI 关联标签 + Pixiv 全新作品搜索，只推荐新图） |
| `/xp` 或 `/pref` | 查看你的 XP 画像（偏好标签权重） |
| `/tagging` | 识别回复图片中的标签 |

**管理员指令**：`/addadmin` `/deladmin` `/delete` `/r18` `/title` `/tags` `/addtags` `/deltags` `/post` `/refresh` `/tagalias` `/autotag` `/dump` `/recaption` `/reindex` `/dupcheck`

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
