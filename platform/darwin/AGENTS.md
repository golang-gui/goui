# macOS 原生开发规范

遵循[平台通用规范](../AGENTS.md)。本文件也适用于 CoreText、macOS OpenGL 等目录外实现及测试。

## Framework 与 Objective-C 绑定

- 扩展 `frameworks` 中已有包，通过现有加载与初始化入口接入；`cocoa` 不建立重复的 framework、selector 或方法绑定。
- Objective-C 类型沿用 `initNSxxx`、`NSxxxClassId`、`NSxxxSel`、class 类型和实例类型的组织。selector 在初始化中登记，调用通过类型方法封装。
- 类方法放 class 类型，实例方法放实例类型；例如使用 `NSWorkspaceClassId.SharedWorkspace()` 形式，不添加 `NSURL_fileURLWithPath` 式平行函数包装。原生 C 函数仍按对应 framework 的 C 绑定方式组织，不硬套 Objective-C 类型。
- 按原生类/协议组织方法和委托桥接，不按 GOUI 的 WindowChrome、拖放等功能另建一套绑定。GOUI 子类的应用行为与窗口适配留在 `cocoa`。
- 核对 Objective-C/C 的 BOOL、NSInteger、CGFloat、结构体参数和返回 ABI，使用现有桥接方式；不能把整数消息调用形式套用于所有返回类型。

## 应用与资源生命周期

- 尊重 `cocoa.Platform` 及 NSApplication 的现有单例初始化路径；不为新能力假设一个进程可以构造多套应用，不借机增加多实例抽象。
- AppKit 的主线程要求不能以后台 goroutine 的 `LockOSThread` 代替。创建、调用、销毁及原生事件循环遵循实际主线程约束。
- 明确借用、autoreleased 和拥有引用。跨 autorelease pool 或原生回调期间保存的对象按所有权需要 retain，最终 release；不要对每次返回都盲目 retain/release。
- delegate、动态子类及关联 Go 数据必须在原生仍可能回调时有效；原生方法可能同步重入，回调销毁对象后不能继续无条件访问。
- AppKit point、backing pixel、GOUI DIP 及上下方向转换属于后端。绑定保留原生坐标，不加入标题栏高度或其他 GUI 策略。

## 写法参考与验收

- 类、selector 和实例方法：[appkit_darwin.go](frameworks/appkit/appkit_darwin.go)。
- Foundation 与生命周期 helper：[foundation_darwin.go](frameworks/foundation/foundation_darwin.go)。
- 单例入口：[platform_darwin.go](cocoa/platform_darwin.go)。示例用于对齐组织，具体原生语义仍须查证。
- 按改动补充目标架构编译及无需桌面的测试；AppKit 窗口、事件和输入行为在真实主线程及桌面会话验证。记录 macOS 版本、架构、实际后端与缩放；bundle 相关功能注明裸可执行文件或 `.app` 环境。
