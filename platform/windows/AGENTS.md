# Windows 原生开发规范

遵循[平台通用规范](../AGENTS.md)。本文件也适用于 Direct2D、DirectWrite、WGL 等目录外实现及测试。

## DLL 与 COM 绑定

- 新增调用先查 `sdk`：Win32/IMM/DWM 扩展 `sdk/winapi`，Shell 扩展 `sdk/shell`，COM/OLE 扩展 `sdk/com`；图形接口归相应 SDK 包。`win32` 不重复加载 DLL 或声明同一符号。
- 沿用所在包的 `NewLazyLibrary`、符号字段、`api_windows.go` / `type_windows.go` 组织和原生命名；不要因新功能另建一套绑定风格。
- COM 沿用现有接口对象、`Unknown`、class/vtable 及方法封装形式。核对继承布局、槽位顺序、参数、返回和引用计数，不能仅凭方法名构造 vtable。
- 按 API 自身契约解释 BOOL、HRESULT、句柄和 LastError，不把调用桥返回的错误一律视为原生失败，也不只检查 HRESULT 是否等于零。
- 保留 Windows 原生类型宽度；指针/句柄与固定宽度整数不可混用。按目标架构核对结构体大小、对齐与偏移。
- 当前 Windows 目标为 x64。结构体按值参数及返回值使用 `goexlib/cgo.Call` / `CallRet`，结构体回调使用 `cgo.NewCallback`；不在单个绑定中打包结构体或按指针宽度判断调用约定。未来架构支持由 FFI 库统一实现；消息协议本身规定的位字段打包不在此限。

## 后端适配与生命周期

- 窗口 style、消息处理、状态映射、DPI/坐标换算属于 `win32`；绘制与排版资源属于对应后端，不塞入 SDK 绑定。
- 区分借用句柄、拥有句柄、共享系统资源及 COM 引用。只释放自己拥有的资源，采用其指定释放入口，覆盖失败回退路径。
- COM/OLE 初始化、反初始化及对象使用遵守 apartment 和所属线程契约；不能因包内存在全局状态就认为任意 goroutine 可以调用。
- 创建窗口、发送消息、拖放等原生调用可能同步派发回调；不能假设 Go 方法返回前对象状态不变。销毁、撤销原生注册、回调引用释放顺序要明确。
- 成功提交 SetWindowPos 等请求不代替实际窗口查询；Win32 样式与消息处理不通过 GUI 层补偿修正。

## 写法参考与验收

- WinAPI：[api_windows.go](sdk/winapi/api_windows.go)、[type_windows.go](sdk/winapi/type_windows.go)。
- Shell：[api_windows.go](sdk/shell/api_windows.go)；COM：[api_windows.go](sdk/com/api_windows.go)。参考其组织方式，新增调用仍需独立核对原生契约。
- 按改动补充 ABI/逻辑测试和 Windows 目标编译。窗口消息、合成、输入、DPI 等仍需原生运行；SSH 无头结果不等于可交互桌面验收。记录 Windows 版本、会话条件、实际绘制后端与缩放。
