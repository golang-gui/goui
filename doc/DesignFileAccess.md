# 文件访问凭据与 macOS 沙箱

## 目标与边界

FileDialog 从纯路径结果升级为拥有访问资源的 FileAccess，同时将平台错误贯穿
GUI/UI。文件与目录共用访问模型，不建立 StorageProvider、虚拟文件系统、全局
权限管理器或隐式提权。应用目录复用 os.UserConfigDir/os.UserCacheDir，读写复用
Go os/io；外部文件访问资格单独管理。Android/iOS、Flatpak Portal、提权执行、
公证和商店提交不在本切片承诺内。

## 公共契约

```go
type FileAccess interface {
    Name() string
    LocalPath() (string, error)
    Credential() ([]byte, error)
    Close() error
}
type FileDialog interface {
    OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
    OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error))
    SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
    RestoreAccess(credential []byte) (FileAccess, error)
}
```

Platform/GUI 对象操作线程亲和；UI 返回轻量包装，在所属 GUI 线程操作资源。
取得路径后可在后台读写，但所有读写结束前不能 Close。Name 是显示名，关闭后
仍可查询；其他操作关闭后返回 os.ErrClosed。当前桌面 LocalPath 返回绝对路径，
未来无路径资源应明确返回不支持，不提前承诺移动端内容 URI 可转路径。

- 成功结果所有权交给调用方，不随对话框或 Window 关闭而释放。
- Close 幂等，释放引用和本对象开启的访问，不删除文件，不保证撤销系统另行
  授予的全部权限，不靠 GC finalizer 决定释放时机。
- Credential 是用于定位资源和恢复访问的不透明字节，应用自行持久化，不是
  密码、证书或永久授权。带版本、后端、格式标识，拒绝损坏/过大/不匹配输入。
- RestoreAccess 不隐式弹选择器、不请求管理员权限。恢复后 Credential 返回
  当前数据；macOS 书签 stale 时更新，由应用写回存储，失败交给调用方。
- SaveFile 只选择目标、不创建文件。新目标尚不存在时 macOS 制作凭据可以
  失败，应实际保存后制作，不为书签偷偷创建空文件。
- 回调恰好一次：成功返回对象；取消为空结果/nil error；创建、不支持、原生
  操作或结果转换失败返回 error。多选转换失败清理全部已取得资源，不返回半份
  成功结果。GUI 保存 NewFileDialog 创建错误，不能将失败伪装成取消。

## 后端实现

Windows/当前非沙箱 Linux 返回轻量路径对象；Close 无系统授权操作。凭据保存
绝对路径，恢复检查资源仍存在，不承诺新权限或跟踪移动。共享编码/路径逻辑放
platform/internal/fileaccess，不放 core 或绘图后端。

macOS 保留 NSOpenPanel/NSSavePanel 的原始 NSURL，在 pool 结束前 retain，
配对释放。扩展现有 NSURL/NSData 绑定，NSError 和 Security 框架绑定遵循既有
类、selector、类型方法和框架初始化规范，cocoa 不建立平行绑定。通过 SecTask
查询当前进程签名里的 app-sandbox，不根据环境、路径或自定义 plist 猜测。
沙箱使用安全作用域书签，非沙箱使用普通书签；原始/恢复 URL 按原生 start/stop
契约管理，不把“不需要额外作用域”误报为拒绝，不将沙箱书签失败降级成路径。
恢复使用 withoutUI，过期书签重新生成。
只读 user-selected entitlement 使用 AllowOnlyReadAccess 创建安全作用域凭据；
不因申请持久访问而扩大原来的读写权限。

## 构建打包

cmd/goui 增加 macOS sandbox、user_files（read-only/read-write）、持久访问凭据
开关及 signing_identity。generate 生成独立 Entitlements；package 在 macOS
将 Entitlements 签入 .app 并校验签名，签名失败不发布目标目录。沙箱最终产物
不能只生成配置而不签名；普通未签名 app 保持原行为。签名身份明确配置，开发
可以显式选择 ad-hoc `-`（不承诺分发/跨构建身份稳定）。不自动选择/安装证书、
上传公证或扩大网络/系统目录权限。

## 实施与验收

- [x] 公共接口、凭据编码、路径对象及无桌面回归。
- [x] macOS 书签、原生生命周期、实际签名沙箱检测。
- [x] GUI/UI 错误传播与线程包装、调用方/替身迁移。
- [x] cmd/goui 配置、Entitlements、签名及失败保护。
- [x] 中文 tests/window/gui/file_access：选择/取消/保存/目录/多选与跨启动恢复。
- [x] 本机/远端测试与目标编译已执行；通过、既有失败和缺口见下文，不宣称全部通过。
- [x] 本机 Linux 和远端 Windows/macOS 真实对话框验收。
- [x] macOS open 启动签名沙箱 app，验证 HOME/标准目录、内部保存、外部文件
  和目录访问、退出重启恢复及显式关闭；丢失资源失败由原生无桌面回归覆盖。
- [ ] 稳定开发/发行证书的跨构建恢复、沙箱中删除/移动资源、真实 Retina/混合 DPI。

测试只读写独立临时文件/目录，不操作用户业务文件。远端源码/产物位于独立
临时目录，不覆盖工作区。桌面验收不依赖 DevServer，SSH 成功不代表沙箱通过。

## 实施与验证记录（2026-10-11）

接口是有意的兼容性变更：原来 `[]string` / GUI/UI 无错误的回调改为
`func([]FileAccess, error)`。先区分错误与取消，再取 LocalPath；处理完毕 Close。
demo 保留用于后续拖放/打开操作的访问资源，替换选择或退出时释放。长期配置
存放 Credential，不存 FileAccess 对象；新建 Save 目标实际写入后再生成凭据。

- 本机 Linux amd64、Go 1.25.8：`go test ./...` 和最终非缓存
  `go test ./... -count=1` 通过。fileaccess/gui/ui/cmd/goui 的 race 通过。
  普通 `go vet` 被既有 gui 文本/Popover 和 ui 文本测试的 unkeyed composites
  警告阻挡；相同范围 `go vet -composites=false` 通过，没有改写旧测试掩盖告警。
- 远端 macOS 15.7.7 arm64、Go 1.24.9：最终非缓存全仓通过。Foundation 原生
  书签创建、30 次恢复/释放、Unicode 文件身份、关闭与丢失资源回归通过；原生
  codesign 签名、Entitlements 核查、错误身份不发布产物的命令包测试通过。
- 远端 Windows 10 22H2 amd64、Go 1.25.8：受影响 fileaccess/shell/win32/gui/ui/
  cmd/goui 包通过。全仓失败项为 Direct2D TestPremultipliedNativeColors 的四个
  gradient/stroke-gradient 断言（got `{64 0 64 128}`，want `{32 0 96 128}`）和
  platform/test 的四个窗口/Popup 绘制超时。独立解包修改前 HEAD `5738b83`，
  在同一 SSH Session 0 执行 `go test ./platform/graphics/direct2d ./platform/test`，
  复现完全相同失败；未把桌面环境限制或既有失败算作本次通过，也未修改绘图代码。
- Windows amd64/macOS arm64 的 demo 与窗口例程编译通过；Linux arm64 新增
  fileaccess 逻辑测试目标编译通过，但完整窗口例程仍被既有 libc
  `syscall.SYS_PIPE` / `SYS_POLL` 未定义阻挡，该文件与修改前 HEAD 一致。

真实桌面使用公开 GUI 接口，截图和 results.log 中的独立读写/生命周期断言
共同验收，不依赖 DevServer：

- Linux X11/Xvnc/Xfwm4，Software 1x：取消、单选、多选两文件、目录、Save 新
  目标未被选择器创建/随后写入、凭据和显式关闭通过。最终程序 OpenGL 1x
  重启恢复、关闭、临时移动自建资源后的失败通过；随后恢复测试文件。
- Windows RDP 桌面 1x、默认绘制配置：取消、单选/多选/目录、保存新目标、
  重复关闭、损坏凭据拒绝与跨进程恢复通过。未分别验收所有 Painter 后端。
- macOS VNC 桌面 1x、默认绘制配置：通过 open 启动实际带沙箱签名的 .app，
  HOME/UserConfigDir/UserCacheDir 都在应用容器；容器内保存日志/凭据，选择
  外部文件和目录、两文件多选、保存新目标并写入、显式关闭、退出重启恢复通过。
  单独只读配置的签名 Entitlements、外部读取、凭据生成与退出重启恢复也通过。
  不把这些场景扩大成所有后端、权限收窄组合或移动端支持保证。

实际操作暴露并修正了两处旧问题：Linux helper 的成功 stderr 警告曾被
CombinedOutput 混入文件路径，改为只读取 stdout 并保留故障 stderr；Windows
HRESULT_CANCEL 常量修正为 `0x800704C7`。分别新增进程边界回归和独立 HRESULT
位值回归，真实取消复验通过。macOS 非 Cancel 的 modal response 按错误上报。

签名验收使用显式 ad-hoc `-`，不是关闭沙箱或跳过签名校验。现有开发身份在
SSH codesign 返回 `errSecInternalComponent`，未解锁钥匙串或改系统安全设置；
稳定证书跨更新仍待应用发行环境验收。App Store、Notarization、Gatekeeper、
Linux Portal、移动端以及提权均不适用本切片。测试仅产生自己的临时文件、应用
配置与远端临时源码/签名产物，保留供核查，没有覆盖用户项目或业务文件。

## 参考与取舍

核对日期 2026-10-11。以下是文档/源码依据，实测完成后另记录 OS/会话/签名方式。

- [Avalonia Storage Items](https://docs.avaloniaui.net/docs/services/storage/storage-item)：采用资源归属与可持久化引用，不照搬完整存储抽象；当前兼容性文档仍注明桌面书签限制。
- [Electron dialog](https://www.electronjs.org/docs/latest/api/dialog#bookmarks-array) / [作用域访问](https://www.electronjs.org/docs/latest/api/app#appstartaccessingsecurityscopedresourcebookmarkdata)：显式保存/恢复/释放；Go 用 Close，不返回清理函数。
- [Apple 沙箱文件访问](https://developer.apple.com/documentation/security/accessing-files-from-the-macos-app-sandbox) / [书签恢复](https://developer.apple.com/documentation/foundation/nsurl/urlbyresolvingbookmarkdata%3Aoptions%3Arelativetourl%3Abookmarkdataisstale%3Aerror%3A)：原始 NSURL、作用域生命周期、stale 更新与签名契约。
- [Go os](https://go.dev/src/os/file.go)：Darwin 标准目录依赖 HOME，签名 app 需实际验证启动环境。
