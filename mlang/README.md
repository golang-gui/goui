# mlang

轻量的多语言文本资源，不依赖 GUI，不读取系统语言或资源文件。
应用可将文本和已解析的 SVG 一起放在自己的 `resource` 包中：

```go
var Save = mlang.String(
    mlang.En("Save"), // 第一项也是无匹配时的默认翻译
    mlang.ZhCN("保存"),
    mlang.Ja("保存"),
    mlang.De("Speichern"),
)
```

`Save` 是 `func() string` 形式的 Getter，调用时按当前语言取值。
资源定义一次，不在每次 UI 构建中重复创建。语言匹配采用 BCP 47 标签，
例如 `de-DE` 可匹配 `de`。明确的空翻译保持为空。

```go
if err := mlang.SetLanguage("zh-CN"); err != nil {
    // 非法语言标签：保持原语言，不发通知。
}
fmt.Println(resource.Save()) // 保存
```

初始语言为 `en`。`Language()` 返回请求的规范化标签，而非某个词条的回退语言。
系统界面语言通过 `gui.Settings.Language()` / `ui.App.Settings().Language()` 查询，
应用在自己的初始化流程或用户操作中主动决定是否传给 `SetLanguage`。系统查询
和应用语言独立，不要在 UI Build 中调用 `SetLanguage`。
合法但无翻译的语言使用各词条第一项，不报加载错误。资源定义中的非法或重复
标签则是程序错误，构造时 panic。

命令式 GUI 采用显式翻译函数：

```go
translateUI := func() { saveButton.SetText(resource.Save()) }
translateUI()
handle := mlang.ConnectChanged(translateUI)
// 所有者销毁时 handle.Disconnect()。
```

声明式 UI 在 Build 内直接取值，`ui.Run` 自动监听语言变化并请求协调更新：

```go
ui.Button(resource.Save()).OnClick(save)
```

语言通知同步发生在 `SetLanguage` 的调用线程。接入 GUI 后须在 GUI 线程切换，
后台任务使用 `App.Post`；不能在 Build 内改变语言。包本身允许并发构造及读取。

带参数的文案由应用定义普通 Go 函数，使用完整句子及 `fmt.Sprintf`，不要拼接
需要翻译的碎片。本包暂不提供复数、日期、数字格式器或 RTL 布局镜像。

八语言辅助方法为 `En`、`ZhCN`、`Ja`、`Ko`、`De`、`Fr`、`Ru`、`Ar`。
框架内置文案首批提供：简中、英、日、韩、德、法、俄、阿拉伯八种语言。
应用不受此列表限制，可以通过 `Text` 增加任意 BCP 47 语言资源。
完整窗口验收程序见 `tests/window/ui/mlang`，运行：

```bash
go run ./tests/window/ui/mlang
```
