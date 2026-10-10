# Dev 自动化：引用、Popover 与文本提交

状态：2026-10-10，实现及本机、远端功能验收完成；Windows OpenGL 的远端启动限制见验证记录。编号、查找和协议状态属于 `gui/dev`，不进入 GUI 内核。

## 1. 边界

- 保留现有 opt-in DevServer，继续使用 Snapshot 与真实输入路径；不新增 Inspector、截图、语义动作或平台注入机制。
- Application 提供 `Windows()` / `Post()`；移除按程序 ID 寻址的 `DispatchWindowEvent`。Application 的普通 Snapshot 保持不变。
- Window 增加 `Popovers() []Popover`、`CommitText(string) error`；Popover 公开已有的 `DispatchEvent`，并增加 `CommitText(string) error`。
- GUI 不保存自动化编号，不认识 HTTP 或 dev，不给用户的 Window / Widget 自动设置 ID。
- dev 枚举公开对象、包装 detached Snapshot、自行管理短期引用。UI 层无需修改。

参考于 2026-10-10 核对 Qt 6.12 官方契约：
[QAccessible](https://doc.qt.io/qt-6/qaccessible.html) 区分运行期对象编号和测试 Identifier；
[QInputMethodEvent](https://doc.qt.io/qt-6/qinputmethodevent.html) 区分已确认文本和预编辑。
采用这两个语义区分，不引入 Qt 的无障碍缓存或完整输入法事件模型。

## 2. ref

所有返回的 Window、Popover、Widget 语义节点都有 dev 分配的字符串 `ref`，
来自同一个服务内部的自增序列。GUI 原始 `id` 保留为描述信息，无论其是否为空、重复或改名，
自动化输入只使用 `ref`。不调用 `SetID`，不修改 GUI 的 Info 类型。

- Window / Popover 引用对应真实宿主；同一存活、可达对象保持编号。
- 关闭、隐藏的 Popover 和销毁的 Window 失去输入引用；利用现有关闭/销毁信号及时清理，
  每次请求仍重新枚举，不能把缓存当作对象仍存活的证明。重新显示分配新引用。
- 不复用已分配编号。服务或应用重启后引用不承诺有效。
- Widget 引用属于本次发布的语义快照；下一次获取 Snapshot 后旧的节点引用失效。
  GUI Snapshot 会过滤结构节点，不能按 Children 下标与实际 Widget 树配对，
  也不能按角色、文字或坐标猜测永久身份。
- 本轮输入请求的 target 必须是 Window / Popover 引用。Widget 引用用于理解、定位和比较，
  根据 Bounds 在所属宿主发送坐标输入；不支持直接调用 Widget 动作。

所有引用表访问、枚举、Snapshot 和输入派发在 `Application.Post` 的 GUI 线程回调中完成。
HTTP goroutine 只接触解析的请求及 detached 响应数据。

## 3. GUI 公开能力

`Window.Popovers()` 返回当前成功显示、属于该窗口的 Popover 列表副本，按显示登记顺序。
对象是借用引用；隐藏、销毁、锚点卸载或换宿主时移除。未显示或 Show 失败不登记。
登记只报告实际生命周期，不改变已有锚点/窗口对原生资源的清理机制。

`Window.CommitText(text)` / `Popover.CommitText(text)`：

- 必须在 GUI 线程调用；将 UTF-8 已确认文本发送给本宿主当前逻辑焦点的 IMClient。
- 不改变焦点、激活窗口、调用 SetText、写剪贴板或伪造物理按键。
- 和原生提交共享 IMContext 的 Commit 路径，保留控件的归一化、选区替换、撤销和信号。
- 已销毁/隐藏宿主、无有效可编辑焦点、禁用/只读、无效 UTF-8，返回错误。
- Window 被模态目标拦截时返回错误，不编辑底层字段、不自动转发到另一宿主。
  可见 Popover 必须是当前模态输入目标，不能向没有键盘输入权的 modeless 内容提交。
- 已知的非空 preedit 存在时返回错误，不替用户取消或确认真实 IME 组合。
- 空字符串在有效目标上是无操作，不删除选区。成功表示已经分发，不承诺随后布局/绘制完成。
- 合成提交不伪造 Composed 来源信息。原生 commit 保留自己的 metadata。

Popover 的程序化文本提交不等同于原生输入法支持：当前原生 IME 委托尚未实现，
候选窗口、caret 的跨宿主原生坐标等继续按 DesignIME 的待办管理，不在本切片扩展。

## 4. 协议

沿用三个简单端点：

- `GET /goui/dev/snapshot`：`{ok:true,snapshot:{windows:[...]}}`。
  window 增加 ref 和 popovers，popover 报告 ref、modal 和 widget；所有语义节点增加 ref。
  原有角色、文字、Bounds、Attributes 等不丢弃。Popover 的 Bounds 继续使用其 Widget
  的宿主局部 DIP，不用请求的 Position 冒充系统实际位置。
- `POST /goui/dev/event`：`{target:"1",type:"pointer_down",x:80,y:24,button:"left"}`。
  target 为 Window / Popover ref；坐标始终是该宿主局部 DIP。原有 key/wheel 解析保留。
  Popover 键盘请求进入其公开 DispatchEvent，并核对实际模态归属；不转给 owner 的
  另一个模态 Popover，不让 modeless Popover 偷走按键。真实原生按键仍由 owner 转发
  到相同的 Popover DispatchEvent，dev 不重新实现按键分发。
- `POST /goui/dev/text`：`{target:"1",text:"你好，GOUI 👋"}`；调用目标宿主的 CommitText。
  不再支持 window 程序 ID 寻址。text 字段必须存在，允许显式空字符串。

对象不存在或引用失效返回明确错误，不退回首个窗口。网络请求超时/取消后尚未开始的
GUI 任务不再执行；已经开始的输入不能回滚，客户端不得自动重试非幂等输入。
本切片不承诺画面完成等待、截图、批处理或完整原生 IME 测试。

## 5. 验证

- gui 包内：Window / Popover 提交，Unicode、选区与 undo、只读/禁用/失焦/隐藏、
  modal 输入归属、preedit 拒绝、原生来源 metadata、回调销毁重入。
- gui 包内：Popover 成功显示登记、失败不登记、重复 Show、Hide/Destroy、锚点卸载、
  换窗口、回调期间释放，Popovers 列表为副本。
- dev 包内：无 ID/重复 ID/改名仍正确引用；根引用稳定、重开与移除失效、节点 ref 唯一；
  Attributes 与过滤子树保留；Window / Popover 指针、滚轮、按键和文本路由；
  不可编辑错误、过期引用、超时队列不迟到派发、正常输入销毁宿主。
- 独立桌面例程：tests/window/gui/dev，中文步骤；仅用于 DevServer 自身验收，
  Unicode 输入/选区替换/撤销、内置菜单及自定义 modal Popover、关闭后旧引用拒绝。
- 执行 go test ./...，针对性 race，Windows/macOS 编译。真实桌面结果与编译分开记录。

## 6. 验证记录

2026-10-10，Linux amd64 / Go 1.25.8：

- `go test -count=1 ./...`：通过；独立窗口入口只编译，不由此命令运行。
  JSON 结果中 14 个既有可选平台测试因原有开关/平台限制跳过（手动输入、剪贴板、
  GLX、透明、原生装饰及状态交互），未新增或扩大 skip；无测试失败。
  部分既有包内平台测试仍会短暂显示窗口，本轮未迁移它们，不将其启动等同于交互验收。
- `go test -race -count=1 ./gui ./gui/dev ./ui`：通过。
- Windows amd64 / macOS arm64，`CGO_ENABLED=0 go test -c` 编译 gui、gui/dev、ui：通过；
  同目标的 `go build ./tests/window/gui/dev`：通过。原生运行结果单独记录如下。
- 本机隔离 X11 桌面（TigerVNC 1.12、Xfwm4、1024×768、96 DPI），
  `tests/window/gui/dev -verify`：Software 1x/2x、OpenGL 1x/2x 四组均通过。
  入口实际记录后端图片类型和 PixelScale，断言 Unicode、选区替换/撤销、空文本、只读、多行、
  模态面板按键/提交、旧引用失效、重新显示及内置菜单点击。
  OpenGL 使用该隔离桌面的软件 GL，未作 GPU 性能或视觉质量结论。
- 远端 macOS 15.7.7 arm64，Software 1x/2x、OpenGL 1x/2x：四组 `-verify` 均通过。
  VNC 真实桌面另行验证原生鼠标聚焦、Command+A、协议 Unicode 提交、Command+Z、
  modal Popover 提交与 owner 拒绝、Escape 关闭后旧引用失效、内置菜单点击，均通过。
- 远端 Windows 10 22H2（19045）amd64，RDP 交互桌面 Session 1：
  Direct2D 1x/2x、Software 1x/2x 四组 `-verify` 均通过。
  Direct2D 1x 另行验证原生鼠标聚焦、Control+A、协议 Unicode 提交、Control+Z、
  modal Popover 提交与 owner 拒绝、Escape 关闭后旧引用失效、内置菜单点击，均通过。
  OpenGL 1x/2x 在当前远端环境创建 WGL 上下文前失败，报错
  `can not choose useful pixel format`，程序退出码为 1；未完成该后端功能验收，
  不能将其他后端通过或交叉编译通过视作其通过，也未据此推断非 RDP 桌面的表现。
- 原生 IME 组合输入人工验收未执行；包内测试覆盖已知 preedit 拒绝与原生来源 metadata。
- 验证初次使用 8888 端口时被已有服务占用，随后改为 49381；未停止原有服务。
  使用时应选择空闲端口，保持 GOUI_DEV_ADDR 与 -addr 一致。
- 远端传输的可执行文件均核对 SHA-256；验证创建的窗口、进程、临时计划任务和桌面连接已清理，
  未关闭用户既有服务或会话。真实桌面验证不等于截图协议、系统无障碍或原生 Popover IME 验收。

复现协议自身的桌面断言：

```sh
GOUI_DEV_ADDR=:49381 GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 \
  go run ./tests/window/gui/dev -verify -addr http://127.0.0.1:49381
```

截图、系统无障碍桥接、原生 Popover IME 和跨进程永久引用不在本切片内。
