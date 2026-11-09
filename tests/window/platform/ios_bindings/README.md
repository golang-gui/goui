# UIKit 绑定与 iOS 宿主验收

仅依赖 platform、Foundation/UIKit，不使用 gui/ui、DevServer 或测试专用原生宿主。
必须在 iOS ARM64 Simulator/device 的真实 UIKit 主线程运行；桌面 Go 测试或仅构建
不能替代。建议在自己的专用 Simulator 中验证，记录 OS、模拟器/真机、后端及 scale。

## 构建与启动

在仓库根目录使用工具支持的 Go 版本运行，产物写入仓库外的新目录：

```sh
go run ./cmd/goui build -config tests/window/platform/ios_bindings/goui.toml -target ios/arm64 -variant simulator -package app -unsigned -o /tmp/IOSBindings.app
```

GLES 版本增加 `-ldflags '-X main.painterName=opengl'`，另选输出目录。指定的后端必须
实际使用，不允许静默降级。device 改为 `-variant device -package ipa -unsigned`；
未签名包需要使用自己的 profile/证书签名，设备编译不算真机运行通过。

```sh
xcrun simctl install UDID /tmp/IOSBindings.app
xcrun simctl launch --console UDID org.golanggui.IOSBindings
xcrun simctl get_app_container UDID org.golanggui.IOSBindings data
```

## 自动步骤及逐项预期

初始可能短暂显示浅色背景和绿色块；程序自动完成，无需点击。检查应用数据容器中
`Documents/IOSBindings/bindings.json`，最终 `completed=true`、`phase=complete`、
`deliveries=3`、`destroyed=2`、`frames>=2`、checks 共九项；任一断言不满足即 panic。
控制台应有最终 `IOSBindings completed=true`，无 Objective-C 异常或主线程违规。

1. 第一轮 WindowSource 交付立即销毁 Window；返回的事件处理函数不得被安装。
2. 第二轮交付在主线程创建指定后端并实际绘制，随后检查：
   - UIApplicationDelegate/WindowSceneDelegate 声明对应协议；BOOL 的 false/true
     正确返回，应用、receiver、Scene/session/options 的身份保持。
   - 未提供的可选方法不安装；UIViewController override 及其显式 super 调用只执行一次。
   - 同名 Objective-C 类重复注册返回错误。
3. 向实际 Scene delegate 发送来源为 nil 的生命周期消息，以及向独立旧 controller
   发送 layout 消息，当前目标代次、就绪状态及销毁计数不变。旧实现丢弃来源参数，
   这些消息会错误改变当前目标。此项是延迟/无关来源的注入回归，**不是系统实际
   断开 Scene 的验收**。
4. 当前 Scene 的目标撤销通知中同步销毁 Window；该窗口不再接收后续目标和焦点。
   再次登记 WindowSource 后成功领取第三个逻辑 Window 并绘制。
5. 第三轮目标撤销通知中同步销毁 Platform；资源在主线程释放。Quit 的 Run 返回后
   才写 completed=true，防止只依据开始执行停止回调判断完成。

绑定身份测试用明确的对象 token 调用测试 delegate，仅检查 ABI 和转发，不让原生
UIKit 把这些 token 当作真实 Scene/session 使用。线程与后端通过例程断言核实。
完整图像、文字、旋转、前后台另执行 [mobile_dispatch](../mobile_dispatch/README.md)，
Software/GLES 分别验证；本例不代表实际 Scene disconnect/reconnect 或真机已通过。

## 副作用、复位和退出

只在本例沙箱保存一份 JSON，不修改用户设置、访问凭据或启动其他应用。回调所用的
Objective-C 动态类由运行时持有到进程退出；自动结束 Go 调度不等于退出 UIKit，原生
空宿主可能保留。使用 `simctl terminate UDID org.golanggui.IOSBindings` 结束。

复测前 terminate 后再 launch，确认 JSON 的 completed 字段来自本次进程；可以仅删除
本例 JSON 或卸载重装。不要复用仍在运行的进程注册同名类。验证后关闭并删除本轮
创建的 Simulator，保留用户既有设备。截图只作辅助，不能替代 JSON 断言。
