# 网易云音乐 · MyGo

用 Go 和 [MyGo](https://github.com/egoist/mygo) 写的第三方网易云音乐桌面客户端，原生界面，无 WebView。

## 功能

- 不必登录也能播放：先按网易云播放，它播放不了的歌曲（VIP 歌曲、下架的歌曲）再由ChKSz音源解析
- 扫码登录（可选），登录态保存在系统凭据存储中；登录后每日推荐与歌单跟着你的口味
- 不必登录也能用歌单：新建、改名、删除、加歌与移除都在本机完成，歌单存在用户数据目录的 `playlists.json`；登录后本地歌单与云端的歌单并列在「我的歌单」里
- 收藏别人的歌单也不必登录：收藏存在本机，列在侧边栏「收藏的歌单」里；登录后收藏仍存到网易云
- 歌曲的红心「喜欢」同样不要求登录：未登录时喜欢存进本机歌单「我喜欢的音乐」，退出后再打开照旧；登录后红心跟云端走
- 每日推荐、推荐歌单、排行榜、我的歌单
- 搜索单曲、歌手、专辑、用户与播客，输入时显示搜索建议，保留搜索历史
- 歌手页的热门与最新歌曲、专辑与动态，关注歌手；专辑页与用户页
- 播客：浏览与收听节目，订阅播客；在本机把节目转写成字幕（Qwen3-ASR），像歌词一样随播放滚动
- 私人 FM（推荐 / 熟悉 / 探索）与心动模式
- 边下边播，支持标准 / 较高 / 极高 / 无损（FLAC）/ Hi-Res 音质
- 听过的歌曲缓存在本机，再次播放无需等待
- 音量均衡，暂停与继续时淡出淡入，拖动进度无杂音
- 退出后再打开，从上次的播放队列与进度继续
- 逐字歌词与翻译，正在播放页的「律动」视图与歌曲评论
- 刘海播放器：在屏幕顶部刘海处显示迷你播放器，悬停展开（在设置中开启）
- 喜欢歌曲、播放队列、随机与循环播放
- 浅色 / 深色 / 跟随系统

## 构建

需要 Go 1.27+。

```bash
go tool mygo dev     # 开发模式运行
go tool mygo build   # 打包到 build/
go test ./...        # 运行测试
```

`go tool mygo build` 默认用 ad-hoc 签名，产物只能本机用；要正式分发得有自己的 Developer ID 证书。签名、公证与打包的细节见 [docs/packaging.md](docs/packaging.md)。

代码怎么分层、功能落在哪个文件、一次播放经过哪些环节，见 [docs/architecture.md](docs/architecture.md)。

## 致谢

- [MyGo](https://github.com/egoist/mygo)：原生 UI 与应用框架
- [NeteaseCloudMusicApi](https://github.com/Binaryify/NeteaseCloudMusicApi)：eapi 加密与登录流程的参考
- [folia-major](https://github.com/chthollyphile/folia-major)：歌词动画效果的参考
- [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx)、[Qwen3-ASR](https://github.com/QwenLM/Qwen3-ASR) 与 [Silero VAD](https://github.com/snakers4/silero-vad)：本机语音识别

本项目与网易云音乐官方无关，音乐版权归各权利人所有。

## 许可证

[AGPL-3.0](LICENSE)
