# Android 排版隔离与 EGL 回滚验收

环境：Android API29+ ARM64 真机或模拟器，OpenGL ES3，Go、cmd/goui 与 adb。
显式使用 OpenGL ES，不能静默降级 Software。多个设备时所有 adb 命令指定同一 SERIAL。

## 启动和操作

在仓库根目录执行，所有产物写到仓库外：

```sh
go run ./cmd/goui build -config tests/window/platform/android_rendering/goui.toml -target android/arm64 -package apk -o /tmp/android-rendering.apk
adb shell setprop debug.checkjni 1
adb install -r /tmp/android-rendering.apk
adb shell am force-stop org.golanggui.AndroidRendering
adb shell pm clear org.golanggui.AndroidRendering
adb shell am start -W -n org.golanggui.AndroidRendering/android.app.NativeActivity
adb shell run-as org.golanggui.AndroidRendering cat cache/rendering.json
```

初始自动检查两个 TextLayout，随后短暂绘制红色小字、绿色大字和绿色块；无交互按钮。
资源在 Paint 结束后的下一批 UI 工作中释放，通常数秒完成，20 秒未完成则 panic。

逐项预期：

1. 不同字号的两个布局测量非零且大字号更大；RGBA 位图具有预期的红/蓝通道。
2. 修改第二个布局的字体/字重和颜色后，其位图为绿色；第一个布局的测量与像素不变。
   像素按 RGBA 预乘值检查，只采 alpha>=192 的字形像素，各通道允许 1 的取整差，
   不比较字体外形或跨平台金图。
3. 第一个 Painter 创建成功。第二个 Painter 尝试在同一 NativeWindow 创建 EGLSurface
   应失败，错误保留 create EGL window surface 阶段；失败者只释放自身的资源。
4. 原 Painter 在失败后通过 RenderImage 离屏绘制并读回中心像素，预期 RGBA 为
   (32,144,96,255)，各通道允许 1 的 8 位量化差；随后完成两帧并在 UI 线程释放。
   无死锁、panic、CheckJNI 警告/错误。
5. rendering.json 中 passed/layoutIsolation/mainThread 为 true，frames 为 2，
   duplicateError 非空。缺少结果或任意断言失败都不算通过。

第 3 项依据 EGL 对同一 native window 已关联 EGLSurface 的限制，用真实原生资源触发
失败，不向生产代码加入注入入口。它用于验证失败回滚，不代表应用应为一个窗口创建多个
Painter，也不覆盖 context/pbuffer 内存耗尽、真正 context loss 或解绑失败。
旋转、前后台、Software/GLES 正常路径另运行 mobile_dispatch。

## 复位和退出

Go 循环结束后系统 Activity 可能保留空白窗口，停止并卸载自己的测试应用，恢复本轮
CheckJNI 设置，清理仓库外 APK。只关闭自己启动的模拟器。

```sh
adb shell am force-stop org.golanggui.AndroidRendering
adb uninstall org.golanggui.AndroidRendering
adb shell setprop debug.checkjni 0
```
