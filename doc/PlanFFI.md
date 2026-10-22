# FFI 地址稳定性检查与修复

核对日期：2026-10-20。基线为 GOUI 44fbaff、cgo efe9960、nanovgo 6d9eac4；
本机 Go 1.25.8、Linux amd64。检查范围是 GOUI 支持的 Linux、Windows x64、macOS
绑定，以及它们实际使用的 cgo 和 nanovgo GL 调用桥；不据此承诺所有第三方库的内存安全。

## 已确认原因

GOUI v0.2.0 的 XNextEvent 输出变量可留在 Go 栈上。转成 uintptr 后经过 LazySymbol
等 Go 包装调用时，栈搬迁不会更新整数地址，原生写入落到旧地址；返回值可能全零。
XFilterEvent 在读取事件中的空 Display 后访问 Display+0x968，形成 SIGSEGV。
已经用仅包含 Xlib 的最小程序复现同一指令、空指针和故障地址，不需要 Markdoc 控件参与。

初始受控复现对照：4096 次不同栈深度的读取中，旧 NextEvent 出现 125 次异常；
固定输出对象后为 0。该比例来自刻意布置的栈增长场景，不是应用实际故障率。

## 检查范围与结论

| 路径 | 检查结果及处理 |
| --- | --- |
| cgo.CallRaw、Symbol.CallRaw、LazySymbol.CallRaw | 每个 uintptr 入口补 uintptrescapes；内层入口不能补救外层已丢失的逃逸信息。覆盖符号解析、真实原生调用及 Go 回调期间的地址和存活性。 |
| cgo Windows x64 Call/CallRet | 整数参数数组不承担 GC 所有权；保留 typed args/ret，固定字符串、指针、间接结构参数和隐藏返回缓冲；私有接口数据字段保持 unsafe.Pointer 类型。 |
| cgo Unix typed bridge / macOS typed函数与objc.Send | 核对 purego v0.10.0 的反射参数所有权及 KeepAlive；保留指针类型，不添加无依据的 uintptr 转发。 |
| nanovgo GL call | 外层转发入口独立补声明，覆盖句柄输出、shader/log、uniform、上传缓冲和指针表。仅修 cgo 时 GL 原有外层仍可复现失效。buffer offset 保持数字语义。 |
| GOUI Linux动态绑定、Windows COM/SDK | 搜索当前 486 处 CallRaw/SyscallN 调用及 typed入口；直接转换由匹配的桥接契约保护，不批量复制 Pinner 到每个查询函数。 |
| 提前保存整数地址 | AST 与文本检索核对转换位置；Fontconfig 的可空文件名改为保留 unsafe.Pointer，直到受保护的调用参数才转换。其余候选为原生句柄、整数、native const 地址或永久 vtable。 |
| Windows消息参数、OLE | uintptr形式的消息参数继续要求调用方明确所有权；现有 frame计算已固定缓冲，OLE对象及表已有持久 pin，异步消息不能依赖临时调用保护。 |
| Cairo surface / CoreGraphics bitmap context | 原生对象在创建返回后继续引用像素；由 textPainter 持久固定，释放原生对象后 Unpin。Pinner 以指针保存，适应已有 next→painter 的资源转移，不复制已使用的 Pinner 值。 |
| XImage软件上传 | drawImage 固定像素直到 XImage 释放；仍在 XDestroyImage 前清空其借用 Data，保留既有分配器边界。 |
| 原生保留的回调和数据 | XIM复制描述符，回调是进程期trampoline；CoreText inline refCon 为 C 分配；OLE已由引用计数和pin管理。未把一次同步调用的保护当成异步保留期。 |

`runtime.KeepAlive` 不能替代地址固定，也不能追回先前已经整数化的地址。
调用后保留指针的绑定已补充调用方持有和固定的注释。三处旧排版验收用例缺少
layout.Destroy，新增持久 pin 后被运行时泄漏检查捕获；已补齐释放，保留全部原断言。

依据：Go 1.25.8 [uintptrescapes](https://github.com/golang/go/blob/go1.25.8/src/cmd/compile/doc.go)、
[unsafe.Pointer](https://pkg.go.dev/unsafe@go1.25.8#Pointer) 与
[runtime.Pinner](https://pkg.go.dev/runtime@go1.25.8#Pinner)；
[Cairo image surface](https://www.cairographics.org/manual/cairo-Image-Surfaces.html#cairo-image-surface-create-for-data)
要求数据保存到 surface 销毁；Apple 的
[bitmap context data](https://developer.apple.com/documentation/coregraphics/cgcontext/data?language=objc)
与[位图上下文示例](https://developer.apple.com/library/archive/documentation/GraphicsImaging/Conceptual/drawingwithquartz2d/dq_context/dq_context.html)
核对借用缓冲及释放顺序。没有引入新FFI框架或改变原生ABI。

## 验证

- cgo raw_pointer_test：真实 libc 写入、独立字节预期和 canary，覆盖包函数、Symbol、
  LazySymbol、Memset，在新goroutine的不同栈深度执行。旧四种路径均失败，修复后三次 race 复验通过。
- cgo pointer_reentry_test：在 t.TempDir 编译真实 C fixture；C持有缓冲期间进入Go回调，
  强制增长栈并GC，返回后再由C写缓冲；typed大结构参数/返回也验证明确数值。
  本机有C编译器，该用例实际执行通过。无编译器环境会明确跳过此fixture，不影响无编译器的raw用例。
- nanovgo gl/pointer_test：真实memset经过私有GL转发，检查输出及canary；
  在已修cgo但去掉GL入口声明的对照副本中仍失败，修复后普通和race均通过。
- tests/window/platform/ffi_events：仅Linux/X11，私有未映射窗口的编号ClientMessage经
  正常SendEvent进入原生队列，分别验证NextEvent与PeekEvent，以及XFilterEvent。
  已发布cgo v0.1.0在第3个事件返回全零；修复入口后各4096次（共8192次）均通过。
- GOUI全包普通/race测试在独立源码副本、独立Xvnc :176 / Xfwm4运行通过。
  旧测试会生成PNG及使用窗口，副作用限于该副本与独立桌面。
- cgo、nanovgo全包普通/race测试通过；Markdoc全包race测试通过。
  Markdoc、SourceView源码检索没有直接的FFI调用，本次无需改动其调用代码。
- Markdoc原生应用场景在Linux/Pango、软件渲染1倍缩放（race）与2倍缩放暗色
  （普通构建）通过；菜单、剪贴板及生命周期场景在软件渲染亮色、OpenGL暗色
  1倍缩放（均race）通过。2倍缩放软件渲染的race构建逐步推进到第12步后，
  达到原有20秒期限而失败；没有放宽期限或断言，也不将该次运行计为通过。
- Windows amd64、macOS arm64 的GOUI全目标构建与cgo测试二进制交叉编译通过。
  不等于Windows/macOS原生运行；这两个平台的原生FFI/回调和绘制仍待验收。
- GOUI go vet ./... 失败；与还原本次改动前的源码副本对照，222条诊断一致，新增0。
  包括既有native uintptr转换和未命名字段字面量；没有关闭检查或弱化断言。

初次无DISPLAY全包测试中的旧Typography用例无法打开display；工作区模式还使
CLI的临时子模块测试继承到不包含该子模块的go.work。随后在隔离副本中使用
临时本地依赖替换、GOWORK=off和独立X11显示重新执行全部测试并通过。
临时配置没有写入工作仓库，也没有修改全局Go环境。

提交前又从三仓暂存区分别导出源码快照，排除工作区原有未跟踪文件，在隔离副本中
用修复后的本地依赖执行 GOWORK=off 的全包普通/race 测试，三仓均通过。
GOUI 在独立 Xvnc :178、Linux/Pango、软件渲染1倍缩放下验证；
本轮 Xfwm4 因 Xfconf 初始化失败退出，实际没有窗口管理器，不计为 WM 场景验收。
NextEvent、PeekEvent 各4096次复验通过。快照初次运行cgo旧测试缺少
testdata/test.dll；随后在副本内从已跟踪的libtest.c编译该fixture及libtest.so，
并从libcallback.c编译libcallback.so，再执行全部测试通过，没有把生成库纳入提交。

## 依赖与交付条件

本地源码修复提交为 cgo `e70289e`、nanovgo `6f7deba`、GOUI `50bad94`；
这些提交号不代表相应模块版本已经发布。

修复分别位于 cgo、nanovgo、GOUI。联调通过仓库外 go.work 或验证副本的临时replace
选取三仓当前源码，不提交机器路径。GOUI go.mod 当前仍是可获取的 cgo v0.1.0、
nanovgo v0.0.8；它们尚不包含本批入口修正。

因此当前是源码修复和本地联调完成，不能声称现有发布依赖已完整修复。
独立交付顺序是发布含修复的cgo、发布含GL入口修复的nanovgo，再让GOUI升级依赖并
执行GOWORK=off验证。没有擅自发布依赖、构造不存在的版本或提交本机replace。

日志、受控对照副本与原生程序位于本次仓库外临时目录；正式交付说明记录最终复验结果。

## 2026-10-23：原生指针类型与静态诊断

本轮使用已发布 cgo v0.1.1，不修改调用桥或依赖版本。发布前的 195 条 vet 诊断中，
167 条是跨包结构体的未命名字段，28 条来自原生指针先保存为 uintptr 再转换。
这些告警不等于 28 个已经证实的运行时故障；原生分配并不会随 Go 栈移动，
但应在绑定中准确表达指针类型，不靠关闭检查或新建转换 helper 消除提示。

- 原生指针返回值经既有 CallRet 直接接收为 `*T` 或 unsafe.Pointer；真正的整数、
  X11 ID 和不透明句柄保留原类型。可选 Xcursor/Xrandr 调用保留符号可用性检查。
- Fontconfig/GLib 的字符串输出槽、X11 属性和 XrmValue 的地址使用指针，复制后
  按既有所有权释放；XIM nested list 同样保留指针到释放，不改变回调保留期。
- Pango 的 GlyphString glyphs/logClusters、Attribute.Class、Analysis.ExtraAttrs
  与本机 Pango 1.50.6 头文件的指针字段一致；后两者的 Go 字段类型也同步改为
  `*AttrClass` 和 `*glib.GSList[Attribute]`。LP64 原生偏移不变，补充 ABI 断言；
  不再保留本包重复的 uintptr 转指针 helper。
- GLib FilenameToURI 使用 typed call 固定参数和输出槽；保留符号查找失败和 GError
  的操作错误，不将缺失符号改为成功，也不以旧 errno 代替该 API 的错误契约。
- 字面量补全字段名只消除对其他包字段顺序的依赖，不改变值或测试断言。

所有权、线程、ABI 布局、异步借用和释放规则保持原契约。验证结果记录在 Plan
本轮发布修正记录，历史失败与旧依赖状态不改写成当时已通过。
