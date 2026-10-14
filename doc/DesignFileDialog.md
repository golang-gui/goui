# GOUI 文件对话框设计

状态：三平台文件选择、GUI／UI 接入及 FileAccess 凭据已实现。2026-10-14 同步
当前契约，替代早期 []string／ErrCancelled 草案；详细资源归属和分平台验证见
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
| Linux/X11 | KDE 优先 kdialog，其余优先 zenity，yad 为降级；检测失败返回不支持。使用 os/exec 传参而非 shell，退出码 1 归正常取消；stdout 与诊断 stderr 分开。路径访问与凭据由共享 fileaccess 实现管理。 |
| Windows | IFileDialog／IFileOpenDialog，GUI 线程 COM／OLE 环境，owner HWND 作为父；Shell 与 COM 绑定留在既有 SDK 包，普通路径对象不模拟沙箱授权。 |
| macOS | NSOpenPanel／NSSavePanel，AppKit 主线程；访问对象保留 URL 与 security-scoped 访问，凭据恢复按实际系统结果交付。沙箱打包与签名见文件访问设计。 |

Linux 当前 helper 后端不能据公共 owner 参数承诺真正的原生父窗口挂接。
xdg-desktop-portal 只是 [待评估方向](Roadmap.md#future-system-integration)，本轮不新增 DBus 依赖。

## 验证

逻辑测试放所属包；真实面板验证使用 tests/window 的独立程序，核对单／多选、
目录／保存、过滤器、取消与故障、回调线程、后台 I/O 后释放和凭据恢复。
已执行与未覆盖范围以 [DesignFileAccess 验证记录](DesignFileAccess.md) 为准；
本文同步不新增桌面验收结论，也不把交叉编译等同于原生面板正确。
