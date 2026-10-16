# GOUI Settings 设计

> 状态：已实现外观、字体与首选 UI 语言的三层暴露；其他设置按实际需求评估，不是当前接口承诺。
> 适用范围：`platform/`(只读事实)、`gui/`、`ui/`
> 相关文档：`doc/DesignPlatformSettings.md`(平台边界)、`doc/PlatformDev.md`

---

## 1. 三层职责

| 层 | 职责 | 可变? | 形态 |
|---|---|---|---|
| **platform** | 严格返回**系统值**;拿不到返回 `error`(`ErrSettingUnsupported`)。**不发明默认值**(那是策略) | 只读 | `(value, error)` |
| **gui** | 把系统事实变成**可直接用的具体值**:系统值,拿不到用 **gui 自有默认常量**兜底。**抹平平台差异** | 只读 | 具体值(无 error/nil)+ 单一变化信号 |
| **ui** | **线程安全**地读取(自动上 UI 线程),值同 gui；内部监听变化、合并声明式更新 | 只读 | 具体值 |

**核心原则**:GUI 层的天职是**抹平平台差异、给自然够用的对外 API**。若把"这项系统有没有、有没有效"原样上抛,等于又暴露了平台差异,每个上层消费者都得重复判断——所以 fallback **必须落在 gui 层**(平台层不能擅自发明默认;这正是 `DesignPlatformSettings.md` 中 "the gui/style layer owns fallback policy" 的含义:兜底归 gui-或-以上,不归 platform)。

## 2. 为什么只读(不可 set)

Settings 是**通用系统设置**,不是"样式的东西"。让它可变(app 覆盖 + 偏好枚举 + 解析)会:①把**策略**塞进本应是"系统事实暴露"的层;②与正在独立开发的**样式系统**职责重叠、出现两个外观覆盖机制;③产生"到底设没设到系统"的歧义。**可变外观/主题是样式系统的话题**,与 Settings 的暴露正交。故 Settings 三层皆**只读**。

## 3. 为什么删掉 `UnsupportedSettings`

它是 `common` 里一个"全项不可用"的空对象,但**全仓库零引用**,且三后端 `newSettings` 从不失败、从不返回它。它本质是"用空对象吞掉失败",与平台层"报告事实、失败返回 error"相悖。gui 层的 fallback 已经承担了"拿不到给默认值"的角色(即便 `platform.Settings` 为 nil,gui getter 也全程回退)。故删除。

## 4. API

```go
// platform
type Settings interface {
    Language() (string, error)                        // 首选 UI 语言，BCP 47
    ColorScheme() (ColorScheme, error)
    AccentColor() (color.Color, error)
    FontFamily() (string, error)
    FontSize()   (float32, error)
}

// gui：Settings 是 gui 自有接口 + settings 具体（包装 platform、隐藏 error、补 gui fallback）；App.Settings() Settings（永不为 nil）
type Settings interface {
    Language() string                                // 有效系统语言，否则 "en"
    ColorScheme() gui.ColorScheme                     // 系统值，否则 Light
    AccentColor() color.Color                         // 系统值，否则 defaultAccentColor(RGB 70,130,220)
    FontFamily() string                               // 系统值，否则 ""(渲染层平台默认字体)
    FontSize()   float32                              // 系统值，否则 14
    ConnectChanged(fn func()) signal.Handle           // 系统变化，UI 线程上触发
}

// ui：ui.App.Settings() ui.Settings（永不为 nil，线程安全）
func (Settings) Language() string
func (Settings) ColorScheme() ui.ColorScheme          // 内部 Sync 上 UI 线程
func (Settings) AccentColor() color.Color
func (Settings) FontFamily() string
func (Settings) FontSize()   float32
// 无 ConnectChanged / OnSettingsChanged —— ui.Run 内部自动监听并 RequestUpdate
```

**类型不外泄下层**：`ColorScheme` 在 gui、ui 各用**类型别名 re-export**（`gui.ColorScheme = platform.ColorScheme`、`ui.ColorScheme = gui.ColorScheme`），签名里只出现本层类型。而 `Clipboard`/`Settings` 要在本层**适配行为**(gui 对不可用服务与 Settings 有效默认值作适配、保留 Clipboard 的 error；ui 负责线程调度),故各层用**本层自有类型**——gui 出接口包装 platform、ui 出结构包装 gui,**不是别名**(只有零适配的纯值/枚举才用别名)。这样用 gui API 不必 import platform、用 ui API 不必 import gui。例外是**有意暴露**的低层逃生口 `App.Platform()` / `App.Typography()`。

fallback 默认值是 **gui 自有常量**(`gui/settings.go`),不依赖 Label,后续可调/由样式系统主导。

## 5. 线程与变化

- **快照初始化**：创建 GUI Settings 时就在 GUI 线程读取首份解析快照，getter 只读快照、不直接查询平台；无效值和查询失败逐字段使用 GUI fallback。颜色复制为自有 RGBA64 值，不保留平台可变对象。变化时先替换完整快照再通知，首次轮询即可与初始快照比较。
- **变化检测由 gui 在 UI 线程上驱动**。`Application.Run` 启动 `Settings.watch`，通过 `Application.NewTimer()` 每两秒在 GUI 线程读取、比较并通知；共用应用计时器调度器，不额外为 Settings 创建 ticker goroutine。Run 返回时停止计时器，过期回调不再查询或通知。创建应用但尚未 Run 时不轮询。
- **platform 层不含 `NewSettings(onChanged)` 或后台 watcher**。当前为同步查询；原生设置变化通知替代轮询属于 [待评估方向](Roadmap.md#future-settings-hidpi)，不能把轮询写成即时通知。
- **单一信号**(`Settings.ConnectChanged`,挂在 Settings 上而非外层 App):设置项增多时,一个 `func()` "变了,重读"即可,无需按项 diff;真出现具体的 per-setting 消费者再拆。
- ui 层 getter 走 `App.Sync`(在 UI 线程直跑、否则投递并等待),读取始终线程安全。
- ui 在首轮 build 前建立一条 Settings 变化连接，内部调用 RequestUpdate，与显式更新共用合并机制。初始化失败、更新失败及运行退出时断开；停止后的排队更新失效。用户在 build 中读取 Settings 即可，不必编写 OnSettingsChanged。固定主题参数仍可忽略系统值，相同样式表不重复下发到 GUI；普通应用状态仍须显式更新。

## 6. 验证

- 三 GOOS 交叉编译;`gui`/`ui` 单测;`platform/test` 的 `TestSettings` 仍覆盖平台层事实读取。
- Modern 路线第 4 步已通过 `go test ./...`、`go test -race ./gui ./ui -run TestSettings -count=1`，以及 Windows amd64／macOS arm64 的 GUI／UI 测试目标交叉编译；原生系统设置切换仍需真机验收，轮询不是即时通知。
