# 多语言文本与集中资源

## 契约与范围

`mlang` 是与 `icon` 同级的独立包，仅依赖 `core/signal` 和 `x/text/language`，
不依赖 platform/gui/ui。应用自己的 resource 包集中定义文本 Getter 与已解析图标；
不添加通用资源基类、全局资源注册表或 Widget 文本绑定接口。

```go
var Save = mlang.String(mlang.En("Save"), mlang.ZhCN("保存"))
// 命令层：saveLabel.SetText(resource.Save())
// 声明层：ui.Button(resource.Save())
```

String 的第一项是该词条默认翻译。Text(tag, text) 支持任意 BCP 47 标签，
En/ZhCN/Ja/Ko/De/Fr/Ru/Ar 是八语言便利入口，不引入封闭语言枚举。重复规范化标签和非法资源标签是构造
错误；外部语言偏好通过 SetLanguage(string) error 验证。当前语言初始为 en，
Language 返回请求的规范化标签，不冒充每个词条实际匹配的语言。

每个 Getter 持有不可变条目、x/text Matcher 和最近一次语言/结果的有界缓存；
没有词条永久登记或整数 ID。匹配保留地区与脚本标签，由标准 matcher 选择资源
支持的语言；无匹配或 und 使用首项。明确的空翻译不触发回退。

首批框架词条：简中 zh-CN、英语 en、日语 ja、韩语 ko、德语 de、法语 fr、
俄语 ru、阿拉伯语 ar。其他语言可由应用通过 Text 定义，暂不附带框架翻译。
本包不加入系统语言探测、语言持久化、通用复数/日期/数字格式器、翻译工具链，
不承诺 RTL 布局镜像。参数化资源可以由应用的具名 Go 函数包装完整格式句子。

后续切片已通过平台 Settings 接入系统界面语言，GUI 提供失败默认值；不改变
mlang 的独立性、初始语言或 UI 的构建规则。应用主动选择何时应用系统语言，
`example/demo` 增加八语言菜单与一次性“使用系统语言”，见
[DesignSystemLanguage](DesignSystemLanguage.md)。

## Demo 样例与用户数据的边界（2026-10-10）

只修改 `example/demo`，不增加框架接口或通用资源注册机制。默认编辑样例、
拖放邀请/接收说明、计时次数、树项目和列表/表格展示文本统一放入 resource，
覆盖既有八种语言；文件名、稳定 ID、`demo-card` 数据格式及实际输入/路径不翻译。

- 单行样例用私有 `sampleInput` 实现既有 `ui.BindState[string]`，内部仍复用
  ui.State 的更新调度。BindText 写回或应用主动 Set 都标记用户持有；语言更新
  只替换尚未编辑的默认值。布局中的用户名是普通持久 State，不需翻译品牌名称。
- 多行编辑器、标签文档和初始快照提示用 TextModel 单调 Revision 记录上次
  应用写入的样例。只有当前 Revision 仍相同时才能更新默认文本；更新后记录
  新 Revision。用户编辑、撤销、重做或读取真实快照后不再自动替换；即使撤销
  回默认原文，也不靠字符串相等猜测归属。文档关闭时删除对应 revision 记录。
- 同一个 Model 原地更新，保持对象、已编辑文本的选区/历史、排序、列宽、展开和选择状态；不会
  因语言改变重新创建模型或再次排序。表格备注单独标记是否编辑，显式“重置备注”
  返回当前语言的默认样例并恢复跟随。批量行更新复用 SliceListModel.Modify。
- 拖放记录格式与原始数据，展示时才格式化接收说明；空文本与未接收明确区分。
  应用内卡片携带稳定值，不把翻译后的 UI 字符串作为业务标识。
- 继续由应用一个 ConnectChanged 更新模型，UI 依既有语言信号请求重建；
  不逐控件订阅、不扩 Root/BuildContext、不重译历史操作日志或外部错误。

## 更新、线程与生命周期

语言状态为进程级，与现有单 Application 模型一致。读取/构造线程安全。
SetLanguage 同步提交状态，在释放锁后发 ConnectChanged；规范化同值不发通知。
接入 GUI 后必须在 GUI 线程切换，后台使用 App.Post，不能在 Build 中切换。

命令层用 translateUI 调用既有 SetText 等 setter，主动执行初次翻译，并连接
ConnectChanged；窗口释放时 Disconnect。只更新展示文案，不替换用户编辑的文本、
选择、焦点或业务模型。资源对象不是状态监听器。

ui.Run 在首次 Build 前连接一次变化信号至 RequestUpdate，沿用已有合并调度。
退出、Quit 和首次构建失败均清理订阅；不逐控件订阅，不给 Root/BuildContext
增加控件特例。后续 Build 重新取值，既有协调保持 Widget 身份与编辑状态。

框架自带文本放 internal/resource（仅依赖 mlang，避免 icon/svg -> gui 的循环）。
编辑菜单打开时取值，已经展开时通过 SetLabel 更新原有项；使用过菜单的编辑器
在挂载期间持有一个语言连接，卸载断开，重挂载恢复。窗口控制和标签辅助图标的
Snapshot.Text 按当前语言取值，角色、ID、动作标识及输入快捷键保持稳定。
诊断错误、协议字段、样式名、用户内容和原生系统自带文案不翻译。

## 参考

- Qt 6：LanguageChange 后重新设置展示文本，而非把翻译器注入所有控件：
  https://doc.qt.io/qt-6/qcoreapplication.html#installTranslator
- Flutter gen_l10n：具名资源 getter 与参数化方法；不采用其隐式上下文依赖：
  https://docs.flutter.dev/ui/internationalization
- BCP 47 解析、脚本/地区匹配采用现有 x/text 依赖：
  https://pkg.go.dev/golang.org/x/text/language

## 验证计划与记录

- 包内：语言解析/同值/失败不提交、回退与空翻译、地区/脚本、重复定义、
  缓存切换、并发读取、信号重入/断开、参数化资源。
- 集成：UI 自动合并重建、原 Widget/用户文本不变、退出失败路径清理；
  八语言词条完整性、实时编辑菜单与复制命令、控制和标签语义文本。
- 独立中文说明窗口程序：命令层和声明层切换、CJK/俄文/阿拉伯文显示、
  编辑选区和焦点保留、亮暗图标、长文本布局，走正常输入路径。
- 未进行母语人工校对；未执行的远端桌面与 RTL 镜像不能记为通过。

### 2026-10-09 首版执行记录

实现独立 mlang 与 14 条框架资源的八语言翻译；x/text v0.22.0 从间接依赖改为
直接依赖，没有升级版本。MenuItem 补充通用 SetLabel，复用模型变化通知，不在
菜单模型中加入语言策略。生产修改限于 mlang、internal/resource、gui、ui、widgets，
没有修改原生平台或绘制后端，也没有修改用户数据/协议标识/诊断错误。

自动验证（Go 1.25.8，Linux amd64）：

- 八语言辅助方法补齐后，新增 helper/通用 Text 的标签一致性与重复定义断言；
  mlang/resource/gui/ui/widgets 非缓存测试、mlang/resource race 和相关 vet 复验通过。

- `DISPLAY=:89 go test ./... -count=1 -json`：2129 个测试/子测试通过，14 个
  既有手动/显式启用测试跳过；独立窗口命令包只参与编译。
- `DISPLAY=:89 go test -race ./mlang ./internal/resource ./gui ./ui ./widgets -count=1`
  通过。包括扩展标签同值、回退/空翻译、八语言完整性、展开菜单复制、挂载/卸载
  连接、UI 自动合并重建、原 Widget/输入内容/选区/焦点保留及失败/退出路径。
- `go vet ./mlang ./internal/resource ./tests/window/ui/mlang/...` 通过。
  完整 `go vet ./gui ./ui ./widgets` 未通过：既有 TextSelection/TextRange/
  geometry 未命名字段字面量触发 composites 警告；HEAD 及本轮未改动的测试已
  存在这些写法，未顺带修改。`go vet -composites=false ./gui ./ui ./widgets` 通过。
- `GOOS=windows GOARCH=amd64 go build ./...`、
  `GOOS=darwin GOARCH=arm64 go build ./...` 通过；两目标的 mlang/resource/gui/ui/
  widgets 测试二进制亦经 `go test -c -o <仓库外目录>` 编译，不冒充原生执行。
- 最初启动临时显示服务的方式未保持进程，第一次全仓执行中既有
  TestTimerNativeRuntime 因 `create platform: can not open display` 失败；改用
  持续会话运行 Xvnc 后，原命令无修改测试重跑通过，不把该次失败算作通过。

本机窗口：Ubuntu 22.04.5、TigerVNC 1.12/X11、Xfwm4 4.16.1；独立临时桌面
`:89`，未在用户桌面 `:3` 操作。字体由 Pango/Fontconfig 选择，环境提供 Noto
CJK 与阿拉伯字形。图形环境 GLX/Mesa 23.2.1 llvmpipe，非硬件加速。

- `GOUI_PLAT_PAINTER=software/opengl GOUI_PLAT_SCALE=1/2` 下运行
  `tests/window/ui/mlang -auto`，四组均八语言断言和正常关闭通过；检查同一
  Widget/SVG Source、setter 准备的编辑内容/选区及 1x/2x 离屏非空像素。
  OpenGL 配置沿用平台可回退 Software 的契约，未增加生产接口探测后端，
  不把配置值或 GLX 环境信息当作每帧实际后端的强制断言。
- OpenGL 配置 1x：真实鼠标逐语言切换、Check 断言、展开菜单 Cycle 八语言
  实时翻译/尺寸变化、CJK/俄文/阿拉伯字形和亮暗 SVG 检查通过；输入与选区保留。
  临时桌面原 XMODIFIERS 下键盘打字未产生文本，使用隔离剪贴板粘贴准备文本，
  没有据此宣称该环境的原生 IME 已验收。
- Software 2x、`XMODIFIERS=@im=none`：实际打字 `second-scale text`、Ctrl+A、
  八个语言按钮逐次切换，Check 输出内容及 `{Anchor:0 Caret:17}` 始终不变；
  同语言点击 Build 计数不变。阿拉伯菜单中实际点击“نسخ”，由 xclip 读取
  临时桌面剪贴板，得到准确原文；点击 Close 输出 LANGUAGE EXIT PASS。
- 日志/截图/构建产物留在 `/tmp/goui-mlang.prLppZ`，不向源码目录新增产物。
  既有全仓测试会创建原生窗口及产生图片，本轮在独立桌面执行，没有改写其约定。

未执行：Windows/macOS 原生桌面、原生混合 DPI、中文等真实 IME、完整窄窗/
屏幕退避矩阵和母语人工校对。RTL 整窗镜像、格式器不在首版范围；系统语言
后续切片的独立验证见 DesignSystemLanguage。
