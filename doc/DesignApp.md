# GOUI 应用入口、句柄与生命周期设计

应用身份接入更新：创建入口直接接收不可变的 `appId string`，不使用 Options。
运行时平台映射与手工图标打包说明见 [DesignApplicationIdentity.md](DesignApplicationIdentity.md)。

> 状态:**已实现**(`feature/style-layout`)。`ui.Run("com.example.Editor", func(app ui.App) RootView)` + `App` 句柄接口(Post/Sync/Quit/RequestUpdate/Clipboard/Settings)+ `ui.Root().StyleSheet(...)` 声明式换肤(reconcile 到 `gui.App`)+ DevPort 走 `import _ gui/dev` + `GOUI_DEV_ADDR`;`var ui.App` / `SetDevPort` 已移除。落地细节见 §8。
> 定位:记 ui 层"如何启动、如何拿运行期能力、配置放哪"的设计。总哲学见 `doc/Design.md`;View/reconciler 模型见 `doc/DesignUI.md`;样式见 `doc/DesignStyle.md`;跨层约定见 `doc/DesignConventions.md`。

---

## 1. 问题:配置期 vs 运行期的生命周期分裂

`gui.Application` 是在 `ui.Run` **内部**才 `NewApplication` 出来的,所以从进程启动到 `Run` 之间有一段"配置窗口期":`ui.App` 这个值存在,但底层 `gui.App` 还不存在。凡是要对着活的 `gui.App` 说话的能力(`SetStyleSheet` 实时换肤、`Post`/`Sync` 投递),在这段窗口期里都没有对话对象——这就是当前 API 的二义来源(`Post`/`Sync` 在 `Run` 前静默 no-op,`SetStyleSheet` 无处生效)。

`SetDevPort` 能蒙混过关,只因它是**一次性启动参数**:只在 `Run` 里被读一次,永不需要运行期生效。把运行期可变的东西(样式)硬塞进同一个"Run 前 setter"模子,才别扭。

## 2. 三类"设置",各归一处(核心纪律)

所有 app 级的"设置"先归类,再分派家。看着都"应用级"不代表同类——同 style/layout 分家一样,按**是否运行期可变 / 是否需实时生效**分:

| 类别 | 例子 | 特征 | 家 |
|---|---|---|---|
| 运行期命令 | `Post` `Sync` `Quit` `RequestUpdate` | 运行期调用,天生从 handler/goroutine 发起 | **`app` 句柄**(递进闭包) |
| 声明式状态 | `StyleSheet` | 运行期可变、要实时生效、可 reconcile | **build 返回的树**(`ui.Root().StyleSheet(...)`) |
| 调试脚手架 | DevPort | 设一次、发布版要可剥离 | **`import _ gui/dev`(链接)+ env(激活)** |
| 命令行 args | — | 平台/单实例才需要 | **不收**;要用时 platform 层自读 `os.Args` |

把这几类塞进同一种 setter,就是二义之源。

## 3. 入口:`ui.Run`,句柄递进闭包

```go
func ui.Run(appId string, build func(app ui.App) ui.RootView) error
```

- `ui.App` 从"全局单例 var"变成**句柄接口类型**。把运行期句柄**当参数递进闭包**,等于用类型系统把"配置期 vs 运行期"这条缝画出来:`app` 只在闭包里存在,而闭包只在 `Run` 之后被调用——`Post`/`Sync`/`Quit` 天生有活对象,§1 的二义**从签名上就不可能发生**。
- **`public var App` 退场**。当前唯一依赖它的是 `ui.State.Set → App.RequestUpdate()`;改走 app.go 里**现成的内部单例** `activeApp`(`currentApp().RequestUpdate()`)。于是 **public 面 = 递进闭包的句柄;internal 面 = 现成运行时单例**,泾渭分明。

### 3.1 句柄能力 + 持久化保证

`ui.App` 句柄挂**运行期命令式操作**:`Post` / `Sync` / `Quit` / `RequestUpdate`,以及只读透传 `Clipboard()` / `Settings()`(`Windows()` 之类后续同理挂这)。

**持久化保证**:`app` 是**整个 run 生命周期有效**的句柄,可随意 stash 进用户自己的 `type MyApp struct{ app ui.App }`、闭进 handler、丢给 goroutine/timer,**任何线程任何时刻调用都安全**(`Post`/`Sync` 本就是跨线程边界)。它不是"只在闭包动态作用域内有效"的临时令牌;`Run` 返回后调用则安全 no-op。

它与"整树重建"如何共存(免得担心 stash 会失效):

- `build` 每次更新整树重跑(`root.updateMounted → build()`),但**每次传进来的 `app` 是同一个**,stash 一次即可;
- 跨重建要存活的应用状态应由调用方持有普通 Go 值或 `ui.State`，不要在每次 build 中重新创建。`ui.State` 本身不是自动挂到节点的 hook；自定义 View 需要节点内状态时使用已有 `BuildContext.State/SetState`，协调身份遵循 [WidgetIdentity](DesignWidgetIdentity.md)。

惯用写法:

```go
type MyApp struct{ app ui.App }              // 轻量描述器,每次 build 重建也无所谓
func (m *MyApp) onSave() { m.app.Post(func(){ /* ... */ }) }  // stash 的 app,handler 里照用

func main() {
    ui.Run("com.example.Editor", func(app ui.App) ui.RootView {
        return (&MyApp{app: app}).view()     // 持久状态另行持有，不在此处重新创建
    })
}
```

### 3.2 框架先例

- **Flutter / SwiftUI**——样式/主题是**树里的数据**(`MaterialApp(theme:)` / `.environment`),运行期"实时"是 reconcile 白送的;根本没有"Run 前的命令式样式 setter"。→ 本设计 StyleSheet 走此路(§4)。
- **GTK / eframe**——app 对象/句柄在 run 时**递进回调**(`activate(app)` / `creator(cc)`),启动参数走 run 的入参。→ 本设计 `func(app ui.App)` 走此路。

### 3.3 `Run` 一次性(不可重复调用)

**App(平台 + 事件循环 + 排版)是进程级单例,`gui.NewApplication(appId)` 已 create-once;`Run` 也一次性:进程内只能跑一轮。**

- **为什么**:事件循环被 `Run` 消费后无法重启(`platform` 的 loop `quitting` 是永久闩锁);更根本地,Wayland/Android/Web 上**循环归 OS 所有,App 根本无法拥有一个可返回/可重启的循环**。这与现代主流(Flutter `runApp`、SwiftUI、winit"run 消费掉 event loop"、GTK4"application 用完即弃")一致,也守住了 goui 已承诺的可移植性(见 `DesignLayout §2`)。
- **换界面不靠重跑**:"复用 App、仅 UI 树不同"正是 reconcile 在**一次 Run 内**干的事——改 state → rebuild 换树(登录→主页别关窗重跑,而是换内容或 `SetQuitOnLastWindowClosed(false)`);模态靠声明式 `Popover`(`DesignPopup`),不靠嵌套/重启循环。
- **防御**:第二次 `Run`(并发或在第一次返回后)**返回 `ErrAppRunOnce`**,不 panic(`Run` 本就返回 error,惯用 `if err := ui.Run(appId, build); err != nil { log.Fatal(err) }` 会响亮暴露)、不静默失灵。实现是一个**一次性闩锁**(`activeRun`:`ran` set 一次、永不复位),同时盖住并发与顺序两种重入。

### 3.4 启动线程归属（2026-10-12）

公共 `platform` 包在现有 `platform.go` 的 `init()` 中调用一次
`runtime.LockOSThread()`，保留启动 OS 线程。`gui`、`ui` 通过已有依赖继承该保证；
不增加线程初始化包或三个后端的重复实现，移除 `ui.Run` 中过晚且重复的锁定。

- 用户从 `main` 创建 Platform／Application、窗口并运行事件循环；不能用
  `go ui.Run(...)` 或把应用创建、运行分配给不同 goroutine。
- macOS AppKit 要求真正的进程主线程；Windows 的窗口消息队列、OLE/COM STA
  和 Linux 的 GOUI 绘制／OpenGL 上下文要求稳定线程。三平台统一保留 main，
  是框架的启动契约，不声称 Windows/X11 原生 API 都要求进程启动线程。
- 此 `init()` 不创建应用或窗口、不初始化 OLE、不启动事件循环；但只要导入公共
  `platform`，即使尚未创建应用，main goroutine 也已被锁定，这是明确的导入副作用。
- 不在 Run 返回时自动解锁，保证原生资源的后续清理仍在同一线程。后台任务继续
  通过 EventLoop.Post／Application.Post／App.Post 或 App.Sync 投递，不自动搬移任意调用。
- 单独使用 `graphics`、`typography`、底层原生绑定或直接绕过公共 `platform`
  入口的后端调用，不由此保证线程归属；调用者自行满足对应原生契约。
- 本轮仅支持既有桌面目标；未来接入移动端时重新设计其系统主导的启动机制，
  不预先新增未使用的构建文件、开关或调度 API。

依据：[Go 1.25.8 LockOSThread 契约](https://pkg.go.dev/runtime@go1.25.8#LockOSThread)
明确保证 init 位于启动线程，init 中锁定可使 main 保留该线程；主 goroutine
并不天然等同于主 OS 线程。参考
[Gio v0.9.0 macOS 初始化](https://github.com/gioui/gio/blob/v0.9.0/app/os_macos.go#L309)
及 [Fyne v2.6.3 GLFW 入口](https://github.com/fyne-io/fyne/blob/v2.6.3/internal/driver/glfw/loop.go#L24)，
只采用线程保留，不引入它们的额外调度机制。

验证在既有 `platform/application_test.go` 中进行：记录包初始化时的线程 ID，
子进程 TestMain 在真正 main goroutine 上用一个 P 和锁线程 worker 制造调度交接，
断言 main 始终保留初始化线程。测试不创建窗口、不依赖桌面；旧实现已复现迁移。
桌面 demo 的概率性启动崩溃仍需区分线程回归、目标编译和真实 AppKit 窗口验收。

本轮验证（Go 1.25.8）：Linux amd64 的非缓存 `go test ./... -count=1`、
`go test -race ./platform ./gui ./ui` 通过；启动线程回归连续 30 次通过。
全仓 Windows amd64／macOS arm64 的 `CGO_ENABLED=0` 构建及 platform 测试目标
编译通过；上传后校验 SHA-256，在 Windows 10 19045 x64／macOS 15.7.7 arm64
各执行 20 次无窗口线程回归，全部通过。全仓测试保留既有窗口／图片输出副作用
及既有跳过项；远端线程回归不创建窗口，不代替真实窗口验收。

后续桌面验证：macOS 15.7.7 arm64、OpenGL、应用缩放 1x，实际启动裸 demo，
通过远端桌面切换页面并点击原生关闭按钮，进程退出码 0，未出现主线程异常。
Software 启动后本轮工作被用户中断；批量重复启动未执行，不计作独立通过。
用户随后反馈已完成验证并要求提交；不推定其验证次数或后端。

## 4. 声明式状态:StyleSheet 进树,不进签名

StyleSheet 是**产品状态**(运行期真会换主题),归声明式树,由 reconcile 落到 `gui.Application.SetStyleSheet`。运行期换肤 = 改 state → `RequestUpdate` → rebuild → reconciler 发现 sheet 变了 → 调一次 `gui.SetStyleSheet` → 全窗口 relayout,与窗口标题/内容走**同一条差异下发管线**,零特例。

承载它的是一个(可选的)顶层应用节点 `ui.Root()`——app 级声明的家:

```go
ui.Run("com.example.Editor", func(app ui.App) ui.RootView {
    return ui.Root().
        StyleSheet(currentTheme).            // 声明式,随 build 变;reconcile 到 gui.App
        Windows(
            ui.Window("main").Title("…").Content(content),
        )
})
```

- **初始设置**:第一次 build 就带上。**运行期切换**:改 `currentTheme` 触发 rebuild 即可。二义从根上消失——ui 层压根没有"Run 前的命令式样式 setter"。
- `gui.Application.SetStyleSheet` 这个**命令式**接口保持不动(命令式内核该有,且调用时 `gui.App` 一定存在);ui 只是 reconcile 到它上面。
- `ui.Root()` 是 app 级声明的落点:今天承载 StyleSheet,将来 app 菜单/托盘/`QuitOnLastWindowClosed` 等 app 级声明也有地方落,不必每样都往句柄上挂命令式 setter。不需要 app 级配置时,仍可直接返回 `WindowView`(`ui.Root()` 可选)。
- `ui.Root()` 是已实现的公开入口，与内部 reconciler 的未导出 `root` 无用户可见冲突。

> **不进树的判据**:能进树的是"运行期可变的产品状态"。DevPort 不是(设一次、脚手架),所以它不进树也不进签名(§5)。这条判据同 style/layout 分家纪律。

## 5. DevPort:`import _` 链接 + env 激活,双正交旋钮

dev server 是 **gui 层**的内省/控制面(读 `gui.App` 做 Snapshot、派发事件),自然属 gui 层,不该从 `ui.Run` 里拉。让它**自足**:

```go
// package gui/dev —— 自足,无需谁来调
func init() {
    if addr := os.Getenv("GOUI_DEV_ADDR"); addr != "" {
        go ListenAndServe(addr)              // 默认绑 127.0.0.1
    }
}
// handler 直接读全局 gui.App,判 nil 吃掉时序:
func handleSnapshot(w http.ResponseWriter, r *http.Request) {
    app := gui.App
    if app == nil {                          // 还没 Run / 没建起来
        http.Error(w, "app not ready", http.StatusServiceUnavailable)
        return
    }
    app.Post(func() { /* 在 UI 线程上 Snapshot / 派发 */ })
}
```

App 侧一行显式接入,可 grep、进版本管理:

```go
import _ "github.com/golang-gui/goui/gui/dev"
```

**两个正交旋钮,各管一件:**

| 旋钮 | 管什么 |
|---|---|
| `import _ ".../gui/dev"`(源码) | 代码**在不在**二进制里(体积)——不导 → `net/http` 不链接,发布版自动瘦 |
| `GOUI_DEV_ADDR`(env) | 服务**跑不跑**(激活)——不设 → 连 goroutine 都不起,零成本休眠 |

要点与被否掉的替代:

- **nil 判断替代生命周期编排**:`init` 可能在 `gui.App` 建起来前就起服务,handler 判 nil 挡住"未就绪"窗口,`gui.App` 一被 `NewApplication` 置上,handler 自动工作。省掉一切 hook / 注册缝。
- **层次正**:读 `gui.App`,对**任何** goui app(走 ui 层或直接 gui 层)都 work;`gui` 不 import `gui/dev`、`gui/dev` import `gui` → 无环;唯一 importer 是 app 那行 blank import。
- **不用 env-only**:env-only 对**桌面** app 不构成额外攻击面(能设你进程 env、连你 localhost 者已是你),但纯 env 无"链接开关",发布版剥不掉 `net/http`。故加 import 这一维。
- **不用 build tag**:tag 是构建旗标(`-tags`),IDE/CI 易漏传;import 是**源码结构**,版本化、可 review,CI 编发布入口(不含那行)自动瘦身,无旗标要记。极性是 opt-in(默认不导=不带不跑),换来显式;发布瘦身只需发布入口别写那行 import(共用 main 则把它放进 dev-only 的 build 单元)。
- **不用注册 seam**:`RegisterDevTools` 之类是多余间接层,`gui.App` 全局 + nil 判断已够。

## 6. args:现在不收

Qt/GTK 收 `argc,argv` 是 C 世界观。Go 里:

1. `os.Args` 本是全局 slice,不需一层层穿。
2. 真正会消费平台 flag(X11 `--display`、Cocoa `-NS*`)的是 **platform 层**，需要时在该层自读 `os.Args`，不应让参数数组穿三层。当前创建入口是 `platform.NewPlatform(name, appId)`；appId 是应用身份，不是命令行参数。
3. 命令行 flag 所有权归 app(`flag.Parse()`);goui 若替它解析/吞掉反而打架。goui 自己的旋钮走 env(同 DevPort),不碰 argv。
4. 加个没消费者的 `args []string` = 投机性表面,违背"别占表面"。

**何时回来**:做①单实例激活(open-file 转发主实例)或②platform 后端要剥离原生 flag 时,它是有明确设计的独立特性(可能 `Run` **返回**剩余 args),非现在拍参数占位。

## 7. 收敛后的落点表

| 东西 | 家 |
|---|---|
| 运行期命令 `Post/Sync/Quit/RequestUpdate` | `app` 句柄(递进闭包,可 stash,whole-run 生命周期) |
| 声明式状态 StyleSheet(及未来 app 菜单等) | build 返回的树(`ui.Root()`) |
| 调试端口 DevPort | `import _ gui/dev`(链接)+ `GOUI_DEV_ADDR`(激活),默认绑 127.0.0.1 |
| 命令行 args | 不收;要用时 platform 层自读 `os.Args` |

`ui.Run("com.example.Editor", func(app ui.App) ui.RootView) error` 签名纯净,四类各归一处。

## 8. 实现影响面(已落地,`feature/style-layout`)

- `ui/app.go`:**句柄就是运行时**——接口 `App` 由具体类型 `app`(即原 `appRuntime`,与 gui 层 `Application`/`application` 同构:`ui.App` 接口、`ui.app` 具体)实现,`Post`/`Sync`/`Quit`/`RequestUpdate` 直接操作 receiver(无 `appHandle` 无状态壳,也无包级转发 + 全局查活)。`Run` 里 `rt` 建好后直接递进 build 闭包(`build(rt)`)。`SetDevPort`/`devAddr`/`devServer`/`stopDevServer` 全删。
- `ui/clipboard.go`/`ui/settings.go`:gui 提供**自有接口** + 未导出具体类型，ui 包装持有 `current *app` 与 GUI 服务，方法通过 `Sync` 转发。Settings 使用 GUI 默认值；Clipboard 保留写入与读取错误，无文本/空文本均为 ("", nil)，应用已停止时 SetText 返回 ErrAppStopped；RequestText 返回未受理错误且不调用回调，受理成功后由回调交付读取结果。错误反馈按 `DesignConventions.md` 第 2 节处理。`ui/state.go`:`State.Set` 用 `currentApp()`（视图树里没句柄，是该单例的内部用途；public=句柄、internal=单例）。
- 外部打开：`App.OpenURL(string) error` / `App.OpenPath(string) error` 是运行时命令，不参与 Build 协调；Sync 返回该次操作结果，退出或未执行返回 `ui.ErrAppStopped`，局部失败不进入 `app.fail`，详见 `DesignExternalOpen.md`。
- **Run 一次性**(§3.3):`activeRun` 一次性闩锁(`activeApp.ran` set 一次、永不复位)取代原 `if currentApp()!=nil` 早检查,盖住并发 + 顺序两种重入,第二次 Run 返回 `ErrAppRunOnce`(原 `ErrAppRunning` 删除)。`activeApp.current`(供 `State.Set` 找 runtime)与 `ran` 闩锁分离:`setActiveApp`/`clearActiveApp` 不碰闩锁,故测试可反复装卸 runtime。
- StyleSheet:新增 `ui.RootNode` + `Root()`(链式 `StyleSheet`/`Windows`,值语义)。`resolveRoot` 从根拆出 `([]WindowView, style.StyleSheet)`;reconciler 的 `applyStyleSheet` 用 `reflect.DeepEqual` 判 sheet 变化(sheet 是含 slice 的**值类型**,`==` 会 panic)→ 变了才 `rt.gui.SetStyleSheet`(nil→回退默认),免每次 rebuild 全窗口 relayout。
- `gui/dev`:**定位重排为自足 opt-in**。`init()` 读 `GOUI_DEV_ADDR` 自启，按请求读取应用并回到 GUI 线程。构造与服务生命周期不公开；协议端点为 `SnapshotPath`、`EventPath`、`TextPath`。dev 自己维护短期 ref，通过 Window／Popover 的公开输入入口派发事件或提交已确认文本；详见 [DevAutomation](DesignDevAutomation.md)。`ui` 不 import `gui/dev`，默认监听 `127.0.0.1`。
- 示例 `example/ui_test`:`App.Run(func()…)` → `Run(func(app App)…)`,句柄内 `app.Clipboard()/Settings()`。
- 测试:`TestAppAppliesRootStyleSheetOnChange`(sheet reconcile + 相同不重发);`TestAppHandleRunsThroughRuntime`(`*app` 作 `App` 句柄直接跑 Post/Sync/RequestUpdate);`TestBeginRunIsOneShot`(第二次 `activeRun` 返回 `ErrAppRunOnce`)。
- **验证**:`go test ./...` 全过;`ui`/`gui`/`gui/dev` 在 linux+windows 编译通过;gofmt 干净。(darwin 交叉编译受 `platform/darwin` 绑定 WIP 阻塞,与本改动无关。)
