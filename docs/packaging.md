# 打包与发版

`mygo.json` 是打包配置（版本号、产物目录、图标、签名），`go tool mygo build` 按它出包。框架的完整说明随模块下载，不在本仓库里：

```bash
open "$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)/docs"
```

`distribution.md` 讲各平台的产物，`cli.md` 讲参数。下面只记本项目自己的做法。

## 本机打包

`mygo.json` 的 `macos` 里不写 `signingIdentity` 时，`go tool mygo build` 用 ad-hoc 签名（等价于 `-sign -`），本机自用足够了：

```bash
go tool mygo build
```

> 上游仓库的 `mygo.json` 里原本写着 `"signingIdentity": "Developer ID Application: xinyu zeng (GJE9R5VE87)"` 和 `"notarize": { "keychainProfile": "notary" }`，那是原作者自己发版用的证书，本机钥匙串里没有。留着它构建会在签名那一步报 `no identity found`，所以已经从本仓库删掉。将来要用别人的配置，先确认那张证书在自己的钥匙串里（`security find-identity -p codesigning`）。

产物：

| 文件 | 说明 |
| --- | --- |
| `build/darwin-arm64/MyGo 云音乐.app` | 约 17 MB |
| `build/darwin-arm64/mygo-cloudmusic-<版本>-arm64.dmg` | 约 7 MB，拖进「应用程序」安装 |

没有 `updates` 配置时，`install.sh`（供应用自更新）和 `SHA256SUMS` 都不会生成，只出上面两个。

dmg 由 `hdiutil` 挂载临时卷制作，在沙箱或没有图形会话的环境里会失败（`hdiutil: create failed - 操作不被允许`），要在正常的桌面会话中执行；只要 `.app` 时加 `-skip-dmg`。

## 带证书打包（分发用）

要给别人下载、不被 Gatekeeper 拦，得有自己的 Apple Developer Program 账号（个人 99 美元/年）。拿到 `Developer ID Application: <名字> (<TeamID>)` 后，在 `mygo.json` 的 `macos` 里加回两行：

```json
"macos": {
  "signingIdentity": "Developer ID Application: <你的名字> (<TeamID>)",
  "notarize": { "keychainProfile": "notary" }
}
```

需要两样东西：

1. 钥匙串里有这张证书（Xcode 的 Accounts 里下载，或从开发者后台导入 `.cer`）；
2. 公证凭据存在 keychain profile `notary` 里：

```bash
xcrun notarytool store-credentials notary \
  --apple-id <Apple ID> --team-id <TeamID> --password <App 专用密码>
```

之后直接 `go tool mygo build`，会签名、公证并把票据 staple 进 dmg。只签名不公证（或反过来）都不行；临时只想签名，可以 `-skip-notarize`，但这样别人下载仍会看到「无法验证开发者」。

ad-hoc 签出来的包只能本机用：拷给别人会被 Gatekeeper 拦下，也没有公证票据。

## 其它平台

```bash
go tool mygo build -platform darwin/universal,linux/amd64
```

没有证书时不需要额外参数，默认就是 ad-hoc。MyGo 不需要 cgo，任意机器都能交叉编译；但 macOS 的签名与 dmg 只能在 macOS 上做，Windows 的安装包需要 NSIS，缺这些工具时构建会跳过并在输出里提示。

## 项目标识

```json
"identifier": "com.github.honghurumeng.cloudmusic"
```

`identifier` 是 bundle id（`CFBundleIdentifier`），改了等于换了应用身份：LaunchServices 的关联、通知授权、钥匙串里登录项的 service 都会重新来一遍。**用户数据不受影响**，因为数据目录跟着 `name` 走（`~/Library/Application Support/MyGo 云音乐/`），不跟 `identifier`。

项目不做自动更新：没有 `updates` 配置，客户端不会去查新版本，`mygo build` 也不再生成 `install.sh` 和更新清单。新版本由用户自己下载 dmg 覆盖安装。

还有一处仍指向上游：`internal/speech/natives.go` 的 `libBase`，语音相关的 sherpa-onnx 库从 `egoist/mygo-cloudmusic` 的 release 下载，校验和也写死在那里。要彻底独立，得先用自己的 release 放同一批 assets（`go run ./scripts/sherpa`，脚本里的 `repo` 常量也要改），再重新生成这个文件。不换也能用，只是依赖上游仓库不删这些 release。

## 发版清单

1. 改 `mygo.json` 的 `version`（打包读这里，不看 git tag）；
2. 在 `CHANGELOG.md` 顶部加一节；
3. `go tool mygo vet . && go test ./internal/...`；
4. 本机打包：`go tool mygo build`（默认 ad-hoc；配了自己的 Developer ID 才会签名并公证），检查：
   ```bash
   codesign --verify --deep --strict "build/darwin-arm64/MyGo 云音乐.app"
   hdiutil verify "build/darwin-arm64/mygo-cloudmusic-<版本>-arm64.dmg"
   ```
5. 提交，把 dmg 放到自己选的地方发布（GitHub Releases 手动上传、网盘都行）。
