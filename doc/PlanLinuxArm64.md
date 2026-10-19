# Linux arm64 适配设计与实施计划

状态：设计与计划已确认，尚未开始代码适配和 QEMU 环境搭建。
基线：2026-10-15，`main` 的 `4167bb7`，本机 Go 1.25.8／Linux amd64。
本计划的主要验收环境为本机 QEMU 完整系统模拟的 Ubuntu arm64 桌面，
不以取得实体 ARM64 设备为实施前置条件，也不将模拟结果写成实体硬件验收。

## 1. 目标与边界

将 Linux arm64 作为现有 Linux 后端的架构适配，而不是新增一套平台实现。
复用 X11 窗口、事件循环、Pango／Cairo 排版、Software 和 OpenGL 绘图，
保持 `platform -> gui -> ui` 的实施顺序，不改变既有依赖方向和上层公共 API。

首期目标：

- `GOOS=linux GOARCH=arm64`，64 位小端 AArch64、LP64、glibc 用户态。
- 沿用项目 Go 版本要求和默认 ARM64 指令集基线，不为优化引入额外 CPU 特性要求。
- `CGO_ENABLED=0`；应用构建不需要 GCC、交叉 C 工具链或目标系统开发头文件。
- 运行时使用 ARM64 的系统动态库，不能混入宿主 amd64 库。
- 在确认的 Ubuntu arm64／X11 桌面上完成包内测试和窗口操作验收。
- Linux amd64 既有行为保持正确；共享代码改动按风险检查 Windows／macOS 编译。

不包含：原生 Wayland、EGL／OpenGL ES、32 位 ARM、musl／Alpine 专项适配、
移动端、新的显示器抽象、控件重构、性能优化或新的安装包格式。
其他发行版与实体 GPU 按后续环境补充验证，不以发行版名称添加猜测性兼容分支。

遵循[平台规范](../platform/AGENTS.md)、[Linux 规范](../platform/linux/AGENTS.md)、
[测试约定](Testing.md)、[绘制颜色契约](DesignGraphicsColor.md)和
[Typography 设计](DesignTypography.md)。全局进度见 [Plan](Plan.md)／[Roadmap](Roadmap.md)。

## 2. 已核实的现状

以下为适配前的检查，不是新功能验收。当前工作区含有既有未跟踪测试和文件；
后续记录必须注明使用工作区快照还是已跟踪源码快照，不混用两者的测试数量。

| 项目 | 2026-10-15 检查结果 | 含义 |
| --- | --- | --- |
| 全仓 Linux arm64 构建 | 失败，libc 引用未定义的 `SYS_PIPE`／`SYS_POLL` | 已确认首个阻塞，不表示没有其他问题 |
| Xlib／GLX／Cairo／Pango／Fontconfig／GLib／GTK 绑定 | arm64 交叉编译通过 | 不等于实际加载、调用和回调正确 |
| Pango 排版、Software／OpenGL 绘图包 | arm64 交叉编译通过 | 不等于窗口、像素或 GPU 验收通过 |
| `cmd/goui` | 已接受 `linux/arm64`，已有 AArch64 ELF 校验 | 不需要另建构建工具或目标类型 |
| purego v0.10.0 | 官方列 Linux arm64 为 Tier 1 | FFI 有支持基础，不替代 GOUI 绑定验证 |
| goexlib/cgo v0.1.0 | 已有 Linux arm64 调用／回调支持 | 先验证当前依赖，不预先升级或改写 FFI |
| QEMU Ubuntu arm64／实体 ARM64 桌面 | 本切片均未执行 | 实施后分别登记环境与结果 |

全仓失败复现命令：

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...
```

关键错误：

```text
platform/linux/libs/libc/syscall_linux.go:29:42: undefined: syscall.SYS_PIPE
platform/linux/libs/libc/syscall_linux.go:61:40: undefined: syscall.SYS_POLL
```

已通过的局部构建：

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
  ./platform/linux/libs/xlib ./platform/linux/libs/glx \
  ./platform/linux/libs/cairo ./platform/linux/libs/pango \
  ./platform/linux/libs/fontconfig ./platform/linux/libs/glib \
  ./platform/linux/libs/gtk ./platform/typography/pango \
  ./platform/graphics/software ./platform/graphics/opengl
```

现有 `platform/graphics/opengl/opengl_linux.go` 使用 GLX，要求 GLX 1.3 或更新，
并依赖 ARB context 创建能力。nanovgo v0.0.8 的桌面 shader 使用
`#version 330 core`。因此 CPU 架构支持与图形 API 支持必须分开确认：
只提供 OpenGL ES 的设备不能据此承诺 OpenGL 后端可用。

`platform/linux/x11.Platform.NewPainter` 在 OpenGL 创建失败时可以回落 Software。
窗口成功显示不能证明 OpenGL 已通过；验收必须确认实际 Painter／Context 类型。
Software 可用也不意味着可以移除当前平台初始化所需的核心动态库。

## 3. 适配设计

### 3.1 保持后端结构

- 继续使用 `platform/linux/x11` 和 `platform/linux/libs`，不增加 `linuxarm64` 后端。
- 普通 Linux 逻辑继续共享；只有原生契约确实存在架构差异且不能由现有依赖处理时，
  才增加最小构建约束，不在业务方法中散布 GOARCH 判断。
- 绑定保留原生 API 的类型、所有权和错误语义，适配留在 `x11`。
- 不为架构适配修改 Window、Widget、Painter、TextLayout 或声明式 View 的公共契约。
- nanovgo、goexlib/cgo 或 purego 仅在复现出对应依赖缺陷后单独修复；
  不先行 fork、升级、替换调用桥或添加 GPU 后端。

### 3.2 系统调用与唤醒管道

Linux arm64 不提供当前代码使用的两个旧系统调用。复用已经在模块图中的
`golang.org/x/sys/unix`，由其处理架构差异，不手写 syscall 编号或寄存器参数。
当前 v0.30.0 的 `Pipe` 基于 `Pipe2`，`Poll` 基于 `Ppoll`，已有可复用实现。

涉及 `platform/linux/libs/libc/syscall_linux.go` 与
`platform/linux/x11/event_linux.go`：

1. 在 libc 薄封装中提供 `Pipe2(fds *[2]int32, flags int) error`，
   调用 `unix.Pipe2`；原生 C fd 宽度仍为 int32，Go 包要求的 `[]int` 在边界转换。
   创建失败不得发布部分结果或关闭无关 fd。
2. 事件循环创建管道时显式传入 `NONBLOCK | CLOEXEC`，一次设置标志，
   不再依赖创建后的逐项 fcntl。管道无效值使用 -1，失败与销毁只关闭已拥有的 fd。
3. `PollFd` 直接复用 `unix.PollFd`，同步调用点的 `Revents` 字段名，
   不使用 unsafe slice 强转或每轮复制整份 fd 列表。
4. `Poll` 保持现有返回数量及 errno 语义，内部调用 `unix.Poll`；
   正确保留无限等待、立即返回、有限超时和 EINTR，重试策略仍在事件循环。
5. 阻塞等待必须通过会通知 Go 调度器的调用路径，不能换成长期阻塞的 RawSyscall。
6. 保留已有任务合并／唤醒、先处理原生事件再执行任务、Quit／Destroy 时序。
   非阻塞写遇 EAGAIN 表示管道已有唤醒数据，不忙等；读取排空后返回等待。
7. `Read`／`Write`／`Close` 等其他正确调用不因统一写法批量替换。
   fcntl 封装和常量仅在确认没有其他引用后清理。

`x/sys` 从间接依赖变为直接依赖即可；不为此切片顺带升级其版本。
这些改动同样作用于 amd64，必须在两种架构上验证，而不是仅检查 arm64 编译。

### 3.3 ABI 与 FFI

按实际使用路径检查，不将“64 位”当作 ABI 相同的证明：

- C int／long／size_t／指针的宽度、符号、字段偏移及对齐。
- XEvent、XClientMessage、XImage、XIM callback／preedit、Pango 属性及度量结构。
- Xlib format=32 属性与宿主 long 数组的区别，检查读取、写入及长度计算。
- Cairo／Pango／OpenGL 的浮点参数、返回值和按值结构体。
- XCreateIC／XSetICValues 等可变参数，包含超出寄存器数量的参数。
- Xlib／Pango 回调和 Go -> C -> Go 重入时，参数、返回值、线程和地址有效期。
- 动态库加载、回调注册、原生保留指针的释放顺序；不使用临时 Go 指针充当长期 C 数据。

有浮点或按值结构体的调用沿用 `cgo.Call`／`CallRet`；正确的整数／句柄调用
不无差别改写。内存是否需要逃逸、保活或 pin 按调用桥和原生保留行为判断，
`runtime.KeepAlive` 不等于 pin，也不保证异步生命周期。

布局断言以目标 ABI／原生头文件为独立依据，不仅比较两份 Go 实现。
真实调用、回调和生命周期在 ARM64 客体执行；macOS arm64 的已有结果不替代它们。
若复现依赖问题，先用最小调用分离出是绑定、FFI 还是 QEMU 的问题，再决定修复归属。

### 3.4 绘图能力边界

- Software：检查像素布局、预乘合成、图片步长、Transform、clip、文字和 1x／2x。
- OpenGL：检查实际 GLX／OpenGL／GLSL 版本、shader 编译、纹理上传、透明、
  FBO 离屏读取与 resize；nanovgo 的最低要求不能因平台默认 context 恰好较新而省略。
- 本轮不增加“arm64 专用”GL 参数、颜色补偿、同步等待或视觉偏移。
- 不缺库不代表 GPU 能力满足；核心库缺失按既有规范报告，
  可选扩展、合成器和服务器能力按实际查询处理，不静默探测全部库后伪装支持。
- 若当前 Mesa／桌面不能提供所需 GLX context，OpenGL 项记为失败或环境不满足，
  不能以 Software 的显示结果替代。先分离环境与代码问题，再决定是否调整环境。

## 4. 本机 QEMU 验证环境

### 4.1 主环境：完整系统模拟

```text
Linux amd64 宿主：Go 交叉编译、测试产物与日志
└─ qemu-system-aarch64：virt、TCG、明确的 AArch64 CPU
   └─ Ubuntu 24.04 arm64：ARM64 内核与 glibc 用户态
      ├─ X11 + Xfce／Xfwm4 + compositor
      ├─ ARM64 系统库、字体、Mesa、输入法
      ├─ VNC 桌面：在客体中运行 Xvnc 与桌面会话
      └─ SSH：传输及执行同源 ARM64 程序和测试产物
```

采用 QEMU `virt` 通用机器，宿主 amd64 执行 arm64 使用 TCG，不期待 KVM
跨指令集加速。选择明确的 64 位 CPU 型号并记录；不依赖 `virt` 的默认 CPU。
完整系统使管道、poll、信号和线程在 ARM64 内核上运行，减少用户态 syscall 转译干扰。

优先使用 Ubuntu 官方 24.04 arm64 cloud image，配套 AArch64 UEFI，
减少自行组合 rootfs、内核和 initramfs 的维护。rootfs 不等于可启动的完整磁盘；
若采用 rootfs，必须另行准备与其匹配的启动链，不能把无法启动归因于 GOUI。

环境约束：

- 下载镜像校验官方 SHA256，记录镜像日期、内核、QEMU、固件及桌面版本。
- 原始镜像只读保留，使用独立 qcow2 可写层；镜像、缓存、产物放在仓库外。
- 根据本机资源分配 vCPU／内存／磁盘，不把模拟器中的耗时作为性能门槛。
- VNC 在 ARM64 客体启动；宿主只观察和输入，不用宿主 amd64 桌面代替客体桌面。
- 启用 Xfwm4 compositor，透明窗口与 Integrated 验收前核实实际合成状态。
- 用 `glxinfo` 等现有工具确认该 Xvnc 会话实际提供的 GLX／OpenGL／GLSL 和 renderer。
  Xvnc／Mesa 的能力需要实测，不能仅靠安装包或 VNC 连接成功推定。
- VNC 若不满足 GLX 验收，可改用客体 Xorg 桌面并通过 QEMU 显示输出／VNC 观察；
  这属于验证环境调整，不修改 GOUI 绘制策略，也不自动获得硬件 GPU 证明。
- 按需提供 CJK／阿拉伯文字体及输入法；中文预编辑必须有客体内实际运行的服务。
- SSH／VNC 仅绑定回环或经受控端口转发访问，不将无认证桌面暴露到外网。
  不修改宿主的键盘布局、输入法或真实剪贴板来代替客体配置。

### 4.2 产物与操作方式

默认在宿主交叉编译，传入客体执行，避免在 TCG 下反复构建整仓。
普通程序、测试二进制及所调用 helper 都必须是 ARM64；记录二进制 SHA256 与源码版本。
跨进程测试需要的产物一并构建，逐包测试输出使用独立路径，避免同名包覆盖。

GUI 操作通过客体 VNC 的真实鼠标／键盘完成，遵循既有输入分发路径；
不以 DevServer、直接调用信号或修改 Widget 状态代替操作验收。
文字输入、候选窗、客体内跨进程剪贴板和拖放分别验证；
宿主与 VNC 的剪贴板桥接不计入 GOUI 的跨应用能力。

需要完整 `go test ./...` 时可在客体安装满足版本要求的 ARM64 Go SDK，
接受模拟编译较慢；日常针对性回归优先使用宿主构建的测试二进制。
不为本次适配建设新的跨层测试框架。

### 4.3 辅助方式与结果边界

`qemu-aarch64 + Ubuntu arm64 rootfs` 用户态模拟可用于快速执行 ABI／离屏回归，
但其 syscall、线程和信号依赖宿主转换，不能替代主环境的完整系统验收。
连接宿主 X server 也不能作为 ARM64 桌面组件全部验证的依据。

QEMU 完整系统可作为本轮功能支持的主要依据，但记录必须写成
“QEMU TCG／Ubuntu arm64／X11／实际 Painter／renderer”，不写“实体 ARM 真机通过”。
实体 ARM GPU 驱动、真实弱内存排序、显示呈现／vsync、帧率、功耗和设备输入
仍需后续硬件补充；这些不阻塞本轮明确环境下的功能适配。

## 5. 测试与验收

### 5.1 包内回归与编译

系统调用回归无需桌面，放在 libc 所属包：

- 管道两端 NONBLOCK、CLOEXEC 标志正确；创建失败和重复清理不误关其他 fd。
- 空管道立即 poll 返回、写后可读、排空后 EAGAIN、有限超时和无限等待被写入唤醒。
- 非法 fd 的 errno／revents 符合契约，不吞掉错误或死循环；EINTR 重试策略正确。
- 等待期间另一个 goroutine 可以执行并唤醒；使用有界测试等待防止故障挂死测试进程。

事件循环涉及 X display 的 Post／Quit／关闭验证属于窗口用例，
不要仅因未显示窗口就放入新的无桌面测试。保留已有测试组织，不借适配顺带迁移。

其他重点：

- Linux 绑定的 size／offset、整数返回、浮点和回调回归。
- Pango 注册字体实际匹配、范围字体、删除线、占位、Cluster、独立布局及销毁。
- Software 预乘像素、图片／文字缓存、clip／Transform／缩放组合。
- core／layout／gui／ui／widgets 现有无桌面逻辑回归。
- 像素断言沿用有依据的容差；不为适配不同字体或架构随意放宽金图。

修复后在宿主执行两种目标构建，并逐包交叉编译相关测试：

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...
```

本机最终运行 `go test ./...`，默认增加 `-count=1` 以获得非缓存结果。
已有用例可能创建窗口、修改剪贴板或写 PNG，使用仓库外同源快照和隔离桌面，
不覆盖工作区资源，不删除旧断言或扩大 skip。
按修改风险补充 race、Windows amd64／macOS arm64 交叉编译；未执行项如实登记。

### 5.2 窗口场景

优先复用现有已跟踪程序，按入口中文注释操作；下表只给主题索引，不维护第二份流程。

| 主题 | 现有入口 | 本切片重点 |
| --- | --- | --- |
| 颜色、透明与绘制 | `tests/window/graphics/alpha`、`tests/window/gui/render_widget` | 实际后端、预乘合成、图片、文字、离屏结果 |
| 窗口与安全边界 | `tests/window/gui/integrated_layout`、`tests/window/gui/border_box`、`tests/window/ui/window_center` | resize、最大化／恢复、透明、拖动、初始定位 |
| 文本、焦点与快捷键 | `tests/window/ui/textediting`、`tests/window/ui/shortcuts`、`tests/window/gui/tab_navigation` | 普通键入、真实 IME、候选位置、焦点和原生回调 |
| Popup、滚动与控件集成 | `tests/window/widgets/drop_down`、`tests/window/gui/scroll_layout`、`example/demo` | Popup 尺寸／退避、嵌套滚动、关闭与资源清理 |
| 拖放与窗口移交 | `tests/window/gui/dragdrop`、`tests/window/widgets/tab_detach`、`tests/window/widgets/tab_transfer` | 客体内拖入／拖出、预览、取消、跨窗口生命周期 |
| 系统集成与打包 | `tests/window/gui/file_access`、`tests/window/tools/packaging`、`example/demo` | 文件选择取消／错误、ARM64 产物、桌面安装身份 |

Software／OpenGL 分别操作绘图和关键窗口场景，覆盖应用设置的 1x／2x。
`GOUI_PLAT_SCALE=2` 只证明对应逻辑／物理换算，不等于真实显示器 DPI 切换。
记录 WM、compositor、字体和输入法，关闭测试创建的窗口与客体应用后检查正常退出。
剪贴板、打开文件／外部应用等副作用仅在隔离客体中由明确步骤触发。

不要求为每个普通控件重新穷举全部组合；以架构敏感的底层路径和已有缺陷回归为主，
demo 覆盖上层集成。外部应用拒绝私有拖放类型等既有平台差异不算 ARM64 新缺陷。

### 5.3 完成标准

1. Linux amd64／arm64 全仓及相关测试编译通过，应用仍可 no-cgo 构建。
2. ARM64 客体中的系统调用、ABI、排版和 Software 离屏回归通过。
3. QEMU 桌面关键交互通过，Software／OpenGL 的实际后端和结果分别明确。
4. 若 OpenGL 环境不满足，保留未完成项，不以软件回落宣称双后端支持完成。
5. 本机回归无新增失败；既有失败、跳过、未执行、环境不满足单独列出证据。
6. 文档记录已验证环境和实体硬件缺口，不扩大到所有 ARM 设备／发行版的保证。

## 6. 阶段与文件范围

| 阶段 | 工作 | 完成条件 |
| --- | --- | --- |
| 0 | 固化本文，建立 QEMU Ubuntu arm64／VNC 环境 | 版本／来源可追溯，SSH／桌面可操作，GL 能力已查询 |
| 1 | Pipe2／Poll 适配及包内回归 | 双架构构建通过，本机回归和 ARM64 管道测试通过 |
| 2 | ARM64 ABI、FFI、排版和 Software 验证及必要修正 | 同源测试在客体执行通过，缺陷归属明确 |
| 3 | 两后端窗口、系统集成、demo 与构建工具验收 | 关键操作通过，实际后端／缩放／差异有记录 |
| 4 | 同步支持说明和进度，按授权提交 | 不混淆模拟／硬件结果，不更改已发布 Demo |

预计修改：

- `platform/linux/libs/libc/syscall_linux.go` 及包内测试。
- `platform/linux/x11/event_linux.go`；仅在现有窗口入口缺少必要覆盖时增加小型独立用例。
- `go.mod`／`go.sum`，仅限实际依赖关系变更，不预设 sum 一定需要变化。
- ABI 诊断证明有问题的原绑定／类型及相应回归，不以此重写整个 libs。
- 本文、`doc/Plan.md`／`doc/Roadmap.md`；验收完成后再同步 README 和
  `cmd/goui/README.md` 的支持说明与使用示例。

已有未跟踪测试不自动纳入提交；需采用时先检查其内容、归属和有效覆盖。
环境启动参数与操作记录放本文或必要的小脚本，不引入通用虚拟机管理器，
也不将镜像、固件、日志、密码、SSH 私钥或可执行文件提交仓库。

建议提交按职责组织，而非每个文件单独提交：

1. 系统调用适配、事件循环最小调整与回归测试。
2. 实际发现的 ABI／FFI 修正与相应测试；没有缺陷则不制造一次整理提交。
3. 验收记录、支持文档和必要窗口用例；实际划分参考当时 diff 和往期提交风格。

本计划不授权自动提交、推送、打 tag、替换 Release 资产或安装宿主系统软件；
进入实施后仍按用户指令推进对应操作。

## 7. 后续 CI 与发布

CI 不是本轮适配前置。后续可增加 amd64／arm64 的 no-cgo 编译检查、
`ubuntu-24.04-arm` 原生 runner 的 ABI／排版／离屏测试，以及带 WM 的桌面冒烟。
原生 ARM runner、QEMU、虚拟桌面和实体 GPU 的结果分别标注，不能互相替代。
不以无桌面的 DISPLAY skip 宣称窗口场景通过。

通过验收后可在新的 Demo 发布中增加 Linux arm64 下载，继续复用 `cmd/goui` 的
build／Linux dir 契约，不修改旧 Demo 的来源／版本，不捆绑系统库或引入 AppImage。
增加目标产物、版本更新和 Release 发布仍需单独授权。

## 8. 当前进度与验收记录

- [x] 记录支持边界、最小修改设计和 QEMU 完整系统验证方案。
- [x] 复现全仓 arm64 编译阻塞，确认受影响原生包的局部交叉编译结果。
- [ ] 搭建并记录 QEMU／Ubuntu arm64／X11／VNC／Mesa 环境。
- [ ] 完成 Pipe2／Poll 修改及双架构回归。
- [ ] 完成 ARM64 客体 ABI／排版／Software 离屏验证与必要修复。
- [ ] 完成客体 Software／OpenGL 桌面操作及 1x／2x 验收。
- [ ] 完成本机最终回归，更新支持说明并按授权提交。

后续每阶段追加日期、源码版本、产物 SHA256、宿主／客体／QEMU／库版本、
命令和操作步骤、实际后端／renderer／缩放、通过／失败／跳过／未执行项。
前置编译失败保留为诊断记录，不在修复后改写成当时已经通过；
本文的完成清单也不将历史 Typography 或 macOS arm64 结果追认为 Linux ARM64 验收。

### 2026-10-15 文档写入验证

- 本轮只新增本文、同步 Plan／Roadmap，不修改生产代码或测试，不搭建 ARM64 环境。
- 三份文档的 167 个本地链接、代码围栏和空白检查通过。
- Linux amd64／Go 1.25.8，在仓库外同源工作区快照及隔离 Xvnc／Xfwm4 中执行
  `CGO_ENABLED=0 go test -json ./... -count=1`：2501 通过、15 项既有跳过、0 失败。
  旧用例的窗口和 PNG 副作用隔离在该会话／快照中，未覆盖工作区资源。
- 未执行 QEMU、ARM64 运行、实体 GPU 或远端验收；以上结果不是 ARM64 支持完成证明。

## 9. 参考依据与取舍

以下来源于 2026-10-15 核对；依赖以仓库锁定版本为准，QEMU 与 Mesa 的实际能力
以搭建环境记录为准，不把最新在线文档当作本机已安装版本的保证。

- [x/sys v0.30.0 Linux syscall 实现](https://github.com/golang/sys/blob/v0.30.0/unix/syscall_linux.go)：
  采用现成 Pipe2／Poll 封装，避免在 GOUI 维护架构 syscall 表。
- [purego v0.10.0 支持范围](https://github.com/ebitengine/purego/blob/v0.10.0/README.md#supported-platforms)
  和 [goexlib/cgo v0.1.0](https://github.com/goexlib/cgo/tree/v0.1.0)：
  有 Linux arm64 基础，仍须验证 GOUI 实际类型、参数和回调。
- [ARM AAPCS64](https://github.com/ARM-software/abi-aa/blob/main/aapcs64/aapcs64.rst)：
  核对布局、参数、返回值和可变参数，不从其他 64 位平台推断。
- [QEMU virt](https://www.qemu.org/docs/master/system/arm/virt.html)
  与 [用户态模拟](https://www.qemu.org/docs/master/user/main.html)：
  选择完整系统模拟作为主要验收，用户态模拟仅辅助，保留硬件差异。
- [Ubuntu 24.04 官方镜像](https://cloud-images.ubuntu.com/releases/24.04/release/)：
  使用可追溯 ARM64 镜像和独立可写层，不把 rootfs 当作完整启动环境。
- [Qt 6.8 X11 依赖](https://doc.qt.io/qt-6.8/linux-requirements.html#opengl-dependencies)：
  借鉴区分桌面 OpenGL 与 ES 的做法；不照搬其全部后端，不因 CPU 适配扩大范围。
- [GitHub runner 文档](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)：
  后续可用原生 ARM runner 持续验证，不将 CI 接入作为当前实施前置。
