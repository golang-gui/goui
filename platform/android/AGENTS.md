# Android 原生开发规范

遵循[平台通用规范](../AGENTS.md)。本指南也适用于目录外的 Android 绘制、排版和构建接入。

## 代码组织

- `platform_android.go` 组织平台实例、能力工厂和系统宿主交付；`application_android.go`
  接收原生入口与 Activity 生命周期；`event_android.go` 连接 Go 队列和 Android Looper；
  `input_android.go` 接收原生输入；`window_android.go` 管理 Window 适配与绘制目标；
  `present_android.go` 负责软件像素呈现。按已有平台的职责命名，不把它们集中在 Window。
- `libs` 按原生库组织 NDK、EGL、Bitmap 与 JNI 绑定。绑定保留原生参数、类型、错误和
  所有权，不夹带窗口策略。实际可跨目标验证的纯 Go 类型可以独立组织，不能为了测试改变 ABI。
- Java 类绑定集中在 `sdk`，按实际原生包/类组织类型和方法；JNIEnv 由调用方显式传入，
  不保存跨线程 JNIEnv。类和 method ID 统一初始化；只绑定实际使用的 API，不建立完整 Android SDK。
- VM、应用 ClassLoader 的进程归属和启动线程附着留在 `platform/internal/androidruntime`；
  Java 类调用复用 `sdk`。SDK 不反向依赖 Android 适配或 androidruntime。
- 基于 eventfd 的唤醒对象归 `platform/internal/eventfd`，使用 `x/sys/unix`；Android 只负责
  注册 Looper 并连接现有任务队列。不另建任务队列、GUI goroutine 或通用 eventfd 框架。
- OpenGL ES 上下文归 `graphics/opengl`；Android Typography 和维护者生成的 Java 源码归
  `typography/androidtext`。不能为了避免依赖环把绘制、排版实现移进窗口或基座。

## 线程、所有权与回调

- ANativeActivity 回调已经位于系统 UI 主线程；Go 回调固定进入的线程后执行，LockOSThread
  不负责切换到 UI 线程。Go 启动入口只准备并等待，GUI 工作通过系统 Looper 执行。
- 分清进程调度设施、Activity 实例、逻辑 Window 和 NativeWindow 绘制目标的寿命。
  Activity/目标重建不销毁逻辑内容，目标代次跨 Activity 递增；进程唤醒设施不随 Activity 关闭。
- 回调应确认所属 Activity/输入队列/NativeWindow，不能由旧实例的清理修改当前实例。
  发出事件或调用原生方法可能重入；继续访问前核实当前 Activity、Window 与目标仍然有效。
- JNI 局部引用在当前调用/显式 local frame 内使用；长期引用使用 global ref，由明确所有者释放。
  Java 调用失败后先处理异常，再调用其他普通 JNI 方法；空对象与异常分别按原生契约处理。
- ANativeActivity 和其 Java Activity 对象由系统借用；不自行销毁系统对象。NativeWindow
  在销毁回调返回前撤销使用许可；Acquire/Release 仅用于明确拥有的原生引用。
- EGL 资源分步创建，失败先保存原生错误再回滚本次资源。Display 的共享与终止归属必须明确，
  不能按每个 Painter 的寿命盲目 Terminate 进程共享 Display。

## 验证

- 纯 Go/内核逻辑、ABI 和真实 Looper/ART/EGL 分开验证。Android 正式构建经 `cmd/goui`
  接入预置基座；裸 GOOS/CGO 交叉编译不等于已验证消费者构建路径。
- 原生验收至少覆盖主线程、Activity/目标重建、前后台、回调中销毁、退出边界和两绘制后端。
  JNI 变化启用 CheckJNI，检查引用寿命及异常后的恢复。按根指南记录真机与模拟器、API、ABI、
  缩放和 GPU；保留失败断言，不根据模拟器表现给生产绑定悄悄改参数。
- 触摸、IME、Popup、SafeArea 等按专题开发；结构整理不顺带增加上层行为或假装能力已经支持。
