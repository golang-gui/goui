# GOUI 文件对话框设计

状态：三平台文件选择、GUI／UI 接入及 FileAccess 凭据已实现。2026-10-14 同步
当前契约，替代早期 []string／ErrCancelled 草案。2026-11-01 增补窗口模态与异步启动，
Linux 已接入 godbus + Portal FileChooser，并保留 helper 降级；详细资源归属和分平台验证见
[文件访问](DesignFileAccess.md)，错误规则见 [跨层约定](DesignConventions.md#错误反馈规则)。

## 定位与分层

- platform 报告原生选择结果／错误，返回调用方拥有的访问对象，不伪造授权。
- gui 包装线程亲和和完成通知；FileDialog 服务永不为 nil，能力创建失败在操作时交付。
- ui 通过 App.FileDialog 的运行时命令转交 GUI 线程，不把文件对话框声明进 View 树。
- 不实现 MessageBox、自定义文件浏览器或虚拟文件系统；确认可由窗口内 Overlay 组合。

## 公共契约

平台入口为 Platform.NewFileDialog() (FileDialog, error)。GUI 接口为：

```go
type FileDialog interface {
    OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
    OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error))
    SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
    RestoreAccess(credential []byte) (FileAccess, error)
}
```

DialogOptions 包含 Title、InitialDir、DefaultName、Filters、AllowMultiple；
FileFilter 包含 Name 与 Patterns。GUI 用平台值类型别名重导出，不复制一套枚举。

GUI 完成回调经 EventLoop.Post 执行，正常运行时在 GUI 线程交付一次结果；
不能假定 platform 回调总是异步。取消为空结果加 nil error，不导出 ErrCancelled。
能力不可用、面板启动失败和访问恢复失败按该次操作交付错误，不默认弹框或退出应用。

SaveFile 仅选择目标，不创建或写文件。成功返回的每个 FileAccess 属于调用方，
独立于面板和 owner 生命周期；所有 I/O 结束后显式 Close，不能依赖 GC。

## 访问对象

FileAccess 提供 Name、LocalPath、Credential、Close：

- LocalPath 可用于 Go 标准库 I/O，但保存路径字符串不等于保存沙箱访问资格。
- Credential 是不透明的可持久化数据，不是密码、永久授权或已打开文件。
- RestoreAccess 恢复调用方拥有的新访问对象；成功后重新保存当前 Credential。
- 新保存目标应先创建文件再取得凭据；Close 幂等，之后路径／凭据操作返回关闭错误。
- UI FileAccess 是线程安全包装值，复制值共享底层资源；须在应用退出前释放。
  UI 服务只在应用运行期间执行命令，不承诺停止后完成新的选择请求。

## 三平台适配

| 平台 | 当前实现及边界 |
| --- | --- |
| Linux/X11 | 优先 Portal FileChooser，目录选择要求接口 version>=3；明确不可用时降级到既有 helper（KDE 优先 kdialog，其余优先 zenity，yad 为降级）。helper 使用 os/exec，不经 shell；退出码 1 归正常取消，stdout／诊断 stderr 分开。路径访问与凭据复用共享 fileaccess。 |
| Windows | IFileDialog／IFileOpenDialog，GUI 线程 COM／OLE 环境，owner HWND 作为父；Shell 与 COM 绑定留在既有 SDK 包，普通路径对象不模拟沙箱授权。 |
| macOS | NSOpenPanel／NSSavePanel，AppKit 主线程；访问对象保留 URL 与 security-scoped 访问，凭据恢复按实际系统结果交付。沙箱打包与签名见文件访问设计。 |

有 owner 时采用窗口级模态，无 owner 时独立显示。UI 的三个操作显式接收 windowID，
返回调度受理错误；具体接口及 ModalScope 复用方案见
[文件访问中的模态设计](DesignFileAccess.md#文件对话框的所属窗口与模态2026-11-01)。
Linux helper 的 attach/modal 参数只表示提交原生关联请求，是否实际形成父子关系
取决于 helper。实测 zenity 3.42.1 的文件选择器接受参数并设置 modal hint，
但未观察到 WM_TRANSIENT_FOR；内容输入隔离由 GUI 保证，不伪造原生父窗口事实。
当前已采用 godbus 接入 Portal FileChooser，并保留上述 helper 降级。选型、线程、资源与后续复用边界见
[Linux D-Bus 设计](DesignDBus.md)。Settings 与输入法不随该切片修改。

## Linux Portal 改造设计（2026-11-01）

状态：已实现；本机验收结果及未覆盖平台见 Plan。

### 改造范围与组织

使用 godbus 调用当前桌面 session bus 上的 org.freedesktop.portal.Desktop，
对象为 /org/freedesktop/portal/desktop。保留现有平台／GUI／UI 公共接口；
GUI 的 ModalScope 复用、owner 重复请求防护及未交付 FileAccess 清理不另建一套。
Windows/macOS、Settings、XIM、OpenURL/OpenPath 不在改造范围。

- `platform/linux/internal/portal/file_dialog_linux.go`：Portal 请求、响应、能力查询、
  连接与服务失效、取消及普通结果。直接使用 godbus，不重复封装通用 D-Bus 库。
- `platform/linux/x11/dialog_linux.go`：校验 owner、生成 parent 标识、选择 Portal 或
  原有 helper、在平台线程创建 FileAccess 和交付完成回调。
- `platform/linux/x11/platform_linux.go`：持有所需的内部 Portal 服务，不在创建
  Platform 时建立 D-Bus 连接；NewFileDialog 不能再以缺少 helper 判定全部不可用。
- `platform/linux/x11/event_linux.go`：沿既有退出清理点结束请求、释放内部服务。
  不改变 X11 的 poll／事件分发模型，也不加入 GLib main context。

协议包不持有 Window、Xlib display 或 Widget。连接在后台按需建立，多个文件选择
请求复用同一条独立拥有的连接；后台不访问 owner。X11 提取的 parent、选项、请求
标识及平台线程投递入口在启动时确定。EventLoop 退出后停止使用该入口；后续如创建
新的事件循环，再按需建立新的服务，不能复用已关闭的连接或旧投递入口。

### 能力查询与 helper 降级

每次操作先走 Portal。首次连接、激活服务和查询 FileChooser.version 都在后台执行；
查询可以触发当前 session bus 的正常服务激活，不因 Portal 尚未运行就直接判定缺失。
不主动启动独立总线，也不永久缓存一次失败。后续请求可以重新查询实际可用性。

OpenFile／SaveFile 使用 FileChooser 基础接口；OpenDirectory 要求支持 version>=3
的 directory 选项。缓存只对当前连接和服务实例有效，服务替换后重新查询。
NewFileDialog 创建适配对象不等待桌面服务；实际不可用按该次操作通过回调报告。
RestoreAccess 继续使用原路径凭据实现，不依赖是否能够打开选择器。

允许降级的情况限于能够确认 Portal 请求尚未受理：

- 无法连接当前 session bus，或没有可用的 Portal 服务／FileChooser 接口。
- 操作所需版本或方法明确不受支持。

维持当前 KDE／zenity／yad helper 选择和已有取消语义，不为本轮再修改 helper 排序。
helper 降级仍只承诺现有 GUI 内容输入隔离，其原生激活限制保留。两个后端都不可用
时交付错误，不能把它伪装为取消。

用户取消、明确拒绝、错误 Response、结果损坏、服务在选择期间退出均不降级。
启动调用超时时可能已经显示选择器，须结束本地请求、尽力关闭已知远端请求并报告
错误，不能再弹一个 helper。只有启动／能力调用设有限超时；等待用户选择没有短超时。

### 参数映射

| GOUI 操作／参数 | Portal 参数 |
| --- | --- |
| OpenFile | FileChooser.OpenFile；multiple=AllowMultiple，directory=false |
| OpenDirectory | FileChooser.OpenFile；directory=true，multiple=false |
| SaveFile | FileChooser.SaveFile；忽略 AllowMultiple |
| owner | `x11:` 加十六进制客户窗口 XID；nil 传空字符串 |
| 窗口模态 | 有 owner 时 modal=true；nil 时 modal=false |
| Title | title 字符串，空值保留为空，由后端选择默认表现 |
| InitialDir | current_folder，文件系统路径字节加末尾 NUL，D-Bus 类型 ay |
| DefaultName | SaveFile 的 current_name；其他操作忽略 |
| Filters | a(sa(us))；Name 为显示名，每个 Patterns 元素按 type=0 的 glob 条件传递 |

每个请求传唯一且不可猜测的 handle_token，不把 Widget ID、窗口标题或用户路径当作
请求标识。参数在启动前复制，worker 不读取调用方仍可修改的切片。无 owner 时不能
遗漏 modal=false 而继承 Portal 的默认模态值。过滤器仅辅助选择，不保证后端禁止
选择不匹配文件。

### 请求、响应与取消

单次操作经过能力查询、发送请求、等待 Response、完成几个阶段；helper 降级是同一
操作的替代启动路径，不会同时启动两个后端。保持一份完成状态，所有结束路径只交付
一次。协议并发状态在内部同步，X11 的窗口与原生对话框登记仍由平台线程管理。

1. 建立连接级信号接收，调用前登记 handle_token 对应的预计 Request 路径并订阅
   Response，防止快速取消／成功先于方法返回而丢失。核对实际返回路径，归并早到
   响应；不能采用方法返回后才开始监听的顺序，不能仅凭相同接口名接受其他请求结果。
2. 有效方法返回后等待 Request.Response。response=0 表示成功；1 是用户取消，
   返回空结果／nil；2 或未知值作为失败，不能归为用户主动取消。
3. owner 销毁时沿现有 cancelFileDialogs 路径取消操作，并关闭对应远端 Request。
   Request.Close 不再发送 Response，因此本地必须自行结束并回调 ErrUnavailable。
4. 取消发生在方法返回前时，取消后仍需处理迟到的请求句柄并关闭；不能只取消 Go
   的方法等待就丢失远端选择器。按 token 预计路径执行清理，迟到实际路径另作核对。
5. Response 与取消竞态由同一操作的完成状态裁决。取消后不再创建或转交 FileAccess。
   如果结果已排队，在平台线程交付前再检查操作及 owner 的有效性。
6. 连接断开或 Portal 服务实例消失时结束相关等待。不能无期限等待不会再到达的响应，
   也不能把操作自动重放给新服务实例；下一次用户操作可以重新建立服务。
7. 事件循环退出时先使本地操作失效，再以有界、尽力而为的 Close 清理远端请求，
   最后撤销订阅和关闭连接。不得用等待用户操作或无限 RPC 阻塞 GUI 退出。

Portal 的 parent/modal 请求交给桌面后端，不对外部窗口按标题或 PID 猜测后补属性，
不添加无条件抢焦点循环。是否形成正确 transient/modal 和点击 owner 时的激活行为
必须在实际桌面验证，不能因方法返回成功就标记该项通过。

### 结果、访问资源与错误

成功结果中的 uris 必须是非空字符串数组。单文件、目录和保存操作要求一个结果，
多文件操作接受一个或多个；协议异常不能静默截断或当作取消。
逐项解析 file URI，只接受无远端主机或 localhost 的绝对本地路径，拒绝 NUL、
不支持的 scheme、异常 authority、query 或 fragment。百分号编码只解码一次，
保留路径中的空格、非 ASCII 字符及字面百分号。

worker 只返回普通路径／错误；平台线程复用 platform/internal/fileaccess.Paths
构造调用方拥有的 FileAccess。任一转换失败清理已取得资源，不返回半份结果。
保存只返回目标，不创建文件、不预写内容。已有路径凭据不声明跨会话永久授权；
完整 Flatpak Documents 权限与持久化凭据支持另行设计，不随本轮承诺。

### 验证与交付

无桌面测试放协议所属包和既有 X11 dialog 测试中，覆盖：

- 三个操作、modal 显式值、版本／服务探测、目录能力缺失和全部参数编码。
- 响应先于方法返回、请求路径核对、成功／取消／异常、无关响应隔离。
- 发送前／等待中／结果排队时取消，迟到句柄关闭，服务重启／连接断开和回调一次。
- 文件 URI 的空格、Unicode、百分号、无效输入、多选部分失败及保存不创建文件。
- Portal 不可用才降级，取消／拒绝／不确定超时不重新弹窗。

真实协议测试可用受控 session bus 和假 Portal 服务，无需桌面；若环境没有测试所需
dbus-daemon，单独报告未执行，不能将跳过算作通过。类型／转换等纯逻辑覆盖仍执行。
测试替身限于协议边界，不为测试导出生产私有状态。

扩展现有 tests/window/gui/file_dialog 说明，在独立 X11 会话及独立 session bus 中
用实际 Portal 后端验证：点击 A 时选择器保持合理层叠与模态激活，B 可操作；三类
选择成功／取消、无 owner、owner 销毁及退出均正确。记录后端、WM、后端进程和
transient/modal 属性；没有 Portal 后端时不能只验 helper 后宣称 Portal 通过。

运行 go test ./...、定向 race、CGO_ENABLED=0 构建及受影响目标编译。Windows/macOS
代码不变，其已有手工验收结论保留；本切片的新增桌面重点是 Linux。
提交按平台协议、X11 接入及其对应测试／文档组织，不把设计完成等同于实现或验收。

参考：[FileChooser 参数与版本](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.FileChooser.html)、
[Request 路径／响应／Close](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Request.html)、
[父窗口标识](https://flatpak.github.io/xdg-desktop-portal/docs/window-identifiers.html)。

## 验证

逻辑测试放所属包；真实面板验证使用 tests/window 的独立程序，核对单／多选、
目录／保存、过滤器、取消与故障、回调线程、后台 I/O 后释放和凭据恢复。
已执行与未覆盖范围以 [DesignFileAccess 验证记录](DesignFileAccess.md) 为准；
本文同步不新增桌面验收结论，也不把交叉编译等同于原生面板正确。
