# 功能结构

MyGo 云音乐是一个 Go 写的桌面客户端：`main.go` 只调用 `app.Run()`，界面由 [MyGo](https://github.com/egoist/mygo) 的 `ui` 包直接绘制，没有 WebView。除网易云与 ChKSz 的接口外，音频下载、解码、歌词排版、封面取色和播客转写都在本机完成。

本文说明代码怎么分层、功能落在哪个文件、一次播放经过哪些环节，以及加功能时该改哪里。打包与发版见 [packaging.md](packaging.md)。

## 分层

```
internal/app         界面、路由、播放状态、设置、字幕调度
├── internal/netease 网易云接口：eapi、登录、目录、播放地址
├── internal/chksz   备用音源（用 netease.Stream）
├── internal/player  队列、边下边播、解码、缓存（用 netease、lyrics）
├── internal/lyrics  歌词解析
└── internal/speech  本机语音识别，为播客写字幕
```

依赖是单向的：`app` 依赖其余全部；`player` 与 `chksz` 借用 `netease` 的 `Track`、`Stream` 类型，`player` 再借用 `lyrics.Line`。没有包反向依赖 `app`，所以这些包都能单独测试。

| 包 | 职责 | 主要文件 |
| --- | --- | --- |
| `internal/app` | 全部界面与状态，约 1.2 万行 | `app.go` 状态与启动、`ui_main.go` 骨架、`ui_pages.go` 页面分发、`ui_nowplaying.go` 正在播放 |
| `internal/netease` | 网易云 Web API：eapi 加密、登录、目录、播放地址、评论 | `eapi.go`、`auth.go`、`netease.go`、`catalog.go`、`playlist.go`、`radio.go`、`podcast.go` |
| `internal/player` | 播放队列、边下边播、MP3/FLAC 解码、音量与淡入淡出、缓存 | `player.go`、`stream.go`、`flac.go`、`cache.go`、`fade.go` |
| `internal/chksz` | 网易云播不了时的备用音源，带限额与缓存 | `chksz.go` |
| `internal/lyrics` | LRC / YRC 解析、逐字切分、翻译、间奏识别 | `lyrics.go` |
| `internal/speech` | 本机语音识别：Silero VAD + Qwen3-ASR，经 sherpa-onnx | `speech.go`、`sherpa.go`、`model.go` |
| `scripts/sherpa` | 打包 sherpa-onnx 动态库并生成 `natives.go` 的清单 | `main.go` |
| `icon/` | 应用图标（Icon Composer 的矢量层）与菜单栏图标，由 `go run ./icon` 绘制导出 | `main.go`、`tray/main.go` |

## 进程与线程

单进程，主线程拥有 `app` 的全部状态，视图也在主线程构建。HTTP、封面解码、歌曲下载、语音转写都在 goroutine 上跑，结果经 `app.post` 回到主线程；`post` 同时重绘主窗口、设置窗口和刘海面板。

`player` 的回调都回到主线程：`Changed` 触发会话保存与字幕同步，`Quality`、`Normalize` 每次播放前现读设置，所以改设置不必重建播放器。

启动顺序（`app.Run`）：

1. `moveOldData` 把旧名字下的数据目录搬到当前名字下；
2. `WhenReady` 里读设置与本机的歌单（`playlists.json`）、应用外观、按设置建 ChKSz 客户端；
3. 建封面缓存与播放器，接上 `Resolve`（音源解析）、`Cache`、`Changed`；
4. 建主窗口与菜单，`boot()` 在后台恢复登录态，再 `newNotch`（仅 macOS）、同步 Dock 与托盘；
5. `OnBeforeQuit` 保存设置、音量与播放会话。

窗口与菜单栏：

- 主窗口 1180×780（最小 880×580），标题栏隐藏、侧边栏毛玻璃；
- 设置窗口独立，560×720，关闭即销毁，下次打开重建；
- 关闭主窗口只是隐藏，音乐继续；Linux 上没有托盘时关闭即退出；
- 应用菜单是「关于 / 设置… / 服务 / 隐藏 / 退出」，另有文件、编辑、控制、窗口四个菜单；控制菜单里有播放、切歌、喜欢、随机、循环和「正在播放」（`⌘L`）的快捷键。

## 界面结构

主窗口分三段：左侧边栏、右侧页面、底部播放条。页面内容滚动时从悬浮的顶栏和播放条下方穿过，两者按滚动位置显示边缘的渐隐。

```
┌─────────────┬───────────────────────────────┐
│搜索框       │     悬浮顶栏：返回 / 前进     │
│             │                               │
│为你推荐     │                               │
│每日推荐     │           页面内容            │
│私人 FM      │          （可滚动）           │
│排行榜       │                               │
│播客         │                               │
│我的歌单     │                               │
│收藏的歌单   │                               │
│             │                               │
│正在播放卡片 ├───────────────────────────────┤
│设置         │                               │
└─────────────┴───────────────────────────────┘
```

「正在播放」是盖住整页的覆盖层（`ui_nowplaying.go`）。打开后页面不再构建，省掉整页的每帧开销；页面的滚动位置和列表状态存在 `app` 里（`pageScroll`、`navScroll`、`trackList`），关掉覆盖层后原样恢复。

### 路由

```go
type route struct {
    page  pageKind // 哪个页面
    id    int64    // 歌单、歌手、专辑、用户或播客的 ID
    query string   // 搜索词
    kind  netease.SearchType // 搜索类型
}
```

`open` 把当前路由压进 `history` 再切换；`back` / `goForward` 对应鼠标侧键、触控板横扫和 `⌘[` / `⌘]`。侧边栏的「搜索结果」只有搜过才出现，回到它时恢复上次的关键词与类型。

### 页面

| 页面 | 内容 | 加载 |
| --- | --- | --- |
| `pageHome` 为你推荐 | 问候与日期、每日推荐卡、今日推荐歌单、官方榜单、推荐歌单 | `loadHome`；窗口重新获得焦点时检查，网易云在北京时间 6:00 换新后重新加载 |
| `pageDaily` 每日推荐 | 当日歌曲、播放全部、随机播放 | `netease.DailySongs` |
| `pageCharts` 排行榜 | 官方榜与各类榜单卡片 | `netease.Charts` |
| `pagePlaylist` 歌单 | 封面、作者、歌曲列表；自己的歌单可改名、删歌、删除；可收藏/取消收藏（未登录也能收藏，存在本机）；可开启心动模式。本机的歌单标着「本地歌单」，没有收藏与心动模式 | `loadPlaylist`（本机的直接取自 `a.local`） |
| `pageSearch` 搜索 | 单曲、歌手、专辑、歌单、用户、播客，分页加载 | `loadSearch` |
| `pageFM` 私人 FM | 推荐 / 熟悉 / 探索三种模式，播放时显示大封面 | `netease.PersonalFM` |
| `pageArtist` 歌手 | 热门、最新、专辑、动态四个标签，可关注 | `loadArtist`、`loadLatest`、`loadAlbums`、`loadEvents` |
| `pageAlbum` 专辑 | 专辑信息与曲目 | `loadAlbum` |
| `pageUser` 用户 | 主页与创建的歌单；自己的主页另有订阅的播客与关注的歌手 | `loadUser` |
| `pagePodcast` 单档播客 | 简介、节目列表、订阅、节目喜欢 | `loadPodcast` |
| `pagePodcasts` 播客发现 | 推荐、热门、订阅三个列表 | `loadPodcastTab` |

页面之外还有三个浮层：扫码登录、歌单命名、删除确认；`a.toast` 显示一次性提示。

### 正在播放

标签按播放内容切换，`⌘1` 起按顺序选择：

| 播放歌曲 | 播放播客节目 |
| --- | --- |
| 歌词、律动、待播清单、选歌、评论 | 简介、字幕（支持的平台才有）、待播清单、评论 |

- **歌词**：逐行滚动，当前行高亮，支持翻译；`Esc` 关闭覆盖层。
- **律动**：Folia 风格的逐字动画，三种样式「流光 / 云阶 / 倾诉」，可再开单词火花；两者都在律动页的「律动效果」菜单里切换。
- **选歌**：待播清单铺成封面墙，可拖动、甩动、触控板滑动，点封面即播放。
- **评论**：先精彩评论、后最新评论，可点赞，滚到底自动加载下一页；评论数变化时标签宽度做滚动动画。
- **字幕**：播客在本机转写的结果，按时间轴像歌词一样滚动。

歌曲的间奏（前奏、间奏、尾奏）里，歌词位置可以换成这首歌的热门评论（`ui_quiet.go`，设置里可关）。

## 功能地图

| 功能 | 入口 | 实现 |
| --- | --- | --- |
| 播放 / 暂停 | 空格、播放条、媒体键、控制中心 | `player.TogglePlay`，`mygo.NowPlaying` |
| 上一首 / 下一首 | 播放条、媒体键 | `player.Prev` / `Next` |
| 随机 / 循环 | 播放条 | `player.ToggleShuffle` / `CycleRepeat` |
| 拖动进度 | 播放条、正在播放大封面 | `player.Seek`，落点前后淡出淡入 |
| 音量 / 静音 | 播放条音量弹层、滚轮 | `player.SetVolume` |
| 喜欢歌曲 | 播放条、列表右键 | `netease.Like`，`ui_like.go` |
| 加入 / 移出歌单 | 歌曲右键菜单 | `ui_songmenu.go`；云端的走 `netease.AddToPlaylist`，本机的走 `localplaylists.go` |
| 新建 / 改名 / 删除歌单 | 歌单页、侧边栏、文件菜单 | 登录时 `netease.CreatePlaylist`、`RenamePlaylist`、`DeletePlaylist`；未登录时写本机的 `playlists.json`（`localplaylists.go`） |
| 收藏歌单 | 歌单页 | `netease.SubscribePlaylist` |
| 关注歌手 / 订阅播客 | 歌手页、播客页 | `netease.FollowArtist`、`SubscribePodcast` |
| 心动模式 | 歌单页（「喜欢的歌单」除外） | `netease.HeartMode`，作为电台续播 |
| 私人 FM | 侧边栏 | `netease.PersonalFM`，三种模式 |
| 搜索建议与历史 | 搜索框 | `netease.SearchSuggest`，历史存设置 |
| 扫码登录 | 设置 → 账号 | `netease.QRKey` / `QRCheck` |
| 每日推荐 | 侧边栏、首页卡片 | `netease.DailySongs` |
| 实时活动 / 刘海播放器 | 设置开启，macOS | `ui_notch.go`、`notch_darwin.go` |
| 菜单栏 / 通知区图标 | 自动，或设置里的应用图标 | `tray.go` |
| 播客字幕 | 正在播放 → 字幕 | `captions.go` + `internal/speech` |

## 播放链路

从点一首歌到出声：

1. **入队**：`player.PlayQueue(tracks, start)` 记下队列与播放顺序；随机只改顺序不改队列，循环有「关 / 列表 / 单曲」三档。
2. **先查缓存**：`player.open` 先看 `songs/` 里有没有这首、这个音质；命中就直接解码，不解析地址，也不下载音频。
3. **解析地址**：没命中才问网易云，失败再问 ChKSz。
   - `netease.SongURL`：匿名也能拿到免费歌（`fee=8`）的地址，VIP 或下架的歌返回 `ErrUnavailable`；
   - 失败且 ChKSz 可用：`chksz.Client.SongURL`，返回 `netease.Stream`；
   - ChKSz 报 `BlockedError`（额度用尽、Key 失效、限流）：包成 `player.StopError`，整个队列停下，不逐首重试；
   - 两边都没有：`errNoSource`，提示未登录且没有 ChKSz Key。
4. **取数据**：`startDownload` 边下边播——读取方等自己要的字节，播到哪下到哪。缓存会留的歌曲写进 `songs/` 的临时文件，下完改名为正式条目；缓存关掉时放内存。
5. **解码**：按扩展名走 `mp3Stream` 或 `flacStream`，统一输出 32 位浮点立体声给 `audio.Player`。拖动进度时先淡出、落点、再淡入，避免爆音；加载中拖动会记下 `pending`，加载完直接落到那里。
6. **出声**：`audio.SetLatency(LatencyPlayback)` 提前混音，macOS 换外观卡住主线程时音乐不断。
7. **系统集成**：`mygo.NowPlaying` 同步歌名、歌手、封面、进度到媒体键与控制中心；`Changed` 回调顺手保存会话、同步字幕。

### 音量均衡

`Normalize` 打开时，按网易云给每首歌测的响度算一个增益（`gain`），在解码输出前乘上去，让不同年代的歌听起来一样响。切换歌曲时增益直接生效，不改系统音量。

### 电台式续播

FM 与心动模式用同一套「电台」：`PlayRadio` / `AttachRadio` 记下电台名和一个 `feed`，队列快放完时向 `feed` 要下一批。`radioGen` 计数，切走电台后旧 feed 迟到的结果会被丢掉。会话里存电台名，恢复播放时重新接上。

### 缓存

| 缓存 | 位置 | 上限 | 淘汰 |
| --- | --- | --- | --- |
| 歌曲 | 缓存目录 `songs/`，每首 `<id>-<level>.mp3` 或 `.flac`，旁边 `.json` 记流信息 | 设置里 0 / 1 / 2 / 5 / 10 GB | 最久没播放的先删 |
| 封面 | 内存，含原图、模糊背景与主色 | 64 MB | 最久没看的先丢；仍被正在播放持有的用弱引用留着 |
| 字幕 | 缓存目录 `transcripts/<节目ID>.json` | 不设上限 | 模型换了就作废 |

歌曲缓存命中时不再解析播放地址，音频也不重新下载，只有封面与歌词照旧请求。

### 会话恢复

播放位置、队列、顺序、随机与循环状态写进 `session.json`：播放中每 30 秒一次，退出时再写一次，写入用 `.part` 加改名保证原子。启动后等登录态恢复再读，接着上次暂停的位置；有电台就重新接上 feed。

### 本地歌单

歌单不登录也能用。本机的歌单存在用户数据目录的 `playlists.json`（`localplaylists.go`），歌单 ID 为负，网易云的不会，于是同一套页面、侧边栏与右键菜单都照用，只在增删改时按 ID 的正负决定写本机还是发请求。每首歌连同歌名、歌手、专辑与封面一起存，播放时照旧解析流地址；存的时候丢掉 `Privilege`，那是当时登录态下的判断，留下会过时。

侧边栏的「我的歌单」先列网易云的，再列本机的；未登录时只剩本机的，一个都没有时给出一行「创建第一个歌单」。登录后新建仍存到网易云，本机的那些留着，登录或退出都不动它们；各页面清空缓存时（`signedIn`），本机歌单的页面从 `a.local` 重新构建，不发请求。

「收藏」也分两条路：登录后点收藏照旧请求网易云；未登录时存进同一个 `playlists.json` 的 `saved` 里（只存歌单的封面、名字与作者，歌曲照旧按需向网易云取，歌单页面公开可得），侧边栏「收藏的歌单」把云端的与本机的合在一起列出，同 ID 只列一次。登录之前收藏的歌单在登录后仍归本机管，取消收藏时也不发请求。

红心「喜欢」同理：未登录时 `toggleLike` 走 `likeHere`，把歌写进本机的「我喜欢的音乐」（`Special` 为 5、ID 为 `likesLocalID`，与网易云的那份同名同义），第一次喜欢时才建这个歌单；启动时 `seedLiked` 用它填满 `a.liked`，退出登录时再填一次，登录后 `a.liked` 仍由 `LikedIDs` 决定。它是心形按钮的歌单，不是拿来编排的：`editable` 对它返回 nil，所以没有改名、删除、从歌单移除，页面也不给心动模式（那是网易云按它的喜欢算的）。播客节目的喜欢没走这条路，仍然要登录。

## 网易云客户端

- **eapi**：请求体按 eapi 规则加密（`eapi.go`），响应是 JSON。
- **登录**：扫码登录拿到 `MUSIC_U` cookie，之后所有请求带上。正式版存系统钥匙串（`mygo.Secrets`，服务为 bundle id，账户 `netease-session`）；开发版存数据目录的 `session` 文件，免得每次重建签名都被钥匙串追问。
- **恢复**：启动时 `RestoreSession` 装回 cookie jar，再 `Account()` 验证；过期就清掉，当作未登录。
- **登录后**：`signedIn` 清空各页面缓存，重新加载当前页与首页，载入喜欢的歌、我的歌单；若播放器空着，把最近在其他设备上听的歌接进来（`pickup.go`），暂停等待继续。

接口按用途分组：

| 文件 | 内容 |
| --- | --- |
| `netease.go` | 歌曲、歌单、歌手、专辑类型；每日推荐、榜单、推荐歌单、歌曲详情、歌词、播放地址、图片下载 |
| `catalog.go` | 搜索（六种类型）、专辑详情、歌手专辑、用户主页 |
| `auth.go` | 登录、登出、账号、我的歌单、喜欢列表、最近播放、喜欢歌曲 |
| `playlist.go` | 新建、改名、加歌、删歌、删除歌单 |
| `radio.go` | 私人 FM、不喜欢、心动模式、评论与评论点赞 |
| `podcast.go` | 播客、节目、推荐与热门、节目喜欢 |
| `subscriptions.go` | 关注歌手、订阅播客、收藏歌单 |
| `artist_events.go` | 歌手动态 |
| `search_suggest.go` | 搜索建议 |

## 音源回退：ChKSz

`internal/chksz` 把 ChKSz 的 `163_music` 接口包装成和 `netease.SongURL` 一样的签名，`app` 只在网易云失败后调用它。

**Key 的来源**（`internal/app/chksz.go`），按顺序：

1. 设置里填的（`settings.ChKSzKey`）；
2. 环境变量 `CHKSZ_API_KEY`；
3. 名为 `api.chksz.key` 的文件，依次在用户数据目录、应用资源目录、可执行文件所在目录、当前工作目录找；
4. 找到后复制进用户数据目录，下次无论从哪启动都能找到。

**限额保护**：

- 成功结果按 URL 里的失效时间戳缓存到失效前 30 秒（上限 30 分钟），期间同一首歌不重复请求；
- 「查不到」缓存 2 分钟，失败缓存 30 秒，避免抖动时反复打接口；
- 每分钟请求数从响应头 `X-RateLimit-Limit` 自适应（默认 20），窗口满了最多等 3 秒，再满就提示「请求太频繁」；
- 日配额看 `X-Quota-Free-Remaining` / `X-Quota-Paid-Remaining`，用尽、401、402、403 当天不再请求；
- 429 按 `Retry-After` 停；
- 跳转只允许同主机，因为 Key 在 query 里，不能被带到别处。

额度用尽、Key 被拒或请求太频繁时返回 `BlockedError`，播放器收到后停止整个队列，而不是把后面每首歌都撞一遍墙。

## 歌词与视觉

`lyrics.Build(lrc, translation, yrc)` 把三种歌词合成 `[]Line`：每行有起止时间、原文、翻译，YRC 还会拆出逐字时间（`Words`）。`Index` 二分找当前行，`Quiets` 找长于阈值的间奏，`IsCredit` 过滤「作词 / 作曲」这类制作信息。

视图分三层：

- **歌词页**：整页滚动，当前行放大高亮，上下行渐隐；
- **律动**：逐字上浮与发光，样式在 `ui_stage_styles.go`，有流光、云阶、倾诉三套排版与火花效果；
- **选歌墙**：封面按网格铺开，拖动带惯性，播放时每排随节拍轻微摆动。

封面取色（`artwork.go`）给每张封面算主色和模糊背景，正在播放的背景、首页每日推荐卡和刘海都用它，切歌时颜色平滑过渡。实验性的「烟雾背景」与「进度条烟雾」分别在 `backdrop.go`、`ui_player.go`，默认关闭。

## 播客与字幕

播客节目是 `netease.Program`，转成 `Track` 后走同一条播放链路，只是 `ProgramID` 非零，界面据此换成「简介 / 字幕」标签。

字幕完全在本机：

1. 首次使用下载模型（Qwen3-ASR-0.6B int8 + Silero VAD + sherpa-onnx 动态库，约 1 GB），存用户数据目录的 `models/`；先试 ModelScope，再试 Hugging Face，逐文件校验 SHA-256，支持断点续传；
2. 播放时从当前位置往后转写；拖动进度后，如果已写好的内容还在落点前 30 秒以内，就等它追上，否则从落点重新起一段；
3. 结果按时间段存进 `transcripts/<节目ID>.json`，下次打开直接读；换模型名则旧字幕作废；
4. 转写只占播放线程以外的 goroutine，识别器一次只处理一个节目，新任务等旧的停下。

macOS 的动态库要签名才能在硬化运行时下加载，`scripts/sherpa` 负责打包、签名并生成 `internal/speech/natives.go` 里的文件名与摘要。

## 设置

设置窗口按组排列，改动即时保存并生效（`ui_settings.go`、`settings.go`）：

| 分组 | 项 | 说明 |
| --- | --- | --- |
| 通用（仅 macOS） | 应用图标 | 程序坞与菜单栏 / 仅程序坞 / 仅菜单栏 |
| 刘海播放器（仅 macOS） | 显示、实时活动、无刘海屏常驻 | 刘海处的迷你播放器 |
| 播放 | 音质、间奏热评、音量均衡 | 音质下一首生效 |
| 音源 | 使用 ChKSz、API Key | 关掉即只走网易云 |
| 播客 | 字幕开关、模型下载 / 删除 | 仅在支持语音识别的平台出现 |
| 缓存 | 上限、清除 | 显示已用空间 |
| 外观 | 跟随系统 / 浅色 / 深色 | 立即应用 |
| 实验性功能 | 进度条烟雾、烟雾背景 | 默认关闭，费电 |
| 账号 | 扫码登录 / 我的主页 / 退出 | 未登录也能听免费歌 |

## 本地数据

macOS 上的位置（其他平台由 MyGo 的 `Path` 决定）：

| 内容 | 路径 |
| --- | --- |
| 设置 | `~/Library/Application Support/MyGo 云音乐/settings.json` |
| 播放会话 | `~/Library/Application Support/MyGo 云音乐/session.json` |
| ChKSz Key | `~/Library/Application Support/MyGo 云音乐/api.chksz.key` |
| 语音模型 | `~/Library/Application Support/MyGo 云音乐/models/` |
| 开发版登录 | `~/Library/Application Support/MyGo 云音乐/session` |
| 歌曲缓存 | `~/Library/Caches/MyGo 云音乐/songs/` |
| 字幕 | `~/Library/Caches/MyGo 云音乐/transcripts/` |
| 登录凭据（正式版） | 系统钥匙串，服务为 bundle id，账户 `netease-session` |

数据目录跟 `mygo.json` 的 `name` 走，不跟 bundle id：换标识不会丢设置和登录。`scripts/reset.sh` 清掉设置、登录与语音模型，回到首次启动的状态；加 `--songs` 连歌曲缓存与字幕一起清，加 `--keep-login` 保留登录。

## 测试与调试

| 层次 | 做法 |
| --- | --- |
| 纯逻辑 | 直接调包：歌词解析、队列与随机、淡入淡出、缓存淘汰、解码、模型校验 |
| 界面 | `ui.NewTester` 建视图，`newTestApp` 造应用，按无障碍标签点按、输入、按键，再断言应用状态 |
| 网络 | 设 `LIVE=1` 才跑真实接口（网易云、ChKSz、播放地址），平时全部跳过 |
| 帧预算 | `perf_test.go` 数空闲页面每秒请求的帧数，防止后台偷偷重绘 |
| 视觉 | `NM_FRAMES=<目录>` 存逐帧 PNG，`NM_SHOTS=<目录>` 走一遍各窗口截图；`MYGO_FRAME_STATS` 输出帧耗时 |

```bash
go test ./internal/...          # 提交前
go test ./internal/app -run TestName -count=5   # 新的界面测试，查抖动
```

界面测试不写「某个标签出现在屏幕上」这种重复视图代码的断言，只测行为：点了之后状态怎么变、播放器放的是不是那首。具体约定见 `AGENTS.md`。

## 加功能的落点

| 要加的 | 改哪里 |
| --- | --- |
| 一个新页面 | `app.go` 的 `pageKind` 与 `route` → `ui_main.go` 的 `content` 与 `load` → 新建 `ui_xxx.go` → 需要入口就加进侧边栏 `sidebar` |
| 正在播放的新标签 | `paneXxx` 常量 → `npTabs` → `choosePane` 的绘制分支 → `npTabs` 的顺序决定 `⌘数字` |
| 一个新设置 | `settings.go` 加字段与默认值 → `ui_settings.go` 加一行 → 用它的地方读 `a.settings`（播放器相关通过 `Quality` / `Normalize` 这类回调） |
| 新音源 | 实现 `Resolve func(id int64, level string) (*netease.Stream, error)`，在 `app.go` 里接给 `a.pl.Resolve`，失败语义参考 `streamURL` |
| 新接口 | `internal/netease` 加函数，`app` 里在 goroutine 调用，`post` 回主线程改状态 |
| 新平台相关代码 | 用 `_darwin.go` / `_windows.go` / `_other.go` 构建标签，参考 `notch_*.go`、`cputime_*.go` |
