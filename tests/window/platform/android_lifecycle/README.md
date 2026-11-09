# Android 宿主回调重入验收

环境：Android API29+ ARM64 真机或模拟器，Go 与 cmd/goui，安装和取证使用 adb。
本例自动执行，不依赖桌面、GUI/UI、触摸或测试原生代码。多个设备时所有 adb 命令
指定相同 `-s SERIAL`；只操作自己的测试设备。

## 启动与操作

在仓库根目录构建，产物放在仓库外：

```sh
go run ./cmd/goui build -config tests/window/platform/android_lifecycle/goui.toml -target android/arm64 -package apk -o /tmp/android-lifecycle.apk
adb shell setprop debug.checkjni 1
adb install -r /tmp/android-lifecycle.apk
adb shell am force-stop org.golanggui.AndroidLifecycle
adb shell pm clear org.golanggui.AndroidLifecycle
adb shell am start -W -n org.golanggui.AndroidLifecycle/android.app.NativeActivity
adb shell run-as org.golanggui.AndroidLifecycle cat cache/lifecycle.json
```

初始界面为空；程序在真实主线程的第一次 SizeEvent 内销毁逻辑 Window，再通过
WindowSource 重新领取宿主。第二个 Window 的第一次 SizeEvent 内销毁平台，触发
嵌套 DestroyEvent 后退出 Go 事件循环；正常数秒完成，超时或计数错误立即 panic。

逐项预期：

1. 宿主交付、SizeEvent 和 DestroyEvent 均位于原生 UI 主线程。
2. 两个逻辑 Window 各交付、处理尺寸、销毁一次；Window 销毁不销毁系统 Activity，
   重领后目标仍可重新绑定。
3. 回调中销毁不会继续向已销毁 Window 发送后续观察，不会访问失效宿主或 JNI 引用。
4. Run 在停止回调结束后返回；`lifecycle.json` 中 passed/callbackReturned 为 true，
   deliveries/destroys/sizes 均为 2。缺结果、panic、CheckJNI 警告/错误均不算通过。

本例不伪造旧 Activity 回调，也不验证多 Activity、多窗口、进程终止后的恢复或绘制。
真实 Activity/目标重建、前后台和 Software/GLES 另运行 mobile_dispatch；SDK 和 JNI
异常后的恢复另运行 android_bindings。未实测的实例交错不能由本例推断已经覆盖。

## 复位与退出

Go 循环退出后系统 Activity 可能保留空白界面；停止并卸载测试应用，清理仓库外 APK。
恢复本次开启的 CheckJNI；只关闭自己启动的模拟器，不关闭共享设备或其他应用。

```sh
adb shell am force-stop org.golanggui.AndroidLifecycle
adb uninstall org.golanggui.AndroidLifecycle
adb shell setprop debug.checkjni 0
```
