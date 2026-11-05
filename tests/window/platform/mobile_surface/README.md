# 移动端窗口与 Software 表面

环境：Go 1.25.8、第三阶段 purego/cgo 修复的显式 workspace；Android ARM64
NativeActivity 或 iOS ARM64 Simulator/Device。维护者需要 NDK/Xcode 制作组件，
消费端只需匹配基座、链接输入和工具；本例不是完整 GUI/UI 移动支持。

构建：使用 [mobile_build](../mobile_build/README.md)，fixture=mobile_surface。
固定入口、FFI 和 runtime 基座独立制作；Go 平台包承担窗口及呈现。测试 probe
内嵌本目录 generate.go，通过 go generate 输出到仓库外；Android 使用
func_name=probe_onCreate 包装正式 NativeActivity 入口，iOS 使用最小 main 和
GoUIScene。SDK、签名、组件和产物均放仓库外；消费构建不调用原生编译器。

初始画面：浅色背景，左上蓝色圆角矩形，下方绿色缓存图片，右下红色标记。
窗口、Painter、图片在 Run 前创建；尺寸取自原生事件。每 250ms 请求绘制，
可用 GOUI_SURFACE_DELAY=1 在 Paint 中额外等待 35ms，帮助触发表面变更竞态。
该等待仅是测试控制，不能当作框架性能或帧调度实现。

操作与预期：

1. 启动后检查 surface.json：initialReady=false、submitted>0、ready>0、
   threadFailures=0、destroyed=0。尺寸应满足 Width*scale=PixelWidth；不应把
   创建参数 320x240 当作移动端实际尺寸。
2. 反复横竖屏至少八次。蓝块和图片保持 DIP 尺寸，红色标记跟随右下角；尺寸
   和 generation 更新，nonce 不变，不能崩溃、卡住或永久停止绘制。
3. Home 到后台，等 visible=false 后间隔一秒读取两次，frames 应稳定；返回
   后 submitted 继续增加，nonce 不变。重复三次。
4. Android 按 Back 结束 Activity 再启动；在系统保留进程的情况下 pid、nonce
   与缓存图像保持不变，lost/ready 增加，destroyed 仍为 0。系统杀进程后的
   新 nonce 属于冷启动，不视为同进程恢复。
5. 观察绿色图片和四角颜色，避免只看日志。用于像素比较时在纯色内部采样，
   跳过圆角抗锯齿、状态栏和导航栏。源像素均为不透明 sRGB：背景
   RGB(238,244,250)、蓝块 RGB(30,130,220)、图片 RGB(40,170,90)、红色标记
   RGB(230,70,50)，预乘与非预乘值相同。最终截图中的纯色预期精确相等；
   若系统色彩管理改变截屏色彩空间，先明确转换，再比较，不能任意放宽容差。

Android 状态：adb shell run-as org.golanggui.mobilesurface cat files/surface.json。
iOS：simctl get_app_container 获取 data 目录，再读取 Documents/surface.json。
文件为临时写入后 rename，外部读取不应看见半份 JSON。应用只写自己的目录。

平台差异：后台不一定丢失绘制目标；iOS 后台不能替代 Scene 断开验收。Scene
disconnect/reconnect 未能由系统实际触发时单独记为原生待验；许可撤销与延迟
消息由包内回归覆盖。当前 Android 输入队列返回未处理，本例不测试 Go 触摸、
键盘或输入法，也不使用 DevServer。

退出：系统停止或强制结束测试应用；复位：卸载/清除私有数据。专用模拟器、
ADB 端口及临时包验证结束后清理；不修改用户桌面设备或长期签名配置。
