# 移动端入口与事件线程验收

环境：Go 1.25.8；Android ARM64 NativeActivity（最低 API 29），或 iOS ARM64
UIKit Scene（最低 iOS 15，Simulator/Device 分别制作原生组件）。需要第三阶段
purego/cgo 的修复分支，通过显式开发 workspace 接入。需要可操作的
模拟器或设备。此例程不依赖桌面 GOUI 后端，也不代表 GUI 控件已经移植。

构建使用 [mobile_build](../mobile_build/README.md)，fixture=mobile_start。
独立原生参考内嵌本目录 generate.go，通过 go generate 输出仓库外后编译。
本例有意保留独立 UIKit/NativeActivity 参考，仅验证固定入口和线程，不替代
正式平台适配。Android 使用 func_name=probe_onCreate；iOS 使用 ProbeScene
并注册 gouistart URL scheme。goui-base build -entry 生成 main 注册，不改源目录。
原生组件仅由维护者制作；消费阶段不运行原生编译器。正式移动打包尚未接入。

操作及逐项预期：

1. 安装并启动。Android 为浅色画面与中央色块，iOS 为计数文本和按钮。
   Go main 在独立 OS 线程等待测试消息；原生界面仍能响应。
   此例仅验证启动 ABI，使用有界测试 channel，不依赖正式窗口宿主；队列溢出直接失败。
   正式 EventLoop 的排序与退出由包内测试和 mobile_surface 例程覆盖。
2. 连续点击十次。Android 点击应用内容，色块随 Go 计数奇偶改变；iOS 点击
   Increment on Go thread。等待 0.5 秒后 taps 应增加十次，threadFailures 为 0。
3. Home 返回桌面再打开，重复至少三次。mainCalls 始终为 1，entries 持续增加，
   active/background 增加，计数不清零，不发生 ANR 或卡住。
4. 观察私有目录 go-state.json：mainCalls=1、events>taps、threadFailures=0、
   returned=0；entryMs 为固定入口的最大调用时间，应及时返回。它包含必要的
   runtime 首次初始化等待，不能等同于整个冷启动时间或性能保证。
5. iOS 点击 Stop Go loop，returned 变为 1。随后点击 Request startup again，
   只增加 entries，不能重新运行 main。也可使用对应的 gouistart://increment、
   gouistart://stop、gouistart://start URL；系统可能先显示打开应用的确认框，
   simctl openurl 成功不等于应用已经收到 URL，须先处理该提示再检查计数。
   Stop 仅用于验收，不定义正式 iOS Application.Quit 的语义。

生成的 `native/ProbeUITests.m` 可放进独立 XCTest UI Testing Bundle，操作已经安装的
应用，覆盖十次真实按钮点击、三次 Home/返回、停止和重复启动。测试 runner
与被测应用分开构建，不能把 runner 使用 Xcode 算作消费端需要原生编译器。

读取状态：Android 用 adb shell run-as org.golanggui.mobilestart cat
files/go-state.json（路径随 NativeActivity internalDataPath）；iOS 用 simctl
get_app_container 获取 org.golanggui.mobilestart 的 data 目录，再读
Documents/go-state.json。读取遇到文件写入中的瞬间应重试，不能放宽最终断言。

退出：通过系统停止/强制结束应用；复位：卸载重装或清理应用数据。测试会写
应用私有 JSON，消耗一个 Go 事件线程和测试用原生定时器；不访问用户文件、
不修改其他应用、不使用 DevServer。临时模拟器和专用 ADB 服务验收后清理。
