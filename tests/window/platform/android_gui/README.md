# Android GUI 首个窗口验收

本例从普通 `gui.NewApplication` 进入 Android 工厂，使用生产 Android Typography、
Software 与 NativeActivity 宿主。它验证 GUI 接入，不替代移动输入或完整平台验收。

环境：Go 1.25.8、Android API 29+ ARM64；首次运行记录实际 API、屏幕及比例。
维护者使用 [mobile_build](../mobile_build/README.md)，fixture=android_gui，
依次执行 base/native/build/package。原生测试参考和 Java helper 从 Go 生成器
输出仓库外；Go 平台包实现窗口和生命周期。消费阶段不运行 C 编译器或 javac。


安装 `output/probe.apk`，启动 `org.golanggui.androidgui/android.app.NativeActivity`。
初始出现标题、多语言文本、动态计数、按钮、输入框和绿色块。普通 Timer 更新文字，
首帧后通过 `Window.DispatchEvent` 的按下/抬起完成一次按钮激活；不绕过事件路径。
按钮的真实触摸与输入框 IME 尚未接入，不能将本例自动点击当作触摸验收。

检查沙箱 `files/startup.json` 与持续更新的 `files/go.json`：

1. 工厂为 android；无 DesktopWindow；Run 前逻辑窗口成功且 Surface 尚未就绪。
2. GOUI owner 与系统 nativeThread 不同，Timer、Frame 和 AfterPaint 均在 owner。
3. 布局尺寸为实际 DIP，三个独立 Label 均有正高度；点击恰好一次，TextInput
   文本和控件 root 地址保持不变。无界父布局不能导致 Label 被测量为零。
4. 横/竖屏各旋转一次，尺寸正确变化；PID、root、owner 与输入框文本不变。
5. Home 后等待一秒以上：ticks 增长而 frames 稳定。重新打开同一 Activity，
   frames 恢复，显示新的计数；Surface generation 随目标替换增加。
6. 截取实际窗口：根据 color 节点 Snapshot.Bounds 换算物理坐标，在绿色块中心
   采样应为 sRGB 不透明 RGB(32,144,96)，每通道容差 1；不要采样抗锯齿边界。
   多语言文字应有画面，但本例不跨设备/字体比较统一金图。

副作用仅自身应用和沙箱 JSON，无网络/剪贴板。旋转/后台测试需恢复专用设备的
旋转设置。复位采用清除测试应用数据；退出用 `adb shell am force-stop
org.golanggui.androidgui`。不在用户正在使用的模拟器上执行清除或关机。

本切片不提供系统设置、标题修改、剪贴板、Popup、文件对话框、触摸、IME 或
GLES；这些能力明确返回不支持，GUI 沿用自身已有的可选服务/设置默认策略。
iOS 平台工厂与 CoreText 接入另行推进，不把 Android 结果等同于 iOS 通过。
